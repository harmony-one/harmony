package utils

import (
	"time"
)

// TimeoutState indicates the state of Timeout class
type TimeoutState int

// Enum for different TimeoutState
const (
	Active TimeoutState = iota
	Inactive
	Expired
)

// Timeout is the implementation of timeout
type Timeout struct {
	state TimeoutState
	d     time.Duration
	start time.Time
}

// NewTimeout creates a new timeout class
func NewTimeout(d time.Duration) *Timeout {
	timeout := Timeout{state: Inactive, d: d, start: time.Now()}
	return &timeout
}

// Start starts the timeout clock
func (timeout *Timeout) Start() {
	timeout.state = Active
	timeout.start = time.Now()
}

// Stop stops the timeout clock
func (timeout *Timeout) Stop() {
	timeout.state = Inactive
	timeout.start = time.Now()
}

// Expired checks whether the timeout is reached/expired
func (timeout *Timeout) Expired(now time.Time) bool {
	if timeout.state == Active && now.Sub(timeout.start) > timeout.d {
		timeout.state = Expired
	}
	if timeout.state == Expired {
		return true
	}
	return false
}

// Duration returns the duration period of timeout
func (timeout *Timeout) Duration() time.Duration {
	return timeout.d
}

// SetDuration set new duration for the timer
func (timeout *Timeout) SetDuration(nd time.Duration) {
	timeout.d = nd
}

// IsActive checks whether timeout clock is active;
// A timeout is active means it's not stopped caused by stop
// and also not expired with time elapses longer than duration from start
func (timeout *Timeout) IsActive() bool {
	return timeout.state == Active
}
