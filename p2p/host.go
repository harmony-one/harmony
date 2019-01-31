package p2p

import (
	p2p_host "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Host is the client + server in p2p network.
type Host interface {
	GetSelfPeer() Peer
	SendMessage(Peer, []byte) error
	BindHandlerAndServe(handler StreamHandler)
	Close() error
	AddPeer(*Peer) error
	GetID() peer.ID
	GetP2PHost() p2p_host.Host
}
