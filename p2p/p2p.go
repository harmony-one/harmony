package p2p

import (
	"fmt"
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
	PubKey      kyber.Point           // Public key of the peer, used for consensus signing
	Ready       bool                  // Ready is true if the peer is ready to join consensus. (FIXME: deprecated)
	ValidatorID int                   // -1 is the default value, means not assigned any validator ID in the shard
	Addrs       []multiaddr.Multiaddr // MultiAddress of the peer
	PeerID      peer.ID               // PeerID, the pubkey for communication
}

func (p Peer) String() string {
	return fmt.Sprintf("%s/%s[%d]", net.JoinHostPort(p.IP, p.Port), p.PeerID, len(p.Addrs))
}
