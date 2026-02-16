package config

import "time"

// Config represents the complete configuration for the Go worker
type Config struct {
	Redis   RedisConfig   `yaml:"redis"`
	Sidecar SidecarConfig `yaml:"sidecar"`
	Worker  WorkerConfig  `yaml:"worker"`
	Retry   RetryConfig   `yaml:"retry"`
}

// RedisConfig contains Redis connection settings
type RedisConfig struct {
	URL      string `yaml:"url"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// SidecarConfig contains Rails sidecar connection settings
type SidecarConfig struct {
	URL      string        `yaml:"url"`
	Protocol string        `yaml:"protocol"` // "http" or "grpc"
	Timeout  time.Duration `yaml:"timeout"`
}

// WorkerConfig contains worker behavior settings
type WorkerConfig struct {
	Concurrency  int           `yaml:"concurrency"`
	Queues       []string      `yaml:"queues"`
	PollInterval time.Duration `yaml:"poll_interval"`
}

// RetryConfig contains retry policy settings
type RetryConfig struct {
	MaxAttempts int           `yaml:"max_attempts"`
	BaseDelay   time.Duration `yaml:"base_delay"`
	MaxDelay    time.Duration `yaml:"max_delay"`
}
