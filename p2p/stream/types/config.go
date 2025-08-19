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
		ProgressTimeout:     30 * time.Second, // 30 seconds without progress (more aggressive)
		MaxIdleTime:         60 * time.Second, // 1 minute without activity (more aggressive)
		ProgressThreshold:   2048,             // 2KB progress threshold (more sensitive)
		HealthCheckInterval: 5 * time.Second,  // Check every 5 seconds (more frequent)
		ChunkReadTimeout:    15 * time.Second, // 15s per 4KB chunk (more aggressive)
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
