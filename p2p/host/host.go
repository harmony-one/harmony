package host

import (
	"github.com/harmony-one/harmony/p2p"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Host is the client + server in p2p network.
type Host interface {
	GetSelfPeer() p2p.Peer
	SendMessage(p2p.Peer, []byte) error
	BindHandlerAndServe(handler p2p.StreamHandler)
	Close() error
	AddPeer(*p2p.Peer) error
	GetID() peer.ID
}
