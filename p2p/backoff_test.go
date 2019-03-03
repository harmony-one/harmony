package p2p

import (
	"testing"
	"time"
)

func TestNewBackoffBase(test *testing.T) {
	newBackoffBase := NewBackoffBase(time.Duration(10), time.Duration(20))
	if newBackoffBase == nil {
		test.Error("Error creating new backoff base")
	}
	newBackoffBase.Backoff()
	newBackoffBase.Reset()
}

func TestBaseBackOffSleep(test *testing.T) {
	newBackoffBase := NewBackoffBase(time.Duration(1), time.Duration(10))
	(*newBackoffBase).Sleep()
	min := (*newBackoffBase).Min
	curr := (*newBackoffBase).Cur
	if min != curr {
		test.Error("Default implementation shouldn't backoff")
	}
}

func TestNewExpBackoff(test *testing.T) {
	newExpBackoff := NewExpBackoff(time.Duration(1), time.Duration(10), 0.1)
	if newExpBackoff == nil {
		test.Error("Error creating new backoff base")
	}
}

func TestExpBackoffSleep(test *testing.T) {
	newExpBackoff := NewExpBackoff(time.Duration(1), time.Duration(10), 0.1)
	(*newExpBackoff).Sleep()
}

func TestExpBackoffAdjustment(test *testing.T) {
	newExpBackoff := NewExpBackoff(time.Duration(1), time.Duration(10), 0.1)
	newExpBackoff.Backoff()
	expectedDuration := time.Duration(float64(newExpBackoff.Cur) * newExpBackoff.Factor)
	actualDuration := newExpBackoff.Cur

	if expectedDuration.Nanoseconds() != actualDuration.Nanoseconds() {
		test.Error("Duration doesn't match")
	}
}

func TestExpBackoffReset(test *testing.T) {
	newExpBackoff := NewExpBackoff(time.Duration(1), time.Duration(10), 0.1)
	newExpBackoff.Reset()
}
