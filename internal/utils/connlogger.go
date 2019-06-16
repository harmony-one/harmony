package utils

import (
	"github.com/ethereum/go-ethereum/log"
	net "github.com/libp2p/go-libp2p-net"
	ma "github.com/multiformats/go-multiaddr"
)

// ConnLogger is a LibP2P connection logger that logs to an Ethereum logger.
// It logs all listener/connection/stream open/close activities at debug level.
// To use one, add it on a LibP2P host swarm as a notifier, ex:
//
//   connLogger := utils.NewConnLogger(
//   host.Network().Notify(connLogger)
type ConnLogger struct {
	l log.Logger
}

func (cl ConnLogger) Listen(n net.Network, ma ma.Multiaddr) {
	WithCaller(cl.l).Debug("listener starting", "net", n, "addr", ma)
}

func (cl ConnLogger) ListenClose(n net.Network, ma ma.Multiaddr) {
	WithCaller(cl.l).Debug("listener closing", "net", n, "addr", ma)
}

func (cl ConnLogger) Connected(n net.Network, c net.Conn) {
	WithCaller(cl.l).Debug("connected", "net", n,
		"localPeer", c.LocalPeer(), "localAddr", c.LocalMultiaddr(),
		"remotePeer", c.RemotePeer(), "remoteAddr", c.RemoteMultiaddr(),
	)
}

func (cl ConnLogger) Disconnected(n net.Network, c net.Conn) {
	WithCaller(cl.l).Debug("disconnected", "net", n,
		"localPeer", c.LocalPeer(), "localAddr", c.LocalMultiaddr(),
		"remotePeer", c.RemotePeer(), "remoteAddr", c.RemoteMultiaddr(),
	)
}

func (cl ConnLogger) OpenedStream(n net.Network, s net.Stream) {
	conn := s.Conn()
	WithCaller(cl.l).Debug("stream opened", "net", n,
		"localPeer", conn.LocalPeer(), "localAddr", conn.LocalMultiaddr(),
		"remotePeer", conn.RemotePeer(), "remoteAddr", conn.RemoteMultiaddr(),
		"protocol", s.Protocol(),
	)
}

func (cl ConnLogger) ClosedStream(n net.Network, s net.Stream) {
	conn := s.Conn()
	WithCaller(cl.l).Debug("stream closed", "net", n,
		"localPeer", conn.LocalPeer(), "localAddr", conn.LocalMultiaddr(),
		"remotePeer", conn.RemotePeer(), "remoteAddr", conn.RemoteMultiaddr(),
		"protocol", s.Protocol(),
	)
}

// NewConnLogger returns a new connection logger that uses the given
// Ethereum logger.  See ConnLogger for usage.
func NewConnLogger(l log.Logger) *ConnLogger {
	return &ConnLogger{l: l}
}

// RootConnLogger is a LibP2P connection logger that logs to Ethereum root
// logger.  See ConnLogger for usage.
var RootConnLogger = NewConnLogger(log.Root())
