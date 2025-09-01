package sttypes

import (
	"testing"
	"time"
)

func TestProgressTracker_NewProgressTracker(t *testing.T) {
	pt := NewProgressTracker(30*time.Second, 1024)

	if pt == nil {
		t.Fatal("ProgressTracker should not be nil")
	}

	if pt.timeoutDuration != 30*time.Second {
		t.Errorf("Expected timeout duration 30s, got %v", pt.timeoutDuration)
	}

	if pt.resetThreshold != 1024 {
		t.Errorf("Expected reset threshold 1024, got %v", pt.resetThreshold)
	}

	// Initially not tracking
	if pt.IsTracking() {
		t.Error("Should not be tracking initially")
	}
}

func TestProgressTracker_UpdateProgress(t *testing.T) {
	pt := NewProgressTracker(30*time.Second, 1024)

	// Update with data smaller than threshold
	pt.UpdateProgress(512)

	if pt.totalBytesRead != 512 {
		t.Errorf("Expected total bytes 512, got %v", pt.totalBytesRead)
	}

	// Update with data larger than threshold
	pt.UpdateProgress(2048)

	if pt.totalBytesRead != 2560 {
		t.Errorf("Expected total bytes 2560, got %v", pt.totalBytesRead)
	}
}

func TestProgressTracker_ShouldTimeout(t *testing.T) {
	pt := NewProgressTracker(100*time.Millisecond, 1024)

	// Should not timeout when not tracking (default state)
	if pt.ShouldTimeout() {
		t.Error("Should not timeout when not tracking")
	}

	// Start tracking
	pt.StartTracking()

	// Should not timeout immediately after starting
	if pt.ShouldTimeout() {
		t.Error("Should not timeout immediately after starting")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Should timeout now
	if !pt.ShouldTimeout() {
		t.Error("Should timeout after waiting")
	}
}

func TestProgressTracker_ResetTimeout(t *testing.T) {
	pt := NewProgressTracker(100*time.Millisecond, 1024)

	// Start tracking first
	pt.StartTracking()

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Should timeout
	if !pt.ShouldTimeout() {
		t.Error("Should timeout after waiting")
	}

	// Reset timeout
	pt.ResetTimeout()

	// Should not timeout now
	if pt.ShouldTimeout() {
		t.Error("Should not timeout after reset")
	}
}

func TestProgressTracker_IsHealthy(t *testing.T) {
	pt := NewProgressTracker(100*time.Millisecond, 1024)

	// Should be healthy initially when not tracking (default state)
	if !pt.IsHealthy() {
		t.Error("Should be healthy initially when not tracking")
	}

	// Start tracking
	pt.StartTracking()

	// Should be healthy immediately after starting
	if !pt.IsHealthy() {
		t.Error("Should be healthy immediately after starting")
	}

	// Wait for progress timeout
	time.Sleep(150 * time.Millisecond)

	// Should not be healthy now (progress timeout exceeded)
	if pt.IsHealthy() {
		t.Error("Should not be healthy after progress timeout")
	}
}

func TestProgressTracker_GetProgressRate(t *testing.T) {
	pt := NewProgressTracker(30*time.Second, 1024)

	// Initial rate should be 0 (no updates yet)
	if rate := pt.GetProgressRate(); rate != 0 {
		t.Errorf("Expected initial rate 0, got %v", rate)
	}

	// Update with some data
	pt.UpdateProgress(2048)

	// Rate should be calculated based on total bytes and time
	rate := pt.GetProgressRate()
	if rate <= 0 {
		t.Errorf("Expected positive rate, got %v", rate)
	}
}

func TestProgressTracker_GetHealthSummary(t *testing.T) {
	pt := NewProgressTracker(30*time.Second, 1024)

	// Update progress
	pt.UpdateProgress(1024)

	summary := pt.GetHealthSummary()

	// Check that summary contains expected fields
	expectedFields := []string{
		"totalBytesRead", "lastProgressTime", "timeSinceProgress",
		"timeoutDuration", "resetThreshold", "isHealthy", "shouldTimeout",
	}

	for _, field := range expectedFields {
		if _, exists := summary[field]; !exists {
			t.Errorf("Health summary missing field: %s", field)
		}
	}

	// Check specific values
	if summary["totalBytesRead"] != int64(1024) {
		t.Errorf("Expected totalBytesRead 1024, got %v", summary["totalBytesRead"])
	}

	if summary["resetThreshold"] != int64(1024) {
		t.Errorf("Expected resetThreshold 1024, got %v", summary["resetThreshold"])
	}
}

func TestProgressTracker_StartTracking(t *testing.T) {
	pt := NewProgressTracker(30*time.Second, 1024)

	// Initially not tracking
	if pt.IsTracking() {
		t.Error("Should not be tracking initially")
	}

	// Start tracking
	pt.StartTracking()

	// Should be tracking now
	if !pt.IsTracking() {
		t.Error("Should be tracking after StartTracking")
	}

	// Should be healthy immediately after starting
	if !pt.IsHealthy() {
		t.Error("Should be healthy immediately after starting tracking")
	}
}

func TestProgressTracker_StopTracking(t *testing.T) {
	pt := NewProgressTracker(30*time.Second, 1024)

	// Start tracking first
	pt.StartTracking()
	if !pt.IsTracking() {
		t.Error("Should be tracking after StartTracking")
	}

	// Stop tracking
	pt.StopTracking()

	// Should not be tracking now
	if pt.IsTracking() {
		t.Error("Should not be tracking after StopTracking")
	}

	// Should always be healthy when not tracking
	if !pt.IsHealthy() {
		t.Error("Should be healthy when not tracking")
	}

	// Should never timeout when not tracking
	if pt.ShouldTimeout() {
		t.Error("Should never timeout when not tracking")
	}
}

func TestProgressTracker_IsTracking(t *testing.T) {
	pt := NewProgressTracker(30*time.Second, 1024)

	// Initially not tracking
	if pt.IsTracking() {
		t.Error("Should not be tracking initially")
	}

	// Start tracking
	pt.StartTracking()
	if !pt.IsTracking() {
		t.Error("Should be tracking after StartTracking")
	}

	// Stop tracking
	pt.StopTracking()
	if pt.IsTracking() {
		t.Error("Should not be tracking after StopTracking")
	}
}
