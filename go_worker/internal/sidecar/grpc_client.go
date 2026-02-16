package sidecar

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gokiq/internal/job"
	pb "gokiq/internal/sidecar/proto/job_execution"
)

// GRPCClient implements the SidecarClient interface using gRPC
type GRPCClient struct {
	client  pb.JobExecutionClient
	conn    *grpc.ClientConn
	timeout time.Duration
}

// NewGRPCClient creates a new gRPC client for sidecar communication
func NewGRPCClient(address string, timeout time.Duration) (*GRPCClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	return &GRPCClient{
		client:  pb.NewJobExecutionClient(conn),
		conn:    conn,
		timeout: timeout,
	}, nil
}

// ExecuteJob sends a job to the Rails sidecar via gRPC
func (c *GRPCClient) ExecuteJob(jobData *job.SidekiqJob) (*job.JobResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Convert args to strings for the proto
	strArgs := make([]string, len(jobData.Args))
	for i, arg := range jobData.Args {
		strArgs[i] = fmt.Sprintf("%v", arg)
	}

	req := &pb.JobRequest{
		Class:      jobData.Class,
		Jid:        jobData.JID,
		Queue:      jobData.Queue,
		Args:       strArgs,
		CreatedAt:  jobData.CreatedAt,
		EnqueuedAt: jobData.EnqueuedAt,
	}

	resp, err := c.client.ExecuteJob(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gRPC execution failed: %w", err)
	}

	return &job.JobResult{
		Status:        resp.Status,
		Result:        "",
		ExecutionTime: resp.ExecutionTime,
		ErrorMessage:  resp.ErrorMessage,
	}, nil
}

// HealthCheck performs a health check via gRPC
func (c *GRPCClient) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.HealthCheck(ctx, &pb.HealthRequest{})
	if err != nil {
		return fmt.Errorf("gRPC health check failed: %w", err)
	}

	if resp.Status != "ok" || !resp.RailsLoaded {
		return fmt.Errorf("gRPC sidecar is not healthy")
	}

	return nil
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	return c.conn.Close()
}
