package concurrency

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go-sidekiq-worker/internal/job"
)

// JobExecutor defines the interface for executing jobs
type JobExecutor interface {
	ExecuteJob(job *job.SidekiqJob) (*job.JobResult, error)
}

// ConcurrentProcessor manages concurrent job processing with semaphore control
type ConcurrentProcessor struct {
	semaphore *Semaphore
	executor  JobExecutor
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	running   bool
}

// NewConcurrentProcessor creates a new concurrent processor
func NewConcurrentProcessor(concurrency int, executor JobExecutor) *ConcurrentProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConcurrentProcessor{
		semaphore: NewSemaphore(concurrency),
		executor:  executor,
		ctx:       ctx,
		cancel:    cancel,
		running:   true,
	}
}

// ProcessJob processes a job concurrently using semaphore control
func (cp *ConcurrentProcessor) ProcessJob(job *job.SidekiqJob) error {
	cp.mu.RLock()
	if !cp.running {
		cp.mu.RUnlock()
		return fmt.Errorf("processor is shutting down")
	}
	cp.mu.RUnlock()

	// Try to acquire semaphore token
	if !cp.semaphore.Acquire(cp.ctx) {
		return fmt.Errorf("failed to acquire semaphore token: context cancelled")
	}

	// Spawn goroutine to process the job
	cp.wg.Add(1)
	go func() {
		defer cp.wg.Done()
		defer cp.semaphore.Release()

		cp.executeJob(job)
	}()

	return nil
}

// executeJob executes a single job
func (cp *ConcurrentProcessor) executeJob(job *job.SidekiqJob) {
	start := time.Now()

	log.Printf("Starting job execution: JID=%s, Class=%s", job.JID, job.Class)

	result, err := cp.executor.ExecuteJob(job)

	duration := time.Since(start)

	if err != nil {
		log.Printf("Job execution failed: JID=%s, Class=%s, Error=%v, Duration=%v",
			job.JID, job.Class, err, duration)
		return
	}

	if result.Status == "success" {
		log.Printf("Job execution completed: JID=%s, Class=%s, Duration=%v",
			job.JID, job.Class, duration)
	} else {
		log.Printf("Job execution failed: JID=%s, Class=%s, Error=%s, Duration=%v",
			job.JID, job.Class, result.ErrorMessage, duration)
	}
}

// Shutdown initiates graceful shutdown of the processor
func (cp *ConcurrentProcessor) Shutdown(timeout time.Duration) error {
	cp.mu.Lock()
	if !cp.running {
		cp.mu.Unlock()
		return nil // Already shutting down
	}
	cp.running = false
	cp.mu.Unlock()

	log.Printf("Initiating graceful shutdown with timeout %v", timeout)

	// Cancel context to prevent new jobs from being accepted
	cp.cancel()

	// Wait for active jobs to complete with timeout
	done := make(chan struct{})
	go func() {
		cp.wg.Wait()
		cp.semaphore.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("Graceful shutdown completed successfully")
		return nil
	case <-time.After(timeout):
		log.Printf("Graceful shutdown timed out after %v", timeout)
		return fmt.Errorf("shutdown timeout exceeded")
	}
}

// ActiveJobs returns the number of currently active jobs
func (cp *ConcurrentProcessor) ActiveJobs() int {
	return cp.semaphore.ActiveCount()
}

// Capacity returns the maximum concurrency capacity
func (cp *ConcurrentProcessor) Capacity() int {
	return cp.semaphore.Capacity()
}

// IsRunning returns whether the processor is currently accepting new jobs
func (cp *ConcurrentProcessor) IsRunning() bool {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.running
}
