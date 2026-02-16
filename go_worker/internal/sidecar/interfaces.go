package sidecar

import "go-sidekiq-worker/internal/job"

// SidecarClient defines the interface for communicating with the Rails sidecar
type SidecarClient interface {
	// ExecuteJob sends a job to the Rails sidecar for execution and returns the result
	ExecuteJob(job *job.SidekiqJob) (*job.JobResult, error)

	// HealthCheck performs a health check on the Rails sidecar
	HealthCheck() error
}
