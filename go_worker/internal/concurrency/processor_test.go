package concurrency

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-sidekiq-worker/internal/job"
)

// MockJobExecutor implements JobExecutor for testing
type MockJobExecutor struct {
	mu            sync.Mutex
	executedJobs  []*job.SidekiqJob
	executionTime time.Duration
	shouldFail    bool
	failureError  error
	callCount     int64
}

func NewMockJobExecutor() *MockJobExecutor {
	return &MockJobExecutor{
		executedJobs:  make([]*job.SidekiqJob, 0),
		executionTime: 10 * time.Millisecond,
	}
}

func (m *MockJobExecutor) ExecuteJob(j *job.SidekiqJob) (*job.JobResult, error) {
	atomic.AddInt64(&m.callCount, 1)

	m.mu.Lock()
	m.executedJobs = append(m.executedJobs, j)
	m.mu.Unlock()

	// Simulate execution time
	time.Sleep(m.executionTime)

	if m.shouldFail {
		return &job.JobResult{
			Status:       "failure",
			ErrorMessage: m.failureError.Error(),
		}, m.failureError
	}

	return &job.JobResult{
		Status:        "success",
		Result:        "Job completed successfully",
		ExecutionTime: m.executionTime.Seconds(),
	}, nil
}

func (m *MockJobExecutor) GetExecutedJobs() []*job.SidekiqJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	jobs := make([]*job.SidekiqJob, len(m.executedJobs))
	copy(jobs, m.executedJobs)
	return jobs
}

func (m *MockJobExecutor) GetCallCount() int64 {
	return atomic.LoadInt64(&m.callCount)
}

func (m *MockJobExecutor) SetExecutionTime(duration time.Duration) {
	m.executionTime = duration
}

func (m *MockJobExecutor) SetShouldFail(shouldFail bool, err error) {
	m.shouldFail = shouldFail
	m.failureError = err
}

func createTestJob(jid, class string) *job.SidekiqJob {
	return &job.SidekiqJob{
		JID:        jid,
		Class:      class,
		Args:       []interface{}{"arg1", "arg2"},
		Queue:      "default",
		CreatedAt:  float64(time.Now().Unix()),
		EnqueuedAt: float64(time.Now().Unix()),
	}
}

func TestNewConcurrentProcessor(t *testing.T) {
	executor := NewMockJobExecutor()
	processor := NewConcurrentProcessor(5, executor)

	if processor.Capacity() != 5 {
		t.Errorf("Capacity = %d, want 5", processor.Capacity())
	}

	if processor.ActiveJobs() != 0 {
		t.Errorf("ActiveJobs = %d, want 0", processor.ActiveJobs())
	}

	if !processor.IsRunning() {
		t.Error("Processor should be running initially")
	}
}

func TestConcurrentProcessor_ProcessJob(t *testing.T) {
	executor := NewMockJobExecutor()
	processor := NewConcurrentProcessor(2, executor)
	defer processor.Shutdown(time.Second)

	job1 := createTestJob("job1", "TestJob")
	job2 := createTestJob("job2", "TestJob")

	// Process jobs
	err1 := processor.ProcessJob(job1)
	err2 := processor.ProcessJob(job2)

	if err1 != nil {
		t.Errorf("ProcessJob returned error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("ProcessJob returned error: %v", err2)
	}

	// Wait for jobs to complete
	time.Sleep(50 * time.Millisecond)

	executedJobs := executor.GetExecutedJobs()
	if len(executedJobs) != 2 {
		t.Errorf("Expected 2 executed jobs, got %d", len(executedJobs))
	}
}

func TestConcurrentProcessor_ConcurrencyLimit(t *testing.T) {
	const concurrency = 2
	executor := NewMockJobExecutor()
	executor.SetExecutionTime(100 * time.Millisecond)

	processor := NewConcurrentProcessor(concurrency, executor)
	defer processor.Shutdown(time.Second)

	// Submit more jobs than concurrency limit
	jobs := make([]*job.SidekiqJob, 5)
	for i := 0; i < 5; i++ {
		jobs[i] = createTestJob(string(rune('1'+i)), "TestJob")
		err := processor.ProcessJob(jobs[i])
		if err != nil {
			t.Errorf("ProcessJob returned error: %v", err)
		}
	}

	// Check that active jobs don't exceed concurrency limit
	time.Sleep(10 * time.Millisecond) // Allow goroutines to start

	activeJobs := processor.ActiveJobs()
	if activeJobs > concurrency {
		t.Errorf("Active jobs %d exceeds concurrency limit %d", activeJobs, concurrency)
	}

	// Wait for all jobs to complete
	time.Sleep(200 * time.Millisecond)

	if processor.ActiveJobs() != 0 {
		t.Errorf("Expected 0 active jobs after completion, got %d", processor.ActiveJobs())
	}

	executedJobs := executor.GetExecutedJobs()
	if len(executedJobs) != 5 {
		t.Errorf("Expected 5 executed jobs, got %d", len(executedJobs))
	}
}

func TestConcurrentProcessor_HighLoad(t *testing.T) {
	const concurrency = 10
	const jobCount = 100

	executor := NewMockJobExecutor()
	executor.SetExecutionTime(5 * time.Millisecond)

	processor := NewConcurrentProcessor(concurrency, executor)
	defer processor.Shutdown(5 * time.Second)

	start := time.Now()

	// Submit many jobs
	for i := 0; i < jobCount; i++ {
		job := createTestJob(fmt.Sprintf("job-%d", i), "TestJob")
		err := processor.ProcessJob(job)
		if err != nil {
			t.Errorf("ProcessJob %d returned error: %v", i, err)
		}
	}

	// Wait for all jobs to complete
	for processor.ActiveJobs() > 0 {
		time.Sleep(10 * time.Millisecond)
	}

	duration := time.Since(start)
	executedJobs := executor.GetExecutedJobs()

	if len(executedJobs) != jobCount {
		t.Errorf("Expected %d executed jobs, got %d", jobCount, len(executedJobs))
	}

	t.Logf("Processed %d jobs in %v with concurrency %d", jobCount, duration, concurrency)
}

func TestConcurrentProcessor_JobFailure(t *testing.T) {
	executor := NewMockJobExecutor()
	executor.SetShouldFail(true, errors.New("job execution failed"))

	processor := NewConcurrentProcessor(2, executor)
	defer processor.Shutdown(time.Second)

	job := createTestJob("failing-job", "FailingJob")
	err := processor.ProcessJob(job)

	if err != nil {
		t.Errorf("ProcessJob should not return error for job execution failure: %v", err)
	}

	// Wait for job to complete
	time.Sleep(50 * time.Millisecond)

	executedJobs := executor.GetExecutedJobs()
	if len(executedJobs) != 1 {
		t.Errorf("Expected 1 executed job, got %d", len(executedJobs))
	}

	if processor.ActiveJobs() != 0 {
		t.Errorf("Expected 0 active jobs after failure, got %d", processor.ActiveJobs())
	}
}

func TestConcurrentProcessor_Shutdown(t *testing.T) {
	executor := NewMockJobExecutor()
	executor.SetExecutionTime(100 * time.Millisecond)

	processor := NewConcurrentProcessor(2, executor)

	// Submit jobs
	job1 := createTestJob("job1", "TestJob")
	job2 := createTestJob("job2", "TestJob")

	processor.ProcessJob(job1)
	processor.ProcessJob(job2)

	// Wait for jobs to start
	time.Sleep(10 * time.Millisecond)

	// Shutdown should wait for active jobs
	start := time.Now()
	err := processor.Shutdown(time.Second)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}

	if duration < 90*time.Millisecond {
		t.Errorf("Shutdown completed too quickly: %v", duration)
	}

	if processor.IsRunning() {
		t.Error("Processor should not be running after shutdown")
	}

	if processor.ActiveJobs() != 0 {
		t.Errorf("Expected 0 active jobs after shutdown, got %d", processor.ActiveJobs())
	}
}

func TestConcurrentProcessor_ShutdownTimeout(t *testing.T) {
	executor := NewMockJobExecutor()
	executor.SetExecutionTime(200 * time.Millisecond)

	processor := NewConcurrentProcessor(1, executor)

	// Submit a long-running job
	job := createTestJob("long-job", "LongJob")
	processor.ProcessJob(job)

	// Wait for job to start
	time.Sleep(10 * time.Millisecond)

	// Shutdown with short timeout should timeout
	start := time.Now()
	err := processor.Shutdown(50 * time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Error("Shutdown should return error when timeout occurs")
	}

	if duration < 45*time.Millisecond || duration > 60*time.Millisecond {
		t.Errorf("Shutdown duration %v should be close to timeout", duration)
	}

	if processor.IsRunning() {
		t.Error("Processor should not be running after shutdown attempt")
	}
}

func TestConcurrentProcessor_ProcessJobAfterShutdown(t *testing.T) {
	executor := NewMockJobExecutor()
	processor := NewConcurrentProcessor(2, executor)

	// Shutdown processor
	processor.Shutdown(time.Second)

	// Try to process job after shutdown
	job := createTestJob("job1", "TestJob")
	err := processor.ProcessJob(job)

	if err == nil {
		t.Error("ProcessJob should return error after shutdown")
	}

	if processor.ActiveJobs() != 0 {
		t.Errorf("Expected 0 active jobs, got %d", processor.ActiveJobs())
	}

	executedJobs := executor.GetExecutedJobs()
	if len(executedJobs) != 0 {
		t.Errorf("Expected 0 executed jobs, got %d", len(executedJobs))
	}
}
