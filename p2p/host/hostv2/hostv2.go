package hostv2

import (
	"context"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/p2p"
	libp2p "github.com/libp2p/go-libp2p"
	p2p_crypto "github.com/libp2p/go-libp2p-crypto"
	p2p_host "github.com/libp2p/go-libp2p-host"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	p2p_config "github.com/libp2p/go-libp2p/config"
	ma "github.com/multiformats/go-multiaddr"
)

const (
	// BatchSizeInByte The batch size in byte (64MB) in which we return data
	BatchSizeInByte = 1 << 16
	// ProtocolID The ID of protocol used in stream handling.
	ProtocolID = "/harmony/0.0.1"
)

// HostV2 is the version 2 p2p host
type HostV2 struct {
	h      p2p_host.Host
	self   p2p.Peer
	priKey p2p_crypto.PrivKey
}

// AddPeer add p2p.Peer into Peerstore
func (host *HostV2) AddPeer(p *p2p.Peer) error {
	if p.PeerID != "" && len(p.Addrs) != 0 {
		host.Peerstore().AddAddrs(p.PeerID, p.Addrs, peerstore.PermanentAddrTTL)
		return nil
	}

	if p.PeerID == "" {
		log.Error("AddPeer PeerID is EMPTY")
		return fmt.Errorf("AddPeer error: peerID is empty")
	}

	// reconstruct the multiaddress based on ip/port
	// PeerID has to be known for the ip/port
	addr := fmt.Sprintf("/ip4/%s/tcp/%s", p.IP, p.Port)
	targetAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		log.Error("AddPeer NewMultiaddr error", "error", err)
		return err
	}

	p.Addrs = append(p.Addrs, targetAddr)

	host.Peerstore().AddAddrs(p.PeerID, p.Addrs, peerstore.PermanentAddrTTL)
	log.Info("AddPeer add to peerstore", "peer", *p)

	return nil
}

// Peerstore returns the peer store
func (host *HostV2) Peerstore() peerstore.Peerstore {
	return host.h.Peerstore()
}

// New creates a host for p2p communication
func New(self *p2p.Peer, priKey p2p_crypto.PrivKey, opts ...p2p_config.Option) *HostV2 {
	listenAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", self.Port))
	if err != nil {
		log.Error("New MA Error", "IP", self.IP, "Port", self.Port)
		return nil
	}
	p2pHost, err := libp2p.New(context.Background(),
		append(opts, libp2p.ListenAddrs(listenAddr), libp2p.Identity(priKey))...,
	)
	catchError(err)

	self.PeerID = p2pHost.ID()

	log.Debug("HostV2 is up!", "port", self.Port, "id", p2pHost.ID().Pretty(), "addr", listenAddr)

	// has to save the private key for host
	h := &HostV2{
		h:      p2pHost,
		self:   *self,
		priKey: priKey,
	}

	return h
}

// GetID returns ID.Pretty
func (host *HostV2) GetID() peer.ID {
	return host.h.ID()
}

// GetSelfPeer gets self peer
func (host *HostV2) GetSelfPeer() p2p.Peer {
	return host.self
}

// BindHandlerAndServe bind a streamHandler to the harmony protocol.
func (host *HostV2) BindHandlerAndServe(handler p2p.StreamHandler) {
	host.h.SetStreamHandler(ProtocolID, func(s net.Stream) {
		handler(s)
	})
	// Hang forever
	<-make(chan struct{})
}

// SendMessage a p2p message sending function with signature compatible to p2pv1.
func (host *HostV2) SendMessage(p p2p.Peer, message []byte) error {
	logger := log.New("from", host.self, "to", p, "PeerID", p.PeerID)
	s, err := host.h.NewStream(context.Background(), p.PeerID, ProtocolID)
	if err != nil {
		logger.Error("NewStream() failed", "peerID", p.PeerID,
			"protocolID", ProtocolID, "error", err)
		return fmt.Errorf("NewStream(%v, %v) failed: %v", p.PeerID,
			ProtocolID, err)
	}
	if nw, err := s.Write(message); err != nil {
		logger.Error("Write() failed", "error", err)
		return fmt.Errorf("Write() failed: %v", err)
	} else if nw < len(message) {
		logger.Error("Short Write()", "expected", len(message), "actual", nw)
		return io.ErrShortWrite
	}

	return nil
}

// Close closes the host
func (host *HostV2) Close() error {
	return host.h.Close()
}
