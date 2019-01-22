package p2p

import "github.com/libp2p/go-libp2p-peer"

// Host is the client + server in p2p network.
type Host interface {
	GetSelfPeer() Peer
	SendMessage(Peer, []byte) error
	BindHandlerAndServe(handler StreamHandler)
	Close() error
	AddPeer(*Peer) error
	GetID() peer.ID
}
