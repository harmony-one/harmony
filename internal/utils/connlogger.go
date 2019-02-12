package utils

import (
	"github.com/ethereum/go-ethereum/log"
	net "github.com/libp2p/go-libp2p-net"
	ma "github.com/multiformats/go-multiaddr"
)

type connLogger struct{}

func (connLogger) Listen(net net.Network, ma ma.Multiaddr) {
	log.Debug("[CONNECTIONS] Listener starting", "net", net, "addr", ma)
}

func (connLogger) ListenClose(net net.Network, ma ma.Multiaddr) {
	log.Debug("[CONNECTIONS] Listener closing", "net", net, "addr", ma)
}

func (connLogger) Connected(net net.Network, conn net.Conn) {
	log.Debug("[CONNECTIONS] Connected", "net", net,
		"localPeer", conn.LocalPeer(), "localAddr", conn.LocalMultiaddr(),
		"remotePeer", conn.RemotePeer(), "remoteAddr", conn.RemoteMultiaddr(),
	)
}

func (connLogger) Disconnected(net net.Network, conn net.Conn) {
	log.Debug("[CONNECTIONS] Disconnected", "net", net,
		"localPeer", conn.LocalPeer(), "localAddr", conn.LocalMultiaddr(),
		"remotePeer", conn.RemotePeer(), "remoteAddr", conn.RemoteMultiaddr(),
	)
}

func (connLogger) OpenedStream(net net.Network, stream net.Stream) {
	conn := stream.Conn()
	log.Debug("[CONNECTIONS] Stream opened", "net", net,
		"localPeer", conn.LocalPeer(), "localAddr", conn.LocalMultiaddr(),
		"remotePeer", conn.RemotePeer(), "remoteAddr", conn.RemoteMultiaddr(),
		"protocol", stream.Protocol(),
	)
}

func (connLogger) ClosedStream(net net.Network, stream net.Stream) {
	conn := stream.Conn()
	log.Debug("[CONNECTIONS] Stream closed", "net", net,
		"localPeer", conn.LocalPeer(), "localAddr", conn.LocalMultiaddr(),
		"remotePeer", conn.RemotePeer(), "remoteAddr", conn.RemoteMultiaddr(),
		"protocol", stream.Protocol(),
	)
}

// ConnLogger is a LibP2P connection logger.
// Add on a LibP2P host by calling:
//
//   host.Network().Notify(utils.ConnLogger)
//
// It logs all listener/connection/stream open/close activities at debug level.
var ConnLogger connLogger
