package sttypes

import (
	"testing"
	"time"
)

func TestProgressTracker_NewProgressTracker(t *testing.T) {
	pt := NewProgressTracker(30*time.Second, 60*time.Second, 1024)

	if pt == nil {
		t.Fatal("ProgressTracker should not be nil")
	}

	if pt.timeoutDuration != 30*time.Second {
		t.Errorf("Expected timeout duration 30s, got %v", pt.timeoutDuration)
	}

	if pt.maxIdleTime != 60*time.Second {
		t.Errorf("Expected max idle time 60s, got %v", pt.maxIdleTime)
	}

	if pt.resetThreshold != 1024 {
		t.Errorf("Expected reset threshold 1024, got %v", pt.resetThreshold)
	}
}

func TestProgressTracker_UpdateProgress(t *testing.T) {
	pt := NewProgressTracker(30*time.Second, 60*time.Second, 1024)

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

func TestProgressTracker_HasProgress(t *testing.T) {
	pt := NewProgressTracker(30*time.Second, 60*time.Second, 1024)

	// No progress initially
	if pt.HasProgress(0) {
		t.Error("Should not have progress initially")
	}

	// Update with data
	pt.UpdateProgress(2048)

	// Check progress - should have progress since we updated with 2048 bytes
	// and the threshold is 1024, so 2048 - 0 = 2048 >= 1024
	if !pt.HasProgress(2048) {
		t.Error("Should have progress after update")
	}

	// Check that calling HasProgress with the same size doesn't show progress
	if pt.HasProgress(2048) {
		t.Error("Should not show progress for same size")
	}

	// Check that calling HasProgress with larger size shows progress
	if !pt.HasProgress(4096) {
		t.Error("Should show progress for larger size")
	}
}

func TestProgressTracker_ShouldTimeout(t *testing.T) {
	pt := NewProgressTracker(100*time.Millisecond, 60*time.Second, 1024)

	// Should not timeout immediately
	if pt.ShouldTimeout() {
		t.Error("Should not timeout immediately")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Should timeout now
	if !pt.ShouldTimeout() {
		t.Error("Should timeout after waiting")
	}
}

func TestProgressTracker_ResetTimeout(t *testing.T) {
	pt := NewProgressTracker(100*time.Millisecond, 60*time.Second, 1024)

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
	pt := NewProgressTracker(100*time.Millisecond, 200*time.Millisecond, 1024)

	// Should be healthy initially
	if !pt.IsHealthy() {
		t.Error("Should be healthy initially")
	}

	// Wait for progress timeout but not idle timeout
	time.Sleep(150 * time.Millisecond)

	// Should not be healthy now (progress timeout exceeded)
	if pt.IsHealthy() {
		t.Error("Should not be healthy after progress timeout")
	}

	// Test idle timeout
	pt2 := NewProgressTracker(60*time.Second, 100*time.Millisecond, 1024)

	// Should be healthy initially
	if !pt2.IsHealthy() {
		t.Error("Should be healthy initially")
	}

	// Wait for idle timeout
	time.Sleep(150 * time.Millisecond)

	// Should not be healthy now (idle timeout exceeded)
	if pt2.IsHealthy() {
		t.Error("Should not be healthy after idle timeout")
	}
}
