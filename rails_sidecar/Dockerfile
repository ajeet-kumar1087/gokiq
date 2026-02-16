FROM ruby:3.2-alpine

# Install system dependencies
RUN apk add --no-cache \
    build-base \
    postgresql-dev \
    git \
    curl \
    tzdata \
    yaml-dev \
    protobuf-dev \
    protoc \
    grpc-dev

WORKDIR /app

# Copy Gemfile and Gemfile.lock (if it exists)
COPY rails_sidecar/Gemfile rails_sidecar/Gemfile.lock* ./

# Install gems
RUN bundle install --jobs 4 --retry 3

# Copy application code and proto
COPY rails_sidecar .
COPY proto /app/proto

# Generate gRPC code using system protoc and system plugins
RUN protoc -I /app/proto --ruby_out=lib --grpc_out=lib --plugin=protoc-gen-grpc=/usr/bin/grpc_ruby_plugin /app/proto/job_execution.proto

# Create non-root user
RUN adduser -D -s /bin/sh rails
RUN chown -R rails:rails /app
USER rails

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:9292/health || exit 1

EXPOSE 9292

CMD ["bundle", "exec", "falcon", "serve", "--bind", "http://0.0.0.0:9292"]