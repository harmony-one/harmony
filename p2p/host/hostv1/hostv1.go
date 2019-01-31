package hostv1

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/p2p"
	p2p_host "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
)

// HostV1 is the version 1 p2p host, using direct socket call.
type HostV1 struct {
	self     p2p.Peer
	listener net.Listener
	quit     chan struct{}
}

// AddPeer do nothing
func (host *HostV1) AddPeer(p *p2p.Peer) error {
	return nil
}

// New creates a HostV1
func New(self *p2p.Peer) *HostV1 {
	h := &HostV1{
		self: *self,
		quit: make(chan struct{}, 1),
	}
	return h
}

// GetSelfPeer gets self peer
func (host *HostV1) GetSelfPeer() p2p.Peer {
	return host.self
}

// GetID return ID
func (host *HostV1) GetID() peer.ID {
	return peer.ID(fmt.Sprintf("%s:%s", host.self.IP, host.self.Port))
}

// BindHandlerAndServe Version 0 p2p. Going to be deprecated.
func (host *HostV1) BindHandlerAndServe(handler p2p.StreamHandler) {
	port := host.self.Port
	addr := net.JoinHostPort("", port)
	var err error
	host.listener, err = net.Listen("tcp4", addr)
	if err != nil {
		log.Error("Socket listen port failed", "addr", addr, "err", err)
		return
	}
	if host.listener == nil {
		log.Error("Listen returned nil", "addr", addr)
		return
	}
	backoff := p2p.NewExpBackoff(250*time.Millisecond, 15*time.Second, 2.0)
	for { // Keep listening
		conn, err := host.listener.Accept()
		select {
		case <-host.quit:
			// If we've already received quit signal, simply ignore the error and return
			log.Info("Quit host", "addr", net.JoinHostPort(host.self.IP, host.self.Port))
			return
		default:
			{
				if err != nil {
					log.Error("Error listening on port.", "port", port,
						"err", err)
					backoff.Sleep()
					continue
				}
				// log.Debug("Received New connection", "local", conn.LocalAddr(), "remote", conn.RemoteAddr())
				go handler(conn)
			}
		}
	}
}

// SendMessage sends message to peer
func (host *HostV1) SendMessage(peer p2p.Peer, message []byte) error {
	logger := log.New("from", host.self, "to", peer, "PeerID", peer.PeerID)
	addr := net.JoinHostPort(peer.IP, peer.Port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		logger.Warn("Dial() failed", "address", addr, "error", err)
		return fmt.Errorf("Dial(%s) failed: %v", addr, err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Warn("Close() failed", "error", err)
		}
	}()

	if nw, err := conn.Write(message); err != nil {
		logger.Warn("Write() failed", "error", err)
		return fmt.Errorf("Write() failed: %v", err)
	} else if nw < len(message) {
		logger.Warn("Short Write()", "actual", nw, "expected", len(message))
		return io.ErrShortWrite
	}

	// No ack (reply) message from the receiver for now.
	return nil
}

// Close closes the host
func (host *HostV1) Close() error {
	host.quit <- struct{}{}
	return host.listener.Close()
}

// GetP2PHost returns nothing
func (host *HostV1) GetP2PHost() p2p_host.Host {
	return nil
}
