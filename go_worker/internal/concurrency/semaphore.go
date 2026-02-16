package concurrency

import (
	"context"
	"sync"
	"time"
)

// Semaphore provides a semaphore-based concurrency limiter
type Semaphore struct {
	ch       chan struct{}
	capacity int
	active   int
	mu       sync.RWMutex
	wg       sync.WaitGroup
}

// NewSemaphore creates a new semaphore with the given capacity
func NewSemaphore(capacity int) *Semaphore {
	if capacity <= 0 {
		capacity = 1
	}
	return &Semaphore{
		ch:       make(chan struct{}, capacity),
		capacity: capacity,
	}
}

// Acquire attempts to acquire a semaphore token
// Returns true if acquired, false if context is cancelled
func (s *Semaphore) Acquire(ctx context.Context) bool {
	select {
	case s.ch <- struct{}{}:
		s.mu.Lock()
		s.active++
		s.wg.Add(1)
		s.mu.Unlock()
		return true
	case <-ctx.Done():
		return false
	}
}

// TryAcquire attempts to acquire a semaphore token without blocking
// Returns true if acquired immediately, false otherwise
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		s.mu.Lock()
		s.active++
		s.wg.Add(1)
		s.mu.Unlock()
		return true
	default:
		return false
	}
}

// Release releases a semaphore token
func (s *Semaphore) Release() {
	select {
	case <-s.ch:
		s.mu.Lock()
		s.active--
		s.mu.Unlock()
		s.wg.Done()
	default:
		// Should not happen in normal operation
	}
}

// ActiveCount returns the current number of active tokens
func (s *Semaphore) ActiveCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// Capacity returns the maximum capacity of the semaphore
func (s *Semaphore) Capacity() int {
	return s.capacity
}

// Wait waits for all active jobs to complete
func (s *Semaphore) Wait() {
	s.wg.Wait()
}

// WaitWithTimeout waits for all active jobs to complete with a timeout
// Returns true if all jobs completed, false if timeout occurred
func (s *Semaphore) WaitWithTimeout(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
