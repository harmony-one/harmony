package sttypes

import "time"

// StreamTimeoutConfig holds configuration for progress-based timeouts
type StreamTimeoutConfig struct {
	// ProgressTimeout is the maximum time without progress before timeout
	ProgressTimeout time.Duration
	// MaxIdleTime is the maximum time without any activity before timeout
	MaxIdleTime time.Duration
	// ProgressThreshold is the minimum bytes to consider as progress
	ProgressThreshold int64
	// HealthCheckInterval is how often to check stream health
	HealthCheckInterval time.Duration
	// ChunkReadTimeout is the timeout for individual chunk reads
	ChunkReadTimeout time.Duration
}

// DefaultStreamTimeoutConfig returns the default timeout configuration
func DefaultStreamTimeoutConfig() *StreamTimeoutConfig {
	return &StreamTimeoutConfig{
		ProgressTimeout:     60 * time.Second,  // 1 minute without progress
		MaxIdleTime:         120 * time.Second, // 2 minutes without activity
		ProgressThreshold:   4096,              // 4KB progress threshold
		HealthCheckInterval: 10 * time.Second,  // Check every 10 seconds
		ChunkReadTimeout:    30 * time.Second,  // 30s per 4KB chunk
	}
}

// NewStreamTimeoutConfig creates a new timeout configuration with custom values
func NewStreamTimeoutConfig(
	progressTimeout time.Duration,
	maxIdleTime time.Duration,
	progressThreshold int64,
	healthCheckInterval time.Duration,
	chunkReadTimeout time.Duration,
) *StreamTimeoutConfig {
	return &StreamTimeoutConfig{
		ProgressTimeout:     progressTimeout,
		MaxIdleTime:         maxIdleTime,
		ProgressThreshold:   progressThreshold,
		HealthCheckInterval: healthCheckInterval,
		ChunkReadTimeout:    chunkReadTimeout,
	}
}
