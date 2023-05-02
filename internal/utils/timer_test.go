package utils

import (
	"testing"
	"time"
)

func TestNewTimeout(t *testing.T) {
	timer := NewTimeout(time.Second)
	if timer == nil || timer.Duration() != time.Second || timer.IsActive() {
		t.Fatalf("timer initialization error")
	}
}

func TestCheckExpire(t *testing.T) {
	timer := NewTimeout(time.Second)
	timer.Start()
	now := time.Now()
	if timer.Expired(now) {
		t.Fatalf("Timer shouldn't be expired")
	}
	if !timer.Expired(now.Add(2 * time.Second)) {
		t.Fatalf("Timer should be expired")
	}
	// start again
	timer.Start()
	if timer.Expired(now) {
		t.Fatalf("Timer shouldn't be expired")
	}
	if !timer.Expired(now.Add(2 * time.Second)) {
		t.Fatalf("Timer should be expired")
	}
	// stop
	timer.Stop()
	if timer.Expired(now) {
		t.Fatalf("Timer shouldn't be expired because it is stopped")
	}
	if timer.Expired(now.Add(2 * time.Second)) {
		t.Fatalf("Timer shouldn't be expired because it is stopped")
	}
}
