package redis

import (
	"time"

	"go-sidekiq-worker/internal/job"
)

// RedisClient defines the interface for Redis operations
type RedisClient interface {
	// PollJobs polls the specified queues for new jobs and returns the first available job
	PollJobs(queues []string) (*job.SidekiqJob, error)

	// EnqueueRetry schedules a job for retry after the specified delay
	EnqueueRetry(job *job.SidekiqJob, delay time.Duration) error

	// MoveToDLQ moves a job to the dead letter queue after max retries exceeded
	MoveToDLQ(job *job.SidekiqJob) error
}
