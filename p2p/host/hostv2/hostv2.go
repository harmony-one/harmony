package hostv2

//go:generate mockgen -source hostv2.go -destination=mock/hostv2_mock.go

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	libp2p "github.com/libp2p/go-libp2p"
	libp2p_crypto "github.com/libp2p/go-libp2p-crypto"
	libp2p_discovery "github.com/libp2p/go-libp2p-discovery"
	libp2p_host "github.com/libp2p/go-libp2p-host"
	libp2p_kad_dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2p_net "github.com/libp2p/go-libp2p-net"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	libp2p_peerstore "github.com/libp2p/go-libp2p-peerstore"
	libp2p_protocol "github.com/libp2p/go-libp2p-protocol"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

const (
	// BatchSizeInByte The batch size in byte (64MB) in which we return data
	BatchSizeInByte = 1 << 16
	// ProtocolID The ID of protocol used in stream handling.
	ProtocolID = "/harmony/0.0.1"

	// Constants for discovery service.
	//numIncoming = 128
	//numOutgoing = 16
)

// pubsub captures the pubsub interface we expect from libp2p.
type pubsub interface {
	Publish(topic string, data []byte) error
	Subscribe(topic string, opts ...libp2p_pubsub.SubOpt) (*libp2p_pubsub.Subscription, error)
}

// discovery captures the discovery interface we expect from libp2p.
type discovery interface {
	Advertise(
		ctx context.Context, ns string, opts ...libp2p_discovery.Option,
	) (time.Duration, error)
}

// discoveryPackage is the freestanding function interface we expect from the
// libp2p_discovery package.
type discoveryPackage interface {
	Advertise(ctx context.Context, a libp2p_discovery.Advertiser, ns string)
}

// realDiscoveryPackage is the real implementation of discoveryPackage.
type realDiscoveryPackage struct{}

func (realDiscoveryPackage) Advertise(
	ctx context.Context, a libp2p_discovery.Advertiser, ns string,
) {
	libp2p_discovery.Advertise(ctx, a, ns)
}

var realDiscPkg realDiscoveryPackage

// HostV2 is the version 2 p2p host
type HostV2 struct {
	h       libp2p_host.Host
	discPkg discoveryPackage
	disc    discovery
	pubsub  pubsub
	self    p2p.Peer
	priKey  libp2p_crypto.PrivKey
	lock    sync.Mutex

	// background goroutine support
	finishing  chan struct{}  // goroutines exit if this is closed
	goroutines sync.WaitGroup // each goroutine holds one ref

	//incomingPeers []p2p.Peer // list of incoming Peers. TODO: fixed number incoming
	//outgoingPeers []p2p.Peer // list of outgoing Peers. TODO: fixed number of outgoing

	// logger
	logger log.Logger
}

func (host *HostV2) runBackground(bodies ...func()) {
	host.goroutines.Add(len(bodies))
	for _, body := range bodies {
		go func(f func()) {
			defer host.goroutines.Done()
			f()
		}(body)
	}
}

// SendMessageToGroups sends a message to one or more multicast groups.
func (host *HostV2) SendMessageToGroups(groups []p2p.GroupID, msg []byte) error {
	var error error
	for _, group := range groups {
		err := host.pubsub.Publish(string(group), msg)
		if err != nil {
			error = err
		}
	}
	return error
}

// subscription captures the subscription interface we expect from libp2p.
type subscription interface {
	Next(ctx context.Context) (*libp2p_pubsub.Message, error)
	Cancel()
}

// GroupReceiverImpl is a multicast group receiver implementation.
type GroupReceiverImpl struct {
	sub subscription
}

// Close closes this receiver.
func (r *GroupReceiverImpl) Close() error {
	r.sub.Cancel()
	r.sub = nil
	return nil
}

// Receive receives a message.
func (r *GroupReceiverImpl) Receive(ctx context.Context) (
	msg []byte, sender libp2p_peer.ID, err error,
) {
	m, err := r.sub.Next(ctx)
	if err == nil {
		msg = m.Data
		sender = libp2p_peer.ID(m.From)
	}
	return msg, sender, err
}

// GroupReceiver returns a receiver of messages sent to a multicast group.
// See the GroupReceiver interface for details.
func (host *HostV2) GroupReceiver(group p2p.GroupID) (
	receiver p2p.GroupReceiver, err error,
) {
	sub, err := host.pubsub.Subscribe(string(group))
	if err != nil {
		return nil, err
	}
	return &GroupReceiverImpl{sub: sub}, nil
}

// AddPeer add p2p.Peer into Peerstore
func (host *HostV2) AddPeer(p *p2p.Peer) error {
	if p.PeerID != "" && len(p.Addrs) != 0 {
		host.Peerstore().AddAddrs(p.PeerID, p.Addrs, libp2p_peerstore.PermanentAddrTTL)
		return nil
	}

	if p.PeerID == "" {
		host.logger.Error("AddPeer PeerID is EMPTY")
		return fmt.Errorf("AddPeer error: peerID is empty")
	}

	// reconstruct the multiaddress based on ip/port
	// PeerID has to be known for the ip/port
	addr := fmt.Sprintf("/ip4/%s/tcp/%s", p.IP, p.Port)
	targetAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		host.logger.Error("AddPeer NewMultiaddr error", "error", err)
		return err
	}

	p.Addrs = append(p.Addrs, targetAddr)

	host.Peerstore().AddAddrs(p.PeerID, p.Addrs, libp2p_peerstore.PermanentAddrTTL)
	host.logger.Info("AddPeer add to libp2p_peerstore", "peer", *p)

	return nil
}

//// AddIncomingPeer add peer to incoming peer list
//func (host *HostV2) AddIncomingPeer(peer p2p.Peer) {
//	host.incomingPeers = append(host.incomingPeers, peer)
//}
//
//// AddOutgoingPeer add peer to outgoing peer list
//func (host *HostV2) AddOutgoingPeer(peer p2p.Peer) {
//	host.outgoingPeers = append(host.outgoingPeers, peer)
//}

// Peerstore returns the peer store
func (host *HostV2) Peerstore() libp2p_peerstore.Peerstore {
	return host.h.Peerstore()
}

// New creates a host for p2p communication
func New(self *p2p.Peer, priKey libp2p_crypto.PrivKey) *HostV2 {
	listenAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", self.Port))
	logger := utils.GetLogInstance()
	if err != nil {
		logger.Error("New MA Error", "IP", self.IP, "Port", self.Port)
		return nil
	}
	// TODO – use WithCancel for orderly host teardown (which we don't have yet)
	ctx := context.Background()
	p2pHost, err := libp2p.New(ctx,
		libp2p.ListenAddrs(listenAddr), libp2p.Identity(priKey),
	)
	catchError(err)
	pubsub, err := libp2p_pubsub.NewGossipSub(ctx, p2pHost)
	// pubsub, err := libp2p_pubsub.NewFloodSub(ctx, p2pHost)
	catchError(err)
	dht, err := libp2p_kad_dht.New(ctx, p2pHost)
	catchError(err)
	disc := libp2p_discovery.NewRoutingDiscovery(dht)

	self.PeerID = p2pHost.ID()

	// has to save the private key for host
	h := &HostV2{
		h:         p2pHost,
		discPkg:   realDiscPkg,
		disc:      disc,
		pubsub:    pubsub,
		self:      *self,
		priKey:    priKey,
		logger:    logger.New("hostID", p2pHost.ID().Pretty()),
		finishing: make(chan struct{}),
	}
	h.runBackground(h.advertiseSelf)

	h.logger.Debug("HostV2 is up!",
		"port", self.Port, "id", p2pHost.ID().Pretty(), "addr", listenAddr)

	return h
}

func (host *HostV2) advertiseSelf() {
	host.logger.Debug("advertiseSelf: advertising myself")
	ctx, cancel := context.WithCancel(context.Background())
	host.discPkg.Advertise(ctx, host.disc, string(host.GetID()))
	host.logger.Debug("advertiseSelf: waiting for host to close")
	for {
		_, ok := <-host.finishing
		if !ok {
			break
		}
	}
	cancel()
	host.logger.Debug("advertiseSelf: finished")
}

// GetID returns ID.Pretty
func (host *HostV2) GetID() libp2p_peer.ID {
	return host.h.ID()
}

// GetSelfPeer gets self peer
func (host *HostV2) GetSelfPeer() p2p.Peer {
	return host.self
}

// BindHandlerAndServe bind a streamHandler to the harmony protocol.
func (host *HostV2) BindHandlerAndServe(handler p2p.StreamHandler) {
	host.h.SetStreamHandler(libp2p_protocol.ID(ProtocolID), func(s libp2p_net.Stream) {
		handler(s)
	})
	// Hang forever
	<-make(chan struct{})
}

// SendMessage a p2p message sending function with signature compatible to p2pv1.
func (host *HostV2) SendMessage(p p2p.Peer, message []byte) error {
	logger := host.logger.New("from", host.self, "to", p, "PeerID", p.PeerID)
	err := host.Peerstore().AddProtocols(p.PeerID, ProtocolID)
	if err != nil {
		logger.Error("AddProtocols() failed", "error", err)
		return p2p.ErrAddProtocols
	}
	s, err := host.h.NewStream(context.Background(), p.PeerID, libp2p_protocol.ID(ProtocolID))
	if err != nil {
		logger.Error("NewStream() failed", "peerID", p.PeerID,
			"protocolID", ProtocolID, "error", err)
		return p2p.ErrNewStream
	}
	if nw, err := s.Write(message); err != nil {
		logger.Error("Write() failed", "peerID", p.PeerID,
			"protocolID", ProtocolID, "error", err)
		return p2p.ErrMsgWrite
	} else if nw < len(message) {
		logger.Error("Short Write()", "expected", len(message), "actual", nw)
		return io.ErrShortWrite
	}

	return nil
}

// Close closes the host
func (host *HostV2) Close() error {
	close(host.finishing)
	host.goroutines.Wait()
	err := host.h.Close()
	if err != nil {
		host.logger.Warn("hostv2.HostV2.Close: libp2p host close failed",
			"error", err)
	}
	return err
}

// GetP2PHost returns the p2p.Host
func (host *HostV2) GetP2PHost() libp2p_host.Host {
	return host.h
}

// ConnectHostPeer connects to peer host
func (host *HostV2) ConnectHostPeer(peer p2p.Peer) {
	ctx := context.Background()
	addr := fmt.Sprintf("/ip4/%s/tcp/%s/ipfs/%s", peer.IP, peer.Port, peer.PeerID.Pretty())
	peerAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		host.logger.Error("ConnectHostPeer", "new ma error", err, "peer", peer)
		return
	}
	peerInfo, err := libp2p_peerstore.InfoFromP2pAddr(peerAddr)
	if err != nil {
		host.logger.Error("ConnectHostPeer", "new peerinfo error", err, "peer",
			peer)
		return
	}
	if err := host.h.Connect(ctx, *peerInfo); err != nil {
		host.logger.Warn("can't connect to peer", "error", err, "peer", peer)
	} else {
		host.logger.Info("connected to peer host", "node", *peerInfo)
	}
}
