package ratelimiter

import (
	"sync"

	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"go.uber.org/ratelimit"
)

// RateLimiter is the interface to limit the incoming request.
// The purpose of rate limiter is to prevent the node from running out of resource
// for consensus on DDoS attacks.
type RateLimiter interface {
	LimitRequest(stid sttypes.StreamID)
}

// rateLimiter is the implementation of RateLimiter which only blocks for the global
// level
// TODO: make request weighted in rate limiter
type rateLimiter struct {
	globalLimiter ratelimit.Limiter

	streamRate int
	limiters   map[sttypes.StreamID]ratelimit.Limiter

	lock sync.RWMutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(global, single int) RateLimiter {
	return &rateLimiter{
		globalLimiter: ratelimit.New(global),

		streamRate: single,
		limiters:   make(map[sttypes.StreamID]ratelimit.Limiter),
	}
}

func (rl *rateLimiter) LimitRequest(stid sttypes.StreamID) {
	rl.lock.RLock()
	limiter, ok := rl.limiters[stid]
	rl.lock.RUnlock()

	// double lock to ensure concurrency
	if !ok {
		rl.lock.Lock()
		if limiter2, ok := rl.limiters[stid]; ok {
			limiter = limiter2
		} else {
			limiter = ratelimit.New(rl.streamRate)
			rl.limiters[stid] = limiter
		}
		rl.lock.Unlock()
	}

	rl.globalLimiter.Take()
	limiter.Take()
}
