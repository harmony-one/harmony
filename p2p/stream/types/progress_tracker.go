package sttypes

import (
	"sync"
	"time"
)

// ProgressTracker monitors data transfer progress during content reading operations
// It implements a simple, focused approach that only tracks reading progress
// and provides timeout functionality when no progress is made
type ProgressTracker struct {
	mu               sync.RWMutex
	lastProgressTime time.Time
	timeoutDuration  time.Duration
	resetThreshold   int64
	totalBytesRead   int64
}

// NewProgressTracker creates a new progress tracker with the given configuration
func NewProgressTracker(timeoutDuration time.Duration, resetThreshold int64) *ProgressTracker {
	now := time.Now()
	return &ProgressTracker{
		lastProgressTime: now,
		timeoutDuration:  timeoutDuration,
		resetThreshold:   resetThreshold,
	}
}

// UpdateProgress updates the progress tracker with new data received during content reading
func (pt *ProgressTracker) UpdateProgress(newSize int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.totalBytesRead += int64(newSize)

	// Check if we made significant progress
	if newSize >= int(pt.resetThreshold) {
		pt.lastProgressTime = time.Now()
	}
}

// ResetTimeout resets the progress timeout - called when progress is detected
func (pt *ProgressTracker) ResetTimeout() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.lastProgressTime = time.Now()
}

// ShouldTimeout checks if the stream should timeout due to lack of progress during content reading
func (pt *ProgressTracker) ShouldTimeout() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	timeSinceProgress := time.Since(pt.lastProgressTime)
	return timeSinceProgress > pt.timeoutDuration
}

// GetStats returns current progress statistics
func (pt *ProgressTracker) GetStats() (totalBytes int64, lastProgress time.Time) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.totalBytesRead, pt.lastProgressTime
}

// IsHealthy checks if the stream is healthy based on reading progress
func (pt *ProgressTracker) IsHealthy() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	timeSinceProgress := time.Since(pt.lastProgressTime)
	return timeSinceProgress <= pt.timeoutDuration
}

// GetProgressRate calculates the current progress rate in bytes per second
// This is a simple calculation based on total bytes read and time since creation
func (pt *ProgressTracker) GetProgressRate() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.totalBytesRead == 0 {
		return 0
	}

	timeSinceCreation := time.Since(pt.lastProgressTime.Add(-pt.timeoutDuration))
	if timeSinceCreation <= 0 {
		return 0
	}

	return float64(pt.totalBytesRead) / timeSinceCreation.Seconds()
}

// GetHealthSummary returns a simple health summary focused on reading progress
func (pt *ProgressTracker) GetHealthSummary() map[string]interface{} {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	now := time.Now()
	return map[string]interface{}{
		"totalBytesRead":    pt.totalBytesRead,
		"lastProgressTime":  pt.lastProgressTime,
		"timeSinceProgress": now.Sub(pt.lastProgressTime).String(),
		"timeoutDuration":   pt.timeoutDuration.String(),
		"resetThreshold":    pt.resetThreshold,
		"isHealthy":         pt.IsHealthy(),
		"shouldTimeout":     pt.ShouldTimeout(),
	}
}
