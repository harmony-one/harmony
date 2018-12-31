package host

import (
	"github.com/harmony-one/harmony-public/pkg/p2p"
)

// Host is the client + server in p2p network.
type Host interface {
	GetSelfPeer() p2p.Peer
	SendMessage(p2p.Peer, []byte) error
	BindHandlerAndServe(handler p2p.StreamHandler)
	Close() error
}
