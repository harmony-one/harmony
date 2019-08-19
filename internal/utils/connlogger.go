package utils

import (
	"os"
	"sync"
	"time"

	net "github.com/libp2p/go-libp2p-core/network"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

type event struct {
	ze *zerolog.Event
}

func (e event) net(n net.Network) event {
	return event{ze: e.ze.
		Interface("net_local_peer", n.LocalPeer()).
		Interface("net_listen_addrs", n.ListenAddresses())}
}

func (e event) listener(ma ma.Multiaddr) event {
	return event{ze: e.ze.Interface("listen_addr", ma)}
}

func (e event) conn(c net.Conn) event {
	return event{ze: e.ze.
		Interface("conn_local_peer", c.LocalPeer()).
		Interface("conn_local_addr", c.LocalMultiaddr()).
		Interface("conn_remote_peer", c.RemotePeer()).
		Interface("conn_remote_addr", c.RemoteMultiaddr())}
}

func (e event) stream(s net.Stream) event {
	return event{ze: e.conn(s.Conn()).ze.
		Interface("stream_protocol_id", s.Protocol())}
}

func (e event) elapsed(startTime time.Time) event {
	if startTime.IsZero() {
		return e
	}
	return event{ze: e.ze.Dur("elapsed", time.Now().Sub(startTime))}
}

func (e event) msg(message string) {
	e.ze.Msg(message)
}

// ConnLogger is a LibP2P connection logger that logs to a zerolog logger.
// It logs all listener/connection/stream open/close activities at debug level.
// To use one, add it on a LibP2P host swarm as a notifier, ex:
//
//   connLogger := utils.NewConnLogger(
//   host.Network().Notify(connLogger)
type ConnLogger struct {
	mtx         sync.Mutex
	logger      zerolog.Logger
	listeners   map[string]time.Time
	connections map[net.Conn]time.Time
	streams     map[net.Stream]time.Time
}

func (cl *ConnLogger) debug() event {
	return event{ze: cl.logger.Debug()}
}

// Listen logs a listener starting listening on an address.
func (cl *ConnLogger) Listen(n net.Network, ma ma.Multiaddr) {
	cl.mtx.Lock()
	defer cl.mtx.Unlock()
	cl.debug().net(n).listener(ma).msg("listener starting")
	cl.listeners[string(ma.Bytes())] = time.Now()
}

// ListenClose logs a listener stopping listening on an address.
func (cl *ConnLogger) ListenClose(n net.Network, ma ma.Multiaddr) {
	cl.mtx.Lock()
	defer cl.mtx.Unlock()
	maBin := string(ma.Bytes())
	cl.debug().net(n).listener(ma).elapsed(cl.listeners[maBin]).msg("listener closing")
	delete(cl.listeners, maBin)
}

// Connected logs a connection getting opened.
func (cl *ConnLogger) Connected(n net.Network, c net.Conn) {
	cl.mtx.Lock()
	defer cl.mtx.Unlock()
	cl.debug().net(n).conn(c).msg("connected")
	cl.connections[c] = time.Now()
}

// Disconnected logs a connection getting closed.
func (cl *ConnLogger) Disconnected(n net.Network, c net.Conn) {
	cl.mtx.Lock()
	defer cl.mtx.Unlock()
	cl.debug().net(n).conn(c).elapsed(cl.connections[c]).msg("disconnected")
	delete(cl.connections, c)
}

// OpenedStream logs a new stream getting opened.
func (cl *ConnLogger) OpenedStream(n net.Network, s net.Stream) {
	cl.mtx.Lock()
	defer cl.mtx.Unlock()
	cl.debug().net(n).stream(s).msg("stream opened")
	cl.streams[s] = time.Now()
}

// ClosedStream logs a stream getting closed.
func (cl *ConnLogger) ClosedStream(n net.Network, s net.Stream) {
	cl.mtx.Lock()
	defer cl.mtx.Unlock()
	cl.debug().net(n).stream(s).elapsed(cl.streams[s]).msg("stream closed")
	delete(cl.streams, s)
}

// NewConnLogger returns a new connection logger that uses the given
// zerolog logger.  See ConnLogger for usage.
func NewConnLogger(logger zerolog.Logger) *ConnLogger {
	return &ConnLogger{
		logger:      logger,
		listeners:   map[string]time.Time{},
		connections: map[net.Conn]time.Time{},
		streams:     map[net.Stream]time.Time{},
	}
}

// RootConnLogger is a LibP2P connection logger that logs to stderr.
// See ConnLogger for usage.
var RootConnLogger = NewConnLogger(zerolog.New(os.Stderr))
