package gating

import (
	libp2p_dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// ExpiryConnectionGater enhances a ExtendedConnectionGater by implementing ban-expiration
type BlockingConnectionGater struct {
	ExtendedConnectionGater
	isGating bool
}

func AddBlocking(gater ExtendedConnectionGater, disablePrivateIPScan bool) *BlockingConnectionGater {
	return &BlockingConnectionGater{
		ExtendedConnectionGater: gater,
		isGating:                disablePrivateIPScan,
	}
}

// Blocking connections at this stage is typical for address filtering.
func (g *BlockingConnectionGater) InterceptAddrDial(p peer.ID, m ma.Multiaddr) (allow bool) {
	if g.isGating {
		return libp2p_dht.PublicQueryFilter(nil, peer.AddrInfo{
			ID:    p,
			Addrs: []ma.Multiaddr{m},
		})
	}
	return true
}
