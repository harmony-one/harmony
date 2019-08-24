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
	time.Sleep(2 * time.Second)
	if timer.CheckExpire() == false {
		t.Fatalf("CheckExpire should be true")
	}
	timer.Start()
	if timer.CheckExpire() == true {
		t.Fatalf("CheckExpire should be false")
	}
	timer.Stop()
	if timer.CheckExpire() == true {
		t.Fatalf("CheckExpire should be false")
	}

}
