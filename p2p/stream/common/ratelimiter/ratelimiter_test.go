package ratelimiter

import (
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

func TestRateLimiter(t *testing.T) {
	sm := &testStreamManager{}
	rl := NewRateLimiter(sm, 10, 10)
	rl.Start()
	defer rl.Close()

	stid := makeTestStreamID(1)
	rl.LimitRequest(stid)
	sm.removeStream(stid)

	time.Sleep(100 * time.Millisecond)
	rlImpl := rl.(*rateLimiter)
	rlImpl.lock.Lock()
	if _, ok := rlImpl.limiters[stid]; ok {
		t.Errorf("after remove, the limiter still in rate limiter")
	}
	rlImpl.lock.Unlock()
}

type testStreamManager struct {
	removeFeed event.Feed
}

func (sm *testStreamManager) SubscribeAddStreamEvent(ch chan<- streammanager.EvtStreamAdded) event.Subscription {
	return nil
}

func (sm *testStreamManager) SubscribeRemoveStreamEvent(ch chan<- streammanager.EvtStreamRemoved) event.Subscription {
	return sm.removeFeed.Subscribe(ch)
}

func (sm *testStreamManager) removeStream(stid sttypes.StreamID) {
	sm.removeFeed.Send(streammanager.EvtStreamRemoved{ID: stid})
}

func makeTestStreamID(index int) sttypes.StreamID {
	s := fmt.Sprintf("test stream %v", index)
	return sttypes.StreamID(s)
}
