package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"gokiq/internal/config"
	"gokiq/internal/job"
)

// HTTPClient implements the SidecarClient interface using HTTP
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
	breaker    *CircuitBreaker
	mu         sync.RWMutex
}

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	failures     int
	lastFailTime time.Time
	state        CircuitState
	mu           sync.RWMutex
}

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	// StateClosed means the circuit is closed and requests are allowed
	StateClosed CircuitState = iota
	// StateOpen means the circuit is open and requests are blocked
	StateOpen
	// StateHalfOpen means the circuit is testing if the service has recovered
	StateHalfOpen
)

// NewClient creates a new SidecarClient based on configuration
func NewClient(cfg config.SidecarConfig) SidecarClient {
	if cfg.Protocol == "grpc" {
		client, err := NewGRPCClient(cfg.URL, cfg.Timeout)
		if err != nil {
			// Fallback or log error - for prototype we'll panic to be explicit
			panic(fmt.Sprintf("failed to initialize gRPC client: %v", err))
		}
		return client
	}
	return NewHTTPClient(cfg.URL, cfg.Timeout)
}

// NewHTTPClient creates a new HTTP client for sidecar communication
func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		breaker: NewCircuitBreaker(5, 30*time.Second), // 5 failures, 30s reset
	}
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
	}
}

// ExecuteJob sends a job to the Rails sidecar for execution
func (c *HTTPClient) ExecuteJob(jobData *job.SidekiqJob) (*job.JobResult, error) {
	// Check circuit breaker
	if !c.breaker.AllowRequest() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	// Prepare request payload
	payload, err := json.Marshal(jobData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/execute", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request with retry logic
	result := &job.JobResult{}
	err = c.executeWithRetry(req, result, 3)

	if err != nil {
		c.breaker.RecordFailure()
		return nil, err
	}

	c.breaker.RecordSuccess()
	return result, nil
}

// HealthCheck performs a health check on the Rails sidecar
func (c *HTTPClient) HealthCheck() error {
	url := fmt.Sprintf("%s/health", c.baseURL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	// Parse health check response
	var healthResp struct {
		Status      string `json:"status"`
		RailsLoaded bool   `json:"rails_loaded"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		return fmt.Errorf("failed to decode health check response: %w", err)
	}

	if healthResp.Status != "ok" || !healthResp.RailsLoaded {
		return fmt.Errorf("sidecar is not healthy: status=%s, rails_loaded=%t",
			healthResp.Status, healthResp.RailsLoaded)
	}

	return nil
}

// executeWithRetry executes an HTTP request with retry logic
func (c *HTTPClient) executeWithRetry(req *http.Request, result interface{}, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			delay := time.Duration(attempt*attempt) * 100 * time.Millisecond
			time.Sleep(delay)
		}

		// Clone request for retry (body needs to be reset)
		reqClone := req.Clone(req.Context())
		if req.Body != nil {
			// Reset body for retry
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return fmt.Errorf("failed to get request body for retry: %w", err)
				}
				reqClone.Body = body
			}
		}

		resp, err := c.httpClient.Do(reqClone)
		if err != nil {
			lastErr = fmt.Errorf("request failed (attempt %d): %w", attempt+1, err)
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("failed to read response body (attempt %d): %w", attempt+1, err)
			continue
		}

		// Check status code
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error (attempt %d): status %d, body: %s",
				attempt+1, resp.StatusCode, string(body))
			continue
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("client error: status %d, body: %s", resp.StatusCode, string(body))
		}

		// Parse successful response
		if result != nil {
			if err := json.Unmarshal(body, result); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}
		}

		return nil
	}

	return fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, lastErr)
}

// AllowRequest checks if the circuit breaker allows the request
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = StateClosed
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailTime = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = StateOpen
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}
