package hostv2

import (
	"bufio"
	"context"
	"fmt"

	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2p"
	libp2p "github.com/libp2p/go-libp2p"
	libp2phost "github.com/libp2p/go-libp2p-host"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	multiaddr "github.com/multiformats/go-multiaddr"
)

const (
	// BatchSizeInByte The batch size in byte (64MB) in which we return data
	BatchSizeInByte = 1 << 16
	// ProtocolID The ID of protocol used in stream handling.
	ProtocolID = "/harmony/0.0.1"
)

// HostV2 is the version 2 p2p host
type HostV2 struct {
	h    libp2phost.Host
	self p2p.Peer
}

// Peerstore returns the peer store
func (host *HostV2) Peerstore() peerstore.Peerstore {
	return host.h.Peerstore()
}

// New creates a host for p2p communication
func New(self p2p.Peer) *HostV2 {
	addr := fmt.Sprintf("/ip4/%s/tcp/%s", self.IP, self.Port)
	sourceAddr, err := multiaddr.NewMultiaddr(addr)
	catchError(err)
	priv := addrToPrivKey(addr)
	p2pHost, err := libp2p.New(context.Background(),
		libp2p.ListenAddrs(sourceAddr),
		libp2p.Identity(priv),
		libp2p.NoSecurity, // The security (signature generation and verification) is, for now, taken care by ourselves.
		// TODO(ricl): Other features to probe
		// libp2p.EnableRelay; libp2p.Routing;
	)
	catchError(err)
	log.Debug("Host is up!", "port", self.Port, "id", p2pHost.ID().Pretty(), "addrs", sourceAddr)
	h := &HostV2{
		h:    p2pHost,
		self: self,
	}
	return h
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
	addr := fmt.Sprintf("/ip4/%s/tcp/%s", p.IP, p.Port)
	targetAddr, err := multiaddr.NewMultiaddr(addr)

	priv := addrToPrivKey(addr)
	peerID, _ := peer.IDFromPrivateKey(priv)
	host.Peerstore().AddAddrs(peerID, []multiaddr.Multiaddr{targetAddr}, peerstore.PermanentAddrTTL)
	s, err := host.h.NewStream(context.Background(), peerID, ProtocolID)
	catchError(err)

	// Create a buffered stream so that read and writes are non blocking.
	w := bufio.NewWriter(bufio.NewWriter(s))

	// Create a thread to read and write data.
	go writeData(w, message)
	return nil
}

// Close closes the host
func (host *HostV2) Close() error {
	return host.h.Close()
}
