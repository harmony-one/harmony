package hostv2

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	libp2p "github.com/libp2p/go-libp2p"
	p2p_crypto "github.com/libp2p/go-libp2p-crypto"
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
	h      libp2phost.Host
	self   p2p.Peer
	priKey p2p_crypto.PrivKey
	pubKey p2p_crypto.PubKey
}

// AddPeer add p2p.Peer into Peerstore
func (host *HostV2) AddPeer(p *p2p.Peer) error {
	if p.PeerID != "" && len(p.Addrs) != 0 {
		host.Peerstore().AddAddrs(p.PeerID, p.Addrs, peerstore.PermanentAddrTTL)
		return nil
	}

	addr := fmt.Sprintf("/ip4/%s/tcp/%s", p.IP, p.Port)
	targetAddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		log.Error("AddPeer NewMultiaddr error", "error", err)
		return err
	}

	_, pubKey, err := utils.GenKeyP2P(p.IP, p.Port)
	if err != nil {
		log.Error("AddPeer GenKey error", "error", err)
		return err
	}

	peerID, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		log.Error("AddPeer IDFromPublicKey error", "error", err)
		return err
	}

	p.PeerID = peerID
	p.Addrs = append(p.Addrs, targetAddr)

	host.Peerstore().AddAddrs(peerID, p.Addrs, peerstore.PermanentAddrTTL)

	return nil
}

// Peerstore returns the peer store
func (host *HostV2) Peerstore() peerstore.Peerstore {
	return host.h.Peerstore()
}

// New creates a host for p2p communication
func New(self p2p.Peer) *HostV2 {
	sourceAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", self.Port))
	catchError(err)

	priKey, pubKey, err := utils.GenKeyP2P(self.IP, self.Port)
	catchError(err)

	p2pHost, err := libp2p.New(context.Background(),
		libp2p.ListenAddrs(sourceAddr),
		libp2p.Identity(priKey),
		// TODO(ricl): Other features to probe
		// libp2p.EnableRelay; libp2p.Routing;
	)

	catchError(err)
	log.Debug("HostV2 is up!", "port", self.Port, "id", p2pHost.ID().Pretty(), "addr", sourceAddr)

	h := &HostV2{
		h:      p2pHost,
		self:   self,
		priKey: priKey,
		pubKey: pubKey,
	}

	return h
}

// GetID returns ID.Pretty
func (host *HostV2) GetID() string {
	return host.h.ID().Pretty()
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
	s, err := host.h.NewStream(context.Background(), p.PeerID, ProtocolID)
	if err != nil {
		log.Error("Failed to send message", "from", host.self, "to", p, "error", err)
		return err
	}

	defer s.Close()
	s.Write(message)

	return nil
}

// Close closes the host
func (host *HostV2) Close() error {
	return host.h.Close()
}
