package p2pimpl

import (
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/p2p/host/hostv1"
	"github.com/harmony-one/harmony/p2p/host/hostv2"
)

// Version The version number of p2p library
// 1 - Direct socket connection
// 2 - libp2p
const Version = 1

// NewHost starts the host
func NewHost(peer p2p.Peer) host.Host {
	// log.Debug("New Host", "ip/port", net.JoinHostPort(peer.IP, peer.Port))
	if Version == 1 {
		h := hostv1.New(peer)
		return h
	}

	h := hostv2.New(peer)
	return h
}
