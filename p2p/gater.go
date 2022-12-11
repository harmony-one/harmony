package p2p

import (
	libp2p_dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type Gater struct {
	isGating bool
}

func NewGater(disablePrivateIPScan bool) connmgr.ConnectionGater {
	return Gater{
		isGating: disablePrivateIPScan,
	}
}

func (gater Gater) InterceptPeerDial(p peer.ID) (allow bool) {
	return true
}

// Blocking connections at this stage is typical for address filtering.
func (gater Gater) InterceptAddrDial(p peer.ID, m ma.Multiaddr) (allow bool) {
	if gater.isGating {
		return libp2p_dht.PublicQueryFilter(nil, peer.AddrInfo{
			ID:    p,
			Addrs: []ma.Multiaddr{m},
		})
	} else {
		return true
	}
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
