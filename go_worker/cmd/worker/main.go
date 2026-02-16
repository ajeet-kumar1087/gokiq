package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-sidekiq-worker/internal/config"
	"go-sidekiq-worker/internal/concurrency"
	"go-sidekiq-worker/internal/redis"
	"go-sidekiq-worker/internal/sidecar"

	"gopkg.in/yaml.v2"
)

func main() {
	// Load configuration
	cfg, err := loadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override config with environment variables if present
	if url := os.Getenv("SIDECAR_URL"); url != "" {
		cfg.Sidecar.URL = url
	}
	if protocol := os.Getenv("SIDECAR_PROTOCOL"); protocol != "" {
		cfg.Sidecar.Protocol = protocol
	}

	// Initialize Redis client
	redisClient, err := redis.NewClient(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to initialize Redis client: %v", err)
	}
	defer redisClient.Close()

	// Initialize Sidecar client
	sidecarClient := sidecar.NewClient(cfg.Sidecar)

	// Initialize Concurrent Processor
	processor := concurrency.NewConcurrentProcessor(cfg.Worker.Concurrency, sidecarClient)

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Go Sidekiq Worker started (concurrency: %d, queues: %v)", 
		cfg.Worker.Concurrency, cfg.Worker.Queues)

	// Main worker loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Poll for jobs
				job, err := redisClient.PollJobs(cfg.Worker.Queues)
				if err != nil {
					log.Printf("Error polling jobs: %v", err)
					time.Sleep(1 * time.Second) // Backoff on error
					continue
				}

				if job == nil {
					// No jobs available, sleep briefly
					time.Sleep(cfg.Worker.PollInterval)
					continue
				}

				// Process the job
				if err := processor.ProcessJob(job); err != nil {
					log.Printf("Error submitting job for processing: %v", err)
					// If processor is full or shutting down, we might need to requeue
					// For now, just log it
				}
			}
		}
	}()

	// Wait for termination signal
	sig := <-sigChan
	log.Printf("Received signal %v, initiating shutdown...", sig)

	// Cancel context and shutdown processor
	cancel()
	if err := processor.Shutdown(30 * time.Second); err != nil {
		log.Printf("Shutdown error: %v", err)
	}

	log.Println("Worker stopped")
}

func loadConfig(path string) (*config.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg config.Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
