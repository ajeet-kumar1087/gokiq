# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Add GOBIN to PATH
ENV PATH="$PATH:$(go env GOPATH)/bin"

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

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