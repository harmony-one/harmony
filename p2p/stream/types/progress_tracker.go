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
	rateWindow       []int64 // Sliding window for rate calculation
	rateWindowSize   int     // Size of the rate calculation window
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
		rateWindowSize:   10, // Track last 10 updates for rate calculation
		rateWindow:       make([]int64, 0, 10),
	}
}

// UpdateProgress updates the progress tracker with new data received
func (pt *ProgressTracker) UpdateProgress(newSize int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.totalBytesRead += int64(newSize)

	// Only update activity time if we actually received data
	if newSize > 0 {
		pt.lastActivityTime = time.Now()

		// Add to rate window
		pt.rateWindow = append(pt.rateWindow, int64(newSize))
		if len(pt.rateWindow) > pt.rateWindowSize {
			pt.rateWindow = pt.rateWindow[1:] // Remove oldest entry
		}
	}

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
	// Both conditions must be met for the stream to be considered healthy
	return timeSinceProgress <= pt.timeoutDuration && timeSinceActivity <= pt.maxIdleTime
}

// GetProgressRate calculates the current progress rate in bytes per second
func (pt *ProgressTracker) GetProgressRate() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if len(pt.rateWindow) < 2 {
		return 0 // Need at least 2 updates to calculate rate
	}

	// Calculate rate based on recent updates in the window
	// This gives us a more accurate picture of current transfer rate
	totalBytes := int64(0)
	for _, bytes := range pt.rateWindow {
		totalBytes += bytes
	}

	// Assume updates happen roughly every 100ms for rate calculation
	// This is a reasonable assumption for stream sync
	timeWindow := time.Duration(len(pt.rateWindow)) * 100 * time.Millisecond

	return float64(totalBytes) / timeWindow.Seconds()
}
