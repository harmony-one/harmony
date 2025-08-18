package sttypes

import (
	"sync"
	"time"
)

// ProgressTracker monitors data transfer progress and implements progress-based timeouts
type ProgressTracker struct {
	mu               sync.RWMutex
	lastDataSize     int64
	lastProgressTime time.Time
	timeoutDuration  time.Duration
	resetThreshold   int64
	maxIdleTime      time.Duration
	totalBytesRead   int64
	lastActivityTime time.Time
	lastCheckedSize  int64
}

// NewProgressTracker creates a new progress tracker with the given configuration
func NewProgressTracker(timeoutDuration, maxIdleTime time.Duration, resetThreshold int64) *ProgressTracker {
	now := time.Now()
	return &ProgressTracker{
		lastProgressTime: now,
		lastActivityTime: now,
		timeoutDuration:  timeoutDuration,
		maxIdleTime:      maxIdleTime,
		resetThreshold:   resetThreshold,
	}
}

// UpdateProgress updates the progress tracker with new data received
func (pt *ProgressTracker) UpdateProgress(newSize int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.totalBytesRead += int64(newSize)
	pt.lastActivityTime = time.Now()

	// Check if we made significant progress
	if newSize >= int(pt.resetThreshold) {
		pt.lastProgressTime = time.Now()
		pt.lastDataSize = pt.totalBytesRead
	}
}

// HasProgress checks if the stream has made progress since last check
func (pt *ProgressTracker) HasProgress(newSize int) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	progress := int64(newSize) - pt.lastCheckedSize
	hasProgress := progress >= pt.resetThreshold

	if hasProgress {
		pt.lastCheckedSize = int64(newSize)
	}

	return hasProgress
}

// ResetTimeout resets the progress timeout
func (pt *ProgressTracker) ResetTimeout() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.lastProgressTime = time.Now()
	pt.lastDataSize = 0
	pt.lastCheckedSize = 0
}

// ShouldTimeout checks if the stream should timeout due to lack of progress
func (pt *ProgressTracker) ShouldTimeout() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	timeSinceProgress := time.Since(pt.lastProgressTime)
	return timeSinceProgress > pt.timeoutDuration
}

// ShouldIdleTimeout checks if the stream should timeout due to inactivity
func (pt *ProgressTracker) ShouldIdleTimeout() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	timeSinceActivity := time.Since(pt.lastActivityTime)
	return timeSinceActivity > pt.maxIdleTime
}

// GetStats returns current progress statistics
func (pt *ProgressTracker) GetStats() (totalBytes int64, lastProgress time.Time, lastActivity time.Time) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.totalBytesRead, pt.lastProgressTime, pt.lastActivityTime
}

// IsHealthy checks if the stream is healthy based on progress
func (pt *ProgressTracker) IsHealthy() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	timeSinceProgress := time.Since(pt.lastProgressTime)
	timeSinceActivity := time.Since(pt.lastActivityTime)

	// Stream is healthy if it's made progress recently AND had activity recently
	return timeSinceProgress <= pt.timeoutDuration && timeSinceActivity <= pt.maxIdleTime
}
