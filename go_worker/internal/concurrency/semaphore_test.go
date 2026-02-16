package concurrency

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewSemaphore(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		expected int
	}{
		{"positive capacity", 5, 5},
		{"zero capacity", 0, 1},
		{"negative capacity", -1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sem := NewSemaphore(tt.capacity)
			if sem.Capacity() != tt.expected {
				t.Errorf("NewSemaphore(%d) capacity = %d, want %d", tt.capacity, sem.Capacity(), tt.expected)
			}
			if sem.ActiveCount() != 0 {
				t.Errorf("NewSemaphore(%d) active count = %d, want 0", tt.capacity, sem.ActiveCount())
			}
		})
	}
}

func TestSemaphore_Acquire_Release(t *testing.T) {
	sem := NewSemaphore(2)
	ctx := context.Background()

	// Test successful acquisition
	if !sem.Acquire(ctx) {
		t.Error("First acquire should succeed")
	}
	if sem.ActiveCount() != 1 {
		t.Errorf("Active count after first acquire = %d, want 1", sem.ActiveCount())
	}

	if !sem.Acquire(ctx) {
		t.Error("Second acquire should succeed")
	}
	if sem.ActiveCount() != 2 {
		t.Errorf("Active count after second acquire = %d, want 2", sem.ActiveCount())
	}

	// Test release
	sem.Release()
	if sem.ActiveCount() != 1 {
		t.Errorf("Active count after first release = %d, want 1", sem.ActiveCount())
	}

	sem.Release()
	if sem.ActiveCount() != 0 {
		t.Errorf("Active count after second release = %d, want 0", sem.ActiveCount())
	}
}

func TestSemaphore_TryAcquire(t *testing.T) {
	sem := NewSemaphore(1)

	// First try acquire should succeed
	if !sem.TryAcquire() {
		t.Error("First TryAcquire should succeed")
	}

	// Second try acquire should fail (no blocking)
	if sem.TryAcquire() {
		t.Error("Second TryAcquire should fail when at capacity")
	}

	// After release, should succeed again
	sem.Release()
	if !sem.TryAcquire() {
		t.Error("TryAcquire after release should succeed")
	}

	sem.Release()
}

func TestSemaphore_AcquireWithCancelledContext(t *testing.T) {
	sem := NewSemaphore(1)
	ctx, cancel := context.WithCancel(context.Background())

	// Fill the semaphore
	if !sem.Acquire(ctx) {
		t.Error("First acquire should succeed")
	}

	// Cancel context
	cancel()

	// This should return false due to cancelled context
	if sem.Acquire(ctx) {
		t.Error("Acquire with cancelled context should return false")
	}

	sem.Release()
}

func TestSemaphore_ConcurrentAccess(t *testing.T) {
	const capacity = 5
	const goroutines = 20
	const iterations = 10

	sem := NewSemaphore(capacity)
	var wg sync.WaitGroup
	var maxActive int
	var mu sync.Mutex

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				ctx := context.Background()
				if sem.Acquire(ctx) {
					// Check that we never exceed capacity
					active := sem.ActiveCount()
					mu.Lock()
					if active > maxActive {
						maxActive = active
					}
					if active > capacity {
						t.Errorf("Active count %d exceeds capacity %d", active, capacity)
					}
					mu.Unlock()

					// Simulate some work
					time.Sleep(time.Millisecond)
					sem.Release()
				}
			}
		}()
	}

	wg.Wait()

	if sem.ActiveCount() != 0 {
		t.Errorf("Final active count = %d, want 0", sem.ActiveCount())
	}

	mu.Lock()
	if maxActive > capacity {
		t.Errorf("Maximum observed active count %d exceeded capacity %d", maxActive, capacity)
	}
	mu.Unlock()
}

func TestSemaphore_Wait(t *testing.T) {
	sem := NewSemaphore(2)
	ctx := context.Background()

	// Acquire tokens in goroutines
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sem.Acquire(ctx) {
				time.Sleep(100 * time.Millisecond)
				sem.Release()
			}
		}()
	}

	// Wait a bit to ensure goroutines acquire tokens
	time.Sleep(10 * time.Millisecond)

	// Wait should block until all tokens are released
	start := time.Now()
	sem.Wait()
	duration := time.Since(start)

	if duration < 80*time.Millisecond {
		t.Errorf("Wait returned too quickly: %v", duration)
	}

	wg.Wait()
}

func TestSemaphore_WaitWithTimeout(t *testing.T) {
	sem := NewSemaphore(1)
	ctx := context.Background()

	// Acquire token and hold it longer than timeout
	go func() {
		if sem.Acquire(ctx) {
			time.Sleep(200 * time.Millisecond)
			sem.Release()
		}
	}()

	// Wait a bit to ensure the goroutine acquires the token
	time.Sleep(10 * time.Millisecond)

	// WaitWithTimeout should return false due to timeout
	if sem.WaitWithTimeout(50 * time.Millisecond) {
		t.Error("WaitWithTimeout should return false when timeout occurs")
	}

	// WaitWithTimeout with longer timeout should succeed
	if !sem.WaitWithTimeout(200 * time.Millisecond) {
		t.Error("WaitWithTimeout should return true when jobs complete within timeout")
	}
}

func TestSemaphore_HighLoad(t *testing.T) {
	const capacity = 10
	const goroutines = 100
	const jobsPerGoroutine = 50

	sem := NewSemaphore(capacity)
	var wg sync.WaitGroup
	var completed int64
	var mu sync.Mutex

	start := time.Now()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < jobsPerGoroutine; j++ {
				ctx := context.Background()
				if sem.Acquire(ctx) {
					// Simulate work
					time.Sleep(time.Millisecond)

					mu.Lock()
					completed++
					mu.Unlock()

					sem.Release()
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	mu.Lock()
	totalJobs := int64(goroutines * jobsPerGoroutine)
	if completed != totalJobs {
		t.Errorf("Completed jobs %d, want %d", completed, totalJobs)
	}
	mu.Unlock()

	if sem.ActiveCount() != 0 {
		t.Errorf("Final active count = %d, want 0", sem.ActiveCount())
	}

	t.Logf("Processed %d jobs in %v with capacity %d", totalJobs, duration, capacity)
}
