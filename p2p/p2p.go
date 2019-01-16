package p2p

import (
	"net"

	"github.com/dedis/kyber"
	peer "github.com/libp2p/go-libp2p-peer"
	multiaddr "github.com/multiformats/go-multiaddr"
)

// StreamHandler handles incoming p2p message.
type StreamHandler func(Stream)

// Peer is the object for a p2p peer (node)
type Peer struct {
	IP          string                // IP address of the peer
	Port        string                // Port number of the peer
	PubKey      kyber.Point           // Public key of the peer
	Ready       bool                  // Ready is true if the peer is ready to join consensus.
	ValidatorID int                   // -1 is the default value, means not assigned any validator ID in the shard
	Addrs       []multiaddr.Multiaddr // MultiAddress of the peer
	PeerID      peer.ID               // PeerID
	// TODO(minhdoan, rj): use this Ready to not send/broadcast to this peer if it wasn't available.
}

func (p Peer) String() string { return net.JoinHostPort(p.IP, p.Port) }
