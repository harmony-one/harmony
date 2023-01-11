package ratelimiter

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ethereum/go-ethereum/event"

	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	p2ptypes "github.com/harmony-one/harmony/p2p/types"
	"go.uber.org/ratelimit"
)

// RateLimiter is the interface to limit the incoming request.
// The purpose of rate limiter is to prevent the node from running out of resource
// for consensus on DDoS attacks.
type RateLimiter interface {
	LimitRequest(stid sttypes.StreamID)

	p2ptypes.LifeCycle
}

// rateLimiter is the implementation of RateLimiter.
// The rateLimiter limit request rate:
//  1. For global stream requests
//  2. For requests from a stream
//
// TODO: make request weighted in rate limiter
type rateLimiter struct {
	globalLimiter ratelimit.Limiter

	streamRate int
	limiters   map[sttypes.StreamID]ratelimit.Limiter
	sm         streammanager.Subscriber

	lock   sync.RWMutex
	closeC chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(sm streammanager.Subscriber, global, single int) RateLimiter {
	return &rateLimiter{
		globalLimiter: ratelimit.New(global),

		streamRate: single,
		limiters:   make(map[sttypes.StreamID]ratelimit.Limiter),
		sm:         sm,

		closeC: make(chan struct{}),
	}
}

// Start start the rate limiter
func (rl *rateLimiter) Start() {
	rmStC := make(chan streammanager.EvtStreamRemoved)
	sub := rl.sm.SubscribeRemoveStreamEvent(rmStC)

	go rl.rmStreamLoop(rmStC, sub)
}

// Close close the rate limiter
func (rl *rateLimiter) Close() {
	close(rl.closeC)
}

func (rl *rateLimiter) rmStreamLoop(rmStC chan streammanager.EvtStreamRemoved, sub event.Subscription) {
	defer sub.Unsubscribe()

	for {
		select {
		case evt := <-rmStC:
			stid := evt.ID
			rl.unRegStream(stid)

		case <-rl.closeC:
			return
		}
	}
}

func (rl *rateLimiter) LimitRequest(stid sttypes.StreamID) {
	serverRequestCounter.Inc()
	timer := prometheus.NewTimer(serverRequestDelayDuration)
	defer timer.ObserveDuration()

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

func (rl *rateLimiter) unRegStream(stid sttypes.StreamID) {
	rl.lock.Lock()
	delete(rl.limiters, stid)
	defer rl.lock.Unlock()
}
