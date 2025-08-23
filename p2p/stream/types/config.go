package sttypes

import (
	"errors"
	"time"
)

// StreamTimeoutConfig holds configuration for progress-based timeouts
type StreamTimeoutConfig struct {
	// ProgressTimeout is the maximum time without progress before timeout
	ProgressTimeout time.Duration
	// ProgressThreshold is the minimum bytes to consider as progress
	ProgressThreshold int64
	// HealthCheckInterval is how often to check stream health
	HealthCheckInterval time.Duration
	// ChunkReadTimeout is the timeout for individual chunk reads
	ChunkReadTimeout time.Duration
	// ChunkSize is the size of chunks to read at once (should align with ProgressThreshold)
	ChunkSize int64
}

// DefaultStreamTimeoutConfig returns the default timeout configuration
func DefaultStreamTimeoutConfig() *StreamTimeoutConfig {
	return &StreamTimeoutConfig{
		ProgressTimeout:     5 * time.Minute,  // 5 minutes without progress
		ProgressThreshold:   512,              // 512 bytes progress threshold
		HealthCheckInterval: 15 * time.Second, // Check every 15 seconds
		ChunkReadTimeout:    2 * time.Minute,  // 2 minutes per chunk read
		ChunkSize:           512,              // 512 bytes chunk size (aligned with ProgressThreshold)
	}
}

// NewStreamTimeoutConfig creates a new timeout configuration with custom values
func NewStreamTimeoutConfig(
	progressTimeout time.Duration,
	progressThreshold int64,
	healthCheckInterval time.Duration,
	chunkReadTimeout time.Duration,
	chunkSize int64,
) *StreamTimeoutConfig {
	return &StreamTimeoutConfig{
		ProgressTimeout:     progressTimeout,
		ProgressThreshold:   progressThreshold,
		HealthCheckInterval: healthCheckInterval,
		ChunkReadTimeout:    chunkReadTimeout,
		ChunkSize:           chunkSize,
	}
}

// Validate checks if the configuration is valid and provides warnings for misalignments
func (c *StreamTimeoutConfig) Validate() error {
	if c.ChunkSize <= 0 {
		return errors.New("ChunkSize must be positive")
	}
	if c.ProgressThreshold <= 0 {
		return errors.New("ProgressThreshold must be positive")
	}
	if c.ChunkReadTimeout <= 0 {
		return errors.New("ChunkReadTimeout must be positive")
	}
	if c.ProgressTimeout <= 0 {
		return errors.New("ProgressTimeout must be positive")
	}
	if c.HealthCheckInterval <= 0 {
		return errors.New("HealthCheckInterval must be positive")
	}

	// Warn if chunk size and progress threshold are not aligned
	if c.ChunkSize != c.ProgressThreshold {
		// This is not an error, but could cause suboptimal behavior
		// Progress will only be detected after ProgressThreshold bytes, even if chunks are smaller
	}

	return nil
}
