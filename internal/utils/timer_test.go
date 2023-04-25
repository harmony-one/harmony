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
	now := time.Now().Add(2 * time.Second)
	if timer.CheckExpire(now) == false {
		t.Fatalf("CheckExpire should be true")
	}
	timer.Start()
	if timer.CheckExpire(now) == true {
		t.Fatalf("CheckExpire should be false")
	}
	timer.Stop()
	if timer.CheckExpire(now) == true {
		t.Fatalf("CheckExpire should be false")
	}
}
