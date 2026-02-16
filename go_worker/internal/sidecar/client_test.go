package sidecar

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gokiq/internal/job"
)

func TestHTTPClient_ExecuteJob_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobs/execute" {
			t.Errorf("Expected path /jobs/execute, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Parse request body
		var receivedJob job.SidekiqJob
		if err := json.NewDecoder(r.Body).Decode(&receivedJob); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Verify job data
		if receivedJob.Class != "TestJob" {
			t.Errorf("Expected job class TestJob, got %s", receivedJob.Class)
		}

		// Send success response
		result := job.JobResult{
			Status:        "success",
			Result:        "Job completed successfully",
			ExecutionTime: 1.23,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(server.URL, 5*time.Second)

	// Create test job
	testJob := &job.SidekiqJob{
		Class:      "TestJob",
		Args:       []interface{}{"arg1", "arg2"},
		JID:        "test-job-id",
		Queue:      "default",
		CreatedAt:  float64(time.Now().Unix()),
		EnqueuedAt: float64(time.Now().Unix()),
	}

	// Execute job
	result, err := client.ExecuteJob(testJob)
	if err != nil {
		t.Fatalf("ExecuteJob failed: %v", err)
	}

	// Verify result
	if result.Status != "success" {
		t.Errorf("Expected status success, got %s", result.Status)
	}
	if result.Result != "Job completed successfully" {
		t.Errorf("Expected result 'Job completed successfully', got %s", result.Result)
	}
	if result.ExecutionTime != 1.23 {
		t.Errorf("Expected execution time 1.23, got %f", result.ExecutionTime)
	}
}

func TestHTTPClient_ExecuteJob_Failure(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := job.JobResult{
			Status:       "failure",
			Result:       "",
			ErrorMessage: "Job execution failed",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 5*time.Second)

	testJob := &job.SidekiqJob{
		Class: "FailingJob",
		JID:   "failing-job-id",
	}

	result, err := client.ExecuteJob(testJob)
	if err != nil {
		t.Fatalf("ExecuteJob failed: %v", err)
	}

	if result.Status != "failure" {
		t.Errorf("Expected status failure, got %s", result.Status)
	}
	if result.ErrorMessage != "Job execution failed" {
		t.Errorf("Expected error message 'Job execution failed', got %s", result.ErrorMessage)
	}
}

func TestHTTPClient_ExecuteJob_ServerError_WithRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Return server error for first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
			return
		}
		// Success on 3rd attempt
		result := job.JobResult{
			Status: "success",
			Result: "Job completed after retry",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 5*time.Second)

	testJob := &job.SidekiqJob{
		Class: "RetryJob",
		JID:   "retry-job-id",
	}

	result, err := client.ExecuteJob(testJob)
	if err != nil {
		t.Fatalf("ExecuteJob failed: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
	if result.Status != "success" {
		t.Errorf("Expected status success, got %s", result.Status)
	}
}

func TestHTTPClient_ExecuteJob_MaxRetriesExceeded(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 5*time.Second)

	testJob := &job.SidekiqJob{
		Class: "AlwaysFailingJob",
		JID:   "always-failing-job-id",
	}

	_, err := client.ExecuteJob(testJob)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	if attempts != 4 { // 1 initial + 3 retries
		t.Errorf("Expected 4 attempts, got %d", attempts)
	}
}

func TestHTTPClient_ExecuteJob_ClientError_NoRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 5*time.Second)

	testJob := &job.SidekiqJob{
		Class: "BadJob",
		JID:   "bad-job-id",
	}

	_, err := client.ExecuteJob(testJob)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt (no retry for client error), got %d", attempts)
	}
}

func TestHTTPClient_HealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("Expected path /health, got %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		response := map[string]interface{}{
			"status":       "ok",
			"rails_loaded": true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 5*time.Second)

	err := client.HealthCheck()
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
}

func TestHTTPClient_HealthCheck_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":       "error",
			"rails_loaded": false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 5*time.Second)

	err := client.HealthCheck()
	if err == nil {
		t.Fatal("Expected health check to fail but got nil error")
	}
}

func TestHTTPClient_HealthCheck_ServerDown(t *testing.T) {
	client := NewHTTPClient("http://localhost:99999", 1*time.Second)

	err := client.HealthCheck()
	if err == nil {
		t.Fatal("Expected health check to fail but got nil error")
	}
}

func TestCircuitBreaker_Closed_AllowsRequests(t *testing.T) {
	breaker := NewCircuitBreaker(3, 10*time.Second)

	if !breaker.AllowRequest() {
		t.Error("Circuit breaker should allow requests when closed")
	}

	if breaker.GetState() != StateClosed {
		t.Errorf("Expected state Closed, got %v", breaker.GetState())
	}
}

func TestCircuitBreaker_OpensAfterMaxFailures(t *testing.T) {
	breaker := NewCircuitBreaker(2, 10*time.Second)

	// Record failures
	breaker.RecordFailure()
	if breaker.GetState() != StateClosed {
		t.Error("Circuit should remain closed after first failure")
	}

	breaker.RecordFailure()
	if breaker.GetState() != StateOpen {
		t.Error("Circuit should open after max failures")
	}

	if breaker.AllowRequest() {
		t.Error("Circuit breaker should not allow requests when open")
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	breaker := NewCircuitBreaker(1, 100*time.Millisecond)

	// Trigger circuit open
	breaker.RecordFailure()
	if breaker.GetState() != StateOpen {
		t.Error("Circuit should be open after failure")
	}

	// Wait for reset timeout
	time.Sleep(150 * time.Millisecond)

	if !breaker.AllowRequest() {
		t.Error("Circuit breaker should allow request after timeout (half-open)")
	}

	if breaker.GetState() != StateHalfOpen {
		t.Errorf("Expected state HalfOpen, got %v", breaker.GetState())
	}
}

func TestCircuitBreaker_ClosesAfterSuccess(t *testing.T) {
	breaker := NewCircuitBreaker(1, 100*time.Millisecond)

	// Open circuit
	breaker.RecordFailure()

	// Wait and go half-open
	time.Sleep(150 * time.Millisecond)
	breaker.AllowRequest()

	// Record success should close circuit
	breaker.RecordSuccess()

	if breaker.GetState() != StateClosed {
		t.Errorf("Expected state Closed after success, got %v", breaker.GetState())
	}
}

func TestHTTPClient_CircuitBreaker_Integration(t *testing.T) {
	failureCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failureCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create client with lower failure threshold for testing
	client := NewHTTPClient(server.URL, 1*time.Second)
	client.breaker = NewCircuitBreaker(2, 30*time.Second) // 2 failures to open

	testJob := &job.SidekiqJob{
		Class: "TestJob",
		JID:   "test-id",
	}

	// First two requests should fail and open circuit
	for i := 0; i < 2; i++ {
		_, err := client.ExecuteJob(testJob)
		if err == nil {
			t.Errorf("Expected error on attempt %d", i+1)
		}
	}

	// Circuit should now be open
	_, err := client.ExecuteJob(testJob)
	if err == nil || err.Error() != "circuit breaker is open" {
		t.Errorf("Expected circuit breaker error, got: %v", err)
	}
}

func TestHTTPClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		result := job.JobResult{Status: "success"}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	// Create client with short timeout
	client := NewHTTPClient(server.URL, 500*time.Millisecond)

	testJob := &job.SidekiqJob{
		Class: "SlowJob",
		JID:   "slow-job-id",
	}

	_, err := client.ExecuteJob(testJob)
	if err == nil {
		t.Fatal("Expected timeout error but got nil")
	}
}
