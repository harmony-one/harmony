package streammanager

import (
	"context"

	"github.com/ethereum/go-ethereum/event"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	p2ptypes "github.com/harmony-one/harmony/p2p/types"
	"github.com/libp2p/go-libp2p/core/network"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// StreamManager is the interface for streamManager
type StreamManager interface {
	p2ptypes.LifeCycle
	Operator
	Subscriber
	Reader
}

// ReaderSubscriber reads stream and subscribe stream events
type ReaderSubscriber interface {
	Reader
	Subscriber
}

// Operator handles new stream or remove stream
type Operator interface {
	NewStream(stream sttypes.Stream) error
	RemoveStream(stID sttypes.StreamID) error
}

// Subscriber is the interface to support stream event subscription
type Subscriber interface {
	SubscribeAddStreamEvent(ch chan<- EvtStreamAdded) event.Subscription
	SubscribeRemoveStreamEvent(ch chan<- EvtStreamRemoved) event.Subscription
}

// Reader is the interface to read stream in stream manager
type Reader interface {
	GetStreams() []sttypes.Stream
	GetStreamByID(id sttypes.StreamID) (sttypes.Stream, bool)
}

// host is the adapter interface of the libp2p host implementation.
// TODO: further adapt the host
type host interface {
	ID() libp2p_peer.ID
	NewStream(ctx context.Context, p libp2p_peer.ID, pids ...protocol.ID) (network.Stream, error)
}

// peerFinder is the adapter interface of discovery.Discovery
type peerFinder interface {
	FindPeers(ctx context.Context, ns string, peerLimit int) (<-chan libp2p_peer.AddrInfo, error)
}
