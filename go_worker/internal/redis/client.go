package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"

	"gokiq/internal/config"
	"gokiq/internal/job"
)

// Client implements the RedisClient interface with connection pooling
type Client struct {
	client *redis.Client
	ctx    context.Context
}

// NewClient creates a new Redis client with connection pooling
func NewClient(cfg config.RedisConfig) (*Client, error) {
	opts := &redis.Options{
		Addr:     cfg.URL,
		Password: cfg.Password,
		DB:       cfg.DB,
		// Connection pool settings
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
		IdleTimeout:  5 * time.Minute,
	}

	client := redis.NewClient(opts)
	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{
		client: client,
		ctx:    ctx,
	}, nil
}

// PollJobs polls the specified queues for new jobs using BLPOP for blocking operation
func (c *Client) PollJobs(queues []string) (*job.SidekiqJob, error) {
	// Convert queue names to Sidekiq format (queue:name)
	sidekiqQueues := make([]string, len(queues))
	for i, queue := range queues {
		sidekiqQueues[i] = fmt.Sprintf("queue:%s", queue)
	}

	// Use BLPOP with 1 second timeout to avoid blocking indefinitely
	result, err := c.client.BLPop(c.ctx, 1*time.Second, sidekiqQueues...).Result()
	if err != nil {
		if err == redis.Nil {
			// No jobs available, return nil without error
			return nil, nil
		}
		return nil, fmt.Errorf("failed to poll jobs from Redis: %w", err)
	}

	if len(result) < 2 {
		return nil, fmt.Errorf("invalid BLPOP result format")
	}

	// result[0] is the queue name, result[1] is the job JSON
	jobJSON := result[1]

	var sidekiqJob job.SidekiqJob
	if err := json.Unmarshal([]byte(jobJSON), &sidekiqJob); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job JSON: %w", err)
	}

	return &sidekiqJob, nil
}

// EnqueueRetry schedules a job for retry after the specified delay
func (c *Client) EnqueueRetry(jobToRetry *job.SidekiqJob, delay time.Duration) error {
	// Update job metadata for retry
	now := float64(time.Now().Unix())
	jobToRetry.FailedAt = now
	jobToRetry.Retry++

	// Serialize job to JSON
	jobJSON, err := json.Marshal(jobToRetry)
	if err != nil {
		return fmt.Errorf("failed to marshal retry job: %w", err)
	}

	// Calculate retry time
	retryAt := time.Now().Add(delay).Unix()

	if delay > 0 {
		// Add to scheduled set for delayed retry
		score := float64(retryAt)
		if err := c.client.ZAdd(c.ctx, "schedule", &redis.Z{
			Score:  score,
			Member: string(jobJSON),
		}).Err(); err != nil {
			return fmt.Errorf("failed to schedule retry job: %w", err)
		}
	} else {
		// Immediate retry - add back to retry queue
		queueName := fmt.Sprintf("queue:%s", jobToRetry.Queue)
		if err := c.client.LPush(c.ctx, queueName, string(jobJSON)).Err(); err != nil {
			return fmt.Errorf("failed to enqueue retry job: %w", err)
		}
	}

	return nil
}

// MoveToDLQ moves a job to the dead letter queue after max retries exceeded
func (c *Client) MoveToDLQ(jobToMove *job.SidekiqJob) error {
	// Update job metadata
	now := float64(time.Now().Unix())
	jobToMove.FailedAt = now

	// Serialize job to JSON
	jobJSON, err := json.Marshal(jobToMove)
	if err != nil {
		return fmt.Errorf("failed to marshal dead job: %w", err)
	}

	// Add to dead letter queue with current timestamp as score
	score := float64(time.Now().Unix())
	if err := c.client.ZAdd(c.ctx, "dead", &redis.Z{
		Score:  score,
		Member: string(jobJSON),
	}).Err(); err != nil {
		return fmt.Errorf("failed to move job to dead letter queue: %w", err)
	}

	// Trim dead queue to prevent unlimited growth (keep last 10000 jobs)
	if err := c.client.ZRemRangeByRank(c.ctx, "dead", 0, -10001).Err(); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Warning: failed to trim dead letter queue: %v\n", err)
	}

	return nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.client.Close()
}

// GetQueueSize returns the current size of a queue
func (c *Client) GetQueueSize(queueName string) (int64, error) {
	sidekiqQueue := fmt.Sprintf("queue:%s", queueName)
	return c.client.LLen(c.ctx, sidekiqQueue).Result()
}

// GetScheduledJobs returns jobs that are ready to be moved from scheduled to queue
func (c *Client) GetScheduledJobs() ([]*job.SidekiqJob, error) {
	now := float64(time.Now().Unix())

	// Get jobs with score <= now (ready to be processed)
	result, err := c.client.ZRangeByScoreWithScores(c.ctx, "schedule", &redis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatFloat(now, 'f', -1, 64),
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled jobs: %w", err)
	}

	jobs := make([]*job.SidekiqJob, 0, len(result))
	for _, z := range result {
		jobJSON := z.Member.(string)
		var sidekiqJob job.SidekiqJob
		if err := json.Unmarshal([]byte(jobJSON), &sidekiqJob); err != nil {
			// Skip malformed jobs but continue processing others
			continue
		}
		jobs = append(jobs, &sidekiqJob)
	}

	return jobs, nil
}

// MoveScheduledToQueue moves a scheduled job to its target queue
func (c *Client) MoveScheduledToQueue(jobToMove *job.SidekiqJob) error {
	// Serialize job to JSON
	jobJSON, err := json.Marshal(jobToMove)
	if err != nil {
		return fmt.Errorf("failed to marshal scheduled job: %w", err)
	}

	// Use pipeline for atomic operation
	pipe := c.client.Pipeline()

	// Remove from scheduled set
	pipe.ZRem(c.ctx, "schedule", string(jobJSON))

	// Add to target queue
	queueName := fmt.Sprintf("queue:%s", jobToMove.Queue)
	pipe.LPush(c.ctx, queueName, string(jobJSON))

	// Execute pipeline
	if _, err := pipe.Exec(c.ctx); err != nil {
		return fmt.Errorf("failed to move scheduled job to queue: %w", err)
	}

	return nil
}

// generateJitter adds random jitter to prevent thundering herd
func generateJitter(baseDelay time.Duration) time.Duration {
	// Add up to 25% jitter
	jitter := time.Duration(rand.Float64() * 0.25 * float64(baseDelay))
	return baseDelay + jitter
}
