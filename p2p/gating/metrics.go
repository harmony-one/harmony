package gating

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type ConnectionGaterMetrics interface {
	RecordDial(allow bool)
	RecordAccept(allow bool)
}

type MeteredConnectionGater struct {
	BlockingConnectionGater
	//m ConnectionGaterMetrics
}

func AddMetering(gater BlockingConnectionGater) *MeteredConnectionGater {
	return &MeteredConnectionGater{BlockingConnectionGater: gater}
}

func (g *MeteredConnectionGater) InterceptPeerDial(p peer.ID) (allow bool) {
	allow = g.BlockingConnectionGater.InterceptPeerDial(p)
	return allow
}

func (g *MeteredConnectionGater) InterceptAddrDial(id peer.ID, ma multiaddr.Multiaddr) (allow bool) {
	allow = g.BlockingConnectionGater.InterceptAddrDial(id, ma)
	return allow
}

func (g *MeteredConnectionGater) InterceptAccept(mas network.ConnMultiaddrs) (allow bool) {
	allow = g.BlockingConnectionGater.InterceptAccept(mas)
	return allow
}

func (g *MeteredConnectionGater) InterceptSecured(dir network.Direction, id peer.ID, mas network.ConnMultiaddrs) (allow bool) {
	allow = g.BlockingConnectionGater.InterceptSecured(dir, id, mas)
	return allow
}
