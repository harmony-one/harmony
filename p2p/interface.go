package p2p

import (
	"context"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	libp2p_host "github.com/libp2p/go-libp2p-core/host"

	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// Host is the client + server in p2p network.
type host interface {
	PubSubHost

	GetSelfPeer() Peer
	AddPeer(*Peer) error
	GetID() libp2p_peer.ID
	GetP2PHost() libp2p_host.Host
	GetPeerCount() int
	ConnectHostPeer(Peer) error
	PubSub() *libp2p_pubsub.PubSub
	C() (int, int, int)
	ListPeer(topic string) []libp2p_peer.ID
	ListTopic() []string
	ListBlockedPeer() []libp2p_peer.ID
}

type PubSubHost interface {
	AddPubSubHandler(psh PubSubHandler) error
	RemovePubSubHandler(spec string) error
	StartPubSubHandler(spec string) error
	StopPubSubHandler(spec string) error

	// SendMessageToGroups sends a message to one or more multicast groups.
	SendMessageToGroups(groups []nodeconfig.GroupID, msg []byte) error
}

// PubSubHandler is the pub sub message handler for a given topic
// TODO: Add version compatibility to topic
type PubSubHandler interface {
	// Topic is the topic the handler is subscribed to
	Topic() string

	// Specifier defines the uniques specifier of a pub sub handler
	Specifier() string

	// ValidateMsg validate the message from the peer with PeerID. Return the validation cache
	// and result. Returned cache is explicitly used in HandleMsg.
	ValidateMsg(ctx context.Context, peer PeerID, msg *Message) (interface{}, ValidateResult)

	// HandleMsg handle the message with the validationCache in step ValidateMsg
	HandleMsg(ctx context.Context, msg *Message, validateCache interface{}) error

	// Options return the additional option used for pub sub handler registration.
	Options() []libp2p_pubsub.ValidatorOpt
}
