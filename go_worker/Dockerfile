# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git protobuf-dev protoc

# Install Go gRPC generators
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Add GOBIN to PATH
ENV PATH="$PATH:$(go env GOPATH)/bin"

# Copy go mod files
COPY go_worker/go.mod go_worker/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code and proto
COPY go_worker .
COPY proto ./proto

# Generate gRPC code
RUN mkdir -p internal/sidecar/proto && \
    protoc -I proto --go_out=. --go-grpc_out=. proto/job_execution.proto

# Tidy dependencies after code generation
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o worker ./cmd/worker

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates curl

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/worker .

# Copy configuration files
COPY --from=builder /app/config ./config

# Create non-root user
RUN adduser -D -s /bin/sh worker

# Ensure permissions as root
RUN chown -R worker:worker /root && \
    chmod -R 755 /root/config

USER worker

EXPOSE 8080

CMD ["./worker"]