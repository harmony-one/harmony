package p2p

import (
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	libp2p_host "github.com/libp2p/go-libp2p-host"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
)

// Host is the client + server in p2p network.
type Host interface {
	GetSelfPeer() Peer
	Close() error
	AddPeer(*Peer) error
	GetID() libp2p_peer.ID
	GetP2PHost() libp2p_host.Host
	GetPeerCount() int

	//AddIncomingPeer(Peer)
	//AddOutgoingPeer(Peer)
	ConnectHostPeer(Peer)

	// SendMessageToGroups sends a message to one or more multicast groups.
	SendMessageToGroups(groups []nodeconfig.GroupID, msg []byte) error

	// GroupReceiver returns a receiver of messages sent to a multicast group.
	// Each call creates a new receiver.
	// If multiple receivers are created for the same group,
	// a message sent to the group will be delivered to all of the receivers.
	GroupReceiver(nodeconfig.GroupID) (receiver GroupReceiver, err error)
}
