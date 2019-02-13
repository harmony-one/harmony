package hostv2

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"

	libp2p "github.com/libp2p/go-libp2p"
	p2p_crypto "github.com/libp2p/go-libp2p-crypto"
	p2p_host "github.com/libp2p/go-libp2p-host"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	p2p_config "github.com/libp2p/go-libp2p/config"
	ma "github.com/multiformats/go-multiaddr"
)

const (
	// BatchSizeInByte The batch size in byte (64MB) in which we return data
	BatchSizeInByte = 1 << 16
	// ProtocolID The ID of protocol used in stream handling.
	ProtocolID = "/harmony/0.0.1"

	// Constants for discovery service.
	numIncoming = 128
	numOutgoing = 16
)

// PubSub captures the pubsub interface we expect from libp2p.
type PubSub interface {
	Publish(topic string, data []byte) error
	Subscribe(topic string, opts ...pubsub.SubOpt) (*pubsub.Subscription, error)
}

// HostV2 is the version 2 p2p host
type HostV2 struct {
	h      p2p_host.Host
	pubsub PubSub
	self   p2p.Peer
	priKey p2p_crypto.PrivKey
	lock   sync.Mutex

	incomingPeers []p2p.Peer // list of incoming Peers. TODO: fixed number incoming
	outgoingPeers []p2p.Peer // list of outgoing Peers. TODO: fixed number of outgoing
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

// Subscription captures the subscription interface
type Subscription interface {
	Next(ctx context.Context) (*pubsub.Message, error)
	Cancel()
}

// GroupReceiverImpl is a multicast group receiver implementation.
type GroupReceiverImpl struct {
	sub Subscription
}

// Close closes this receiver.
func (r *GroupReceiverImpl) Close() error {
	r.sub.Cancel()
	r.sub = nil
	return nil
}

// Receive receives a message.
func (r *GroupReceiverImpl) Receive(ctx context.Context) (
	msg []byte, sender peer.ID, err error,
) {
	m, err := r.sub.Next(ctx)
	if err == nil {
		msg = m.Data
		sender = peer.ID(m.From)
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

// AddIncomingPeer add peer to incoming peer list
func (host *HostV2) AddIncomingPeer(peer p2p.Peer) {
	host.incomingPeers = append(host.incomingPeers, peer)
}

// AddOutgoingPeer add peer to outgoing peer list
func (host *HostV2) AddOutgoingPeer(peer p2p.Peer) {
	host.outgoingPeers = append(host.outgoingPeers, peer)
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
	// TODO – use WithCancel for orderly host teardown (which we don't have yet)
	ctx := context.Background()
	p2pHost, err := libp2p.New(ctx,
		append(opts, libp2p.ListenAddrs(listenAddr), libp2p.Identity(priKey))...,
	)
	catchError(err)
	pubsub, err := pubsub.NewGossipSub(ctx, p2pHost)
	// pubsub, err := pubsub.NewFloodSub(ctx, p2pHost)
	catchError(err)

	self.PeerID = p2pHost.ID()

	log.Debug("HostV2 is up!", "port", self.Port, "id", p2pHost.ID().Pretty(), "addr", listenAddr)

	// has to save the private key for host
	h := &HostV2{
		h:      p2pHost,
		pubsub: pubsub,
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
	host.h.SetStreamHandler(protocol.ID(ProtocolID), func(s net.Stream) {
		handler(s)
	})
	// Hang forever
	<-make(chan struct{})
}

// SendMessage a p2p message sending function with signature compatible to p2pv1.
func (host *HostV2) SendMessage(p p2p.Peer, message []byte) error {
	logger := log.New("from", host.self, "to", p, "PeerID", p.PeerID)
	host.Peerstore().AddProtocols(p.PeerID, ProtocolID)
	s, err := host.h.NewStream(context.Background(), p.PeerID, protocol.ID(ProtocolID))
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
	return host.h.Close()
}

// GetP2PHost returns the p2p.Host
func (host *HostV2) GetP2PHost() p2p_host.Host {
	return host.h
}

// ConnectHostPeer connects to peer host
func (host *HostV2) ConnectHostPeer(peer p2p.Peer) {
	ctx := context.Background()
	addr := fmt.Sprintf("/ip4/%s/tcp/%s/ipfs/%s", peer.IP, peer.Port, peer.PeerID.Pretty())
	peerAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		utils.GetLogInstance().Error("ConnectHostPeer", "new ma error", err, "peer", peer)
		return
	}
	peerInfo, err := peerstore.InfoFromP2pAddr(peerAddr)
	if err != nil {
		utils.GetLogInstance().Error("ConnectHostPeer", "new peerinfo error", err, "peer", peer)
		return
	}
	host.lock.Lock()
	defer host.lock.Unlock()
	if err := host.h.Connect(ctx, *peerInfo); err != nil {
		utils.GetLogInstance().Warn("can't connect to peer", "error", err, "peer", peer)
	} else {
		utils.GetLogInstance().Info("connected to peer host", "node", *peerInfo)
	}
}
