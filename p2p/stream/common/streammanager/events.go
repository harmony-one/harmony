package streammanager

import (
	"github.com/ethereum/go-ethereum/event"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

// EvtStreamAdded is the event of adding a new stream
type (
	EvtStreamAdded struct {
		Stream sttypes.Stream
	}

	// EvtStreamRemoved is an event of stream removed
	EvtStreamRemoved struct {
		ID sttypes.StreamID
	}
)

// SubscribeAddStreamEvent subscribe the add stream event
func (sm *streamManager) SubscribeAddStreamEvent(ch chan<- EvtStreamAdded) event.Subscription {
	return sm.addStreamFeed.Subscribe(ch)
}

// SubscribeRemoveStreamEvent subscribe the remove stream event
func (sm *streamManager) SubscribeRemoveStreamEvent(ch chan<- EvtStreamRemoved) event.Subscription {
	return sm.removeStreamFeed.Subscribe(ch)
}
