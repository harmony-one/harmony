package p2p

import (
	"fmt"
	"github.com/harmony-one/bls/ffi/go/bls"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
	"net"
)

// StreamHandler handles incoming p2p message.
type StreamHandler func(Stream)

// Peer is the object for a p2p peer (node)
type Peer struct {
	IP              string         // IP address of the peer
	Port            string         // Port number of the peer
	ConsensusPubKey *bls.PublicKey // Public key of the peer, used for consensus signing
	Addrs           []ma.Multiaddr // MultiAddress of the peer
	PeerID          libp2p_peer.ID // PeerID, the pubkey for communication
}

func (p Peer) String() string {
	BlsPubKey := "nil"
	if p.ConsensusPubKey != nil {
		BlsPubKey = p.ConsensusPubKey.SerializeToHexStr()
	}
	return fmt.Sprintf("BlsPubKey:%s-%s/%s[%d]", BlsPubKey, net.JoinHostPort(p.IP, p.Port), p.PeerID, len(p.Addrs))
}
