package p2p

import (
	"fmt"
	"net"

	"github.com/harmony-one/bls/ffi/go/bls"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
)

// StreamHandler handles incoming p2p message.
type StreamHandler func(Stream)

// Peer is the object for a p2p peer (node)
type Peer struct {
	IP          string         // IP address of the peer
	Port        string         // Port number of the peer
	BlsPubKey   *bls.PublicKey // Public key of the peer, used for consensus signing
	ValidatorID int            // -1 is the default value, means not assigned any validator ID in the shard
	Addrs       []ma.Multiaddr // MultiAddress of the peer
	PeerID      libp2p_peer.ID // PeerID, the pubkey for communication
}

func (p Peer) String() string {
	return fmt.Sprintf("%s/%s[%d]", net.JoinHostPort(p.IP, p.Port), p.PeerID, len(p.Addrs))
}
