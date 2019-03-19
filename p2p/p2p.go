package p2p

import (
	"fmt"
	"net"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/bls/ffi/go/bls"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
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
	return fmt.Sprintf("%s/%s[%d]", net.JoinHostPort(p.IP, p.Port), p.PeerID, len(p.Addrs))
}

// GetAddressHex returns the hex string of the address of consensus pubKey.
func (p Peer) GetAddressHex() string {
	addr := common.Address{}
	addrBytes := p.ConsensusPubKey.GetAddress()
	addr.SetBytes(addrBytes[:])
	return addr.Hex()
}
