package rate

import (
	"context"
	"time"
)

// IDLimiter is the limiter interface to limit incoming traffics per ID
type IDLimiter interface {
	AllowN(id string, n int) bool
	WaitN(ctx context.Context, id string, n int) error
}

// timer is the interface of time used for testing
type timer interface {
	now() time.Time
	since(time.Time) time.Duration
	newTicker(time.Duration) *time.Ticker
}

// timerImpl is the timer for production. Use package time directly
type timerImpl struct{}

func (t timerImpl) now() time.Time                         { return time.Now() }
func (t timerImpl) since(t2 time.Time) time.Duration       { return time.Since(t2) }
func (t timerImpl) newTicker(d time.Duration) *time.Ticker { return time.NewTicker(d) }
