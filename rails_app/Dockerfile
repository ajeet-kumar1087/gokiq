FROM ruby:3.2-alpine

# Install system dependencies
RUN apk add --no-cache \
    build-base \
    postgresql-dev \
    sqlite-dev \
    nodejs \
    npm \
    git \
    curl \
    tzdata \
    yaml-dev

WORKDIR /app

# Copy Gemfile and Gemfile.lock (if it exists)
COPY rails_app/Gemfile rails_app/Gemfile.lock* ./

# Install gems
RUN bundle install --jobs 4 --retry 3

# Copy package.json if it exists
COPY rails_app/package*.json ./
RUN if [ -f package.json ]; then npm install; fi

# Copy application code
COPY rails_app .

# Precompile assets (if needed)
RUN if [ -f config/application.rb ]; then \
      RAILS_ENV=production bundle exec rails assets:precompile || true; \
    fi

# Create non-root user
RUN adduser -D -s /bin/sh rails
RUN chown -R rails:rails /app
USER rails

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:3000/health || exit 1

EXPOSE 3000

CMD ["bundle", "exec", "rails", "server", "-b", "0.0.0.0"]