package p2p

import (
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	libp2p_dht "github.com/libp2p/go-libp2p-kad-dht"
	ma "github.com/multiformats/go-multiaddr"
)

type Gater struct{}

func NewGater(disablePrivateIPScan bool) connmgr.ConnectionGater {
	if !disablePrivateIPScan {
		return nil
	}
	return Gater{}
}

func (gater Gater) InterceptPeerDial(p peer.ID) (allow bool) {
	return true
}

// Blocking connections at this stage is typical for address filtering.
func (gater Gater) InterceptAddrDial(p peer.ID, m ma.Multiaddr) (allow bool) {
	return libp2p_dht.PublicQueryFilter(nil, peer.AddrInfo{
		ID:    p,
		Addrs: []ma.Multiaddr{m},
	})
}

func (gater Gater) InterceptAccept(network.ConnMultiaddrs) (allow bool) {
	return true
}

func (gater Gater) InterceptSecured(network.Direction, peer.ID, network.ConnMultiaddrs) (allow bool) {
	return true
}

// NOTE: the go-libp2p implementation currently IGNORES the disconnect reason.
func (gater Gater) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}
