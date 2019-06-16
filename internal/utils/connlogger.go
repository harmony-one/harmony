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

func netLogger(n net.Network, l log.Logger) log.Logger {
	return l.New("net", n)
}

func connLogger(c net.Conn, l log.Logger) log.Logger {
	return l.New(
		"connLocalPeer", c.LocalPeer(),
		"connLocalAddr", c.LocalMultiaddr(),
		"connRemotePeer", c.RemotePeer(),
		"connRemoteAddr", c.RemoteMultiaddr())
}

func streamLogger(s net.Stream, l log.Logger) log.Logger {
	return connLogger(s.Conn(), l).New("streamProtocolID", s.Protocol())
}

func (cl ConnLogger) Listen(n net.Network, ma ma.Multiaddr) {
	WithCaller(netLogger(n, cl.l)).Debug("listener starting", "addr", ma)
}

func (cl ConnLogger) ListenClose(n net.Network, ma ma.Multiaddr) {
	WithCaller(netLogger(n, cl.l)).Debug("listener closing", "addr", ma)
}

func (cl ConnLogger) Connected(n net.Network, c net.Conn) {
	WithCaller(connLogger(c, netLogger(n, cl.l))).Debug("connected")
}

func (cl ConnLogger) Disconnected(n net.Network, c net.Conn) {
	WithCaller(connLogger(c, netLogger(n, cl.l))).Debug("disconnected")
}

func (cl ConnLogger) OpenedStream(n net.Network, s net.Stream) {
	WithCaller(streamLogger(s, netLogger(n, cl.l))).Debug("stream opened")
}

func (cl ConnLogger) ClosedStream(n net.Network, s net.Stream) {
	WithCaller(streamLogger(s, netLogger(n, cl.l))).Debug("stream closed")
}

// NewConnLogger returns a new connection logger that uses the given
// Ethereum logger.  See ConnLogger for usage.
func NewConnLogger(l log.Logger) *ConnLogger {
	return &ConnLogger{l: l}
}

// RootConnLogger is a LibP2P connection logger that logs to Ethereum root
// logger.  See ConnLogger for usage.
var RootConnLogger = NewConnLogger(log.Root())
