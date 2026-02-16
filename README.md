# Gokiq

A high-performance Go-based background job orchestrator that replaces Sidekiq while maintaining compatibility with existing Rails job infrastructure.

## Project Structure

```
├── docker-compose.yml          # Docker orchestration configuration
├── go_worker/                  # Gokiq Orchestrator (Go)
│   ├── Dockerfile             # Go worker container configuration
│   ├── go.mod                 # Go module dependencies
│   ├── cmd/worker/            # Main application entry point
│   ├── internal/              # Internal Go packages
│   └── config/               # Configuration files
├── rails_sidecar/             # Rails Execution Sidecar (Ruby/Falcon)
│   ├── Dockerfile            # Rails sidecar container configuration
│   ├── Gemfile               # Ruby dependencies
│   └── lib/                  # Ruby libraries
└── rails_app/                 # Example Rails application
```

## Services

### Redis
- **Port**: 6379
- **Purpose**: Job queue storage using Sidekiq-compatible JSON format.

### Gokiq Worker (Go)
- **Purpose**: High-concurrency job polling, semaphore-based backpressure, and orchestration.
- **Key Advantage**: 15x-100x less RAM usage than Ruby threads.

### Rails Sidecar (Falcon)
- **Purpose**: Fiber-based execution environment for Rails jobs.
- **Port**: 9292 (HTTP) / 50051 (gRPC)
- **Health Check**: `/health` endpoint

## Getting Started

1. Build and start Gokiq:
   ```bash
   docker-compose up --build go_worker rails_sidecar
   ```

2. Run Benchmark:
   ```bash
   ruby run_benchmark.rb
   ```