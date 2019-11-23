package hostv2

//go:generate mockgen -source=hostv2.go -package=hostv2 -destination=hostv2_mock_for_test.go

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"

	libp2p "github.com/libp2p/go-libp2p"
	libp2p_crypto "github.com/libp2p/go-libp2p-crypto"
	libp2p_host "github.com/libp2p/go-libp2p-host"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	libp2p_peerstore "github.com/libp2p/go-libp2p-peerstore"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
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

// topicHandle is a pubsub topic handle.
type topicHandle interface {
	Publish(ctx context.Context, data []byte) error
	Subscribe() (subscription, error)
}

type topicHandleImpl struct {
	t *libp2p_pubsub.Topic
}

func (th topicHandleImpl) Publish(ctx context.Context, data []byte) error {
	return th.t.Publish(ctx, data)
}

func (th topicHandleImpl) Subscribe() (subscription, error) {
	return th.t.Subscribe()
}

type topicJoiner interface {
	JoinTopic(topic string) (topicHandle, error)
}

type topicJoinerImpl struct {
	pubsub *libp2p_pubsub.PubSub
}

func (tj topicJoinerImpl) JoinTopic(topic string) (topicHandle, error) {
	th, err := tj.pubsub.Join(topic)
	if err != nil {
		return nil, err
	}
	return topicHandleImpl{th}, nil
}

// HostV2 is the version 2 p2p host
type HostV2 struct {
	h      libp2p_host.Host
	joiner topicJoiner
	joined map[string]topicHandle
	self   p2p.Peer
	priKey libp2p_crypto.PrivKey
	lock   sync.Mutex

	//incomingPeers []p2p.Peer // list of incoming Peers. TODO: fixed number incoming
	//outgoingPeers []p2p.Peer // list of outgoing Peers. TODO: fixed number of outgoing

	// logger
	logger *zerolog.Logger
}

func (host *HostV2) getTopic(topic string) (topicHandle, error) {
	host.lock.Lock()
	defer host.lock.Unlock()
	if t, ok := host.joined[topic]; ok {
		return t, nil
	} else if t, err := host.joiner.JoinTopic(topic); err != nil {
		return nil, errors.Wrapf(err, "cannot join pubsub topic %x", topic)
	} else {
		host.joined[topic] = t
		return t, nil
	}
}

// SendMessageToGroups sends a message to one or more multicast groups.
// It returns a nil error if and only if it has succeeded to schedule the given
// message for sending.
func (host *HostV2) SendMessageToGroups(groups []nodeconfig.GroupID, msg []byte) (err error) {
	for _, group := range groups {
		t, e := host.getTopic(string(group))
		if e != nil {
			err = e
			continue
		}
		e = t.Publish(context.Background(), msg)
		if e != nil {
			err = e
			continue
		}
	}
	return err
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
	if r.sub == nil {
		return nil, libp2p_peer.ID(""), fmt.Errorf("GroupReceiver has been closed")
	}
	m, err := r.sub.Next(ctx)
	if err == nil {
		msg = m.Data
		sender = libp2p_peer.ID(m.From)
	}
	return msg, sender, err
}

// GroupReceiver returns a receiver of messages sent to a multicast group.
// See the GroupReceiver interface for details.
func (host *HostV2) GroupReceiver(group nodeconfig.GroupID) (
	receiver p2p.GroupReceiver, err error,
) {
	t, err := host.getTopic(string(group))
	if err != nil {
		return nil, err
	}
	sub, err := t.Subscribe()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot subscribe to topic %x", group)
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
		host.logger.Error().Msg("AddPeer PeerID is EMPTY")
		return fmt.Errorf("AddPeer error: peerID is empty")
	}

	// reconstruct the multiaddress based on ip/port
	// PeerID has to be known for the ip/port
	addr := fmt.Sprintf("/ip4/%s/tcp/%s", p.IP, p.Port)
	targetAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		host.logger.Error().Err(err).Msg("AddPeer NewMultiaddr error")
		return err
	}

	p.Addrs = append(p.Addrs, targetAddr)

	host.Peerstore().AddAddrs(p.PeerID, p.Addrs, libp2p_peerstore.PermanentAddrTTL)
	host.logger.Info().Interface("peer", *p).Msg("AddPeer add to libp2p_peerstore")

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
	// TODO: Convert to zerolog or internal logger interface
	logger := utils.Logger()
	if err != nil {
		logger.Error().Str("IP", self.IP).Str("Port", self.Port).Msg("New MA Error")
		return nil
	}
	// TODO – use WithCancel for orderly host teardown (which we don't have yet)
	ctx := context.Background()
	p2pHost, err := libp2p.New(ctx,
		libp2p.ListenAddrs(listenAddr), libp2p.Identity(priKey),
	)
	catchError(err)
	pubsub, err := libp2p_pubsub.NewGossipSub(ctx, p2pHost)
	catchError(err)

	self.PeerID = p2pHost.ID()

	subLogger := logger.With().Str("hostID", p2pHost.ID().Pretty()).Logger()
	// has to save the private key for host
	h := &HostV2{
		h:      p2pHost,
		joiner: topicJoinerImpl{pubsub},
		joined: map[string]topicHandle{},
		self:   *self,
		priKey: priKey,
		logger: &subLogger,
	}

	h.logger.Debug().
		Str("port", self.Port).
		Str("id", p2pHost.ID().Pretty()).
		Str("addr", listenAddr.String()).
		Str("PubKey", self.ConsensusPubKey.SerializeToHexStr()).
		Msg("HostV2 is up!")

	return h
}

// GetID returns ID.Pretty
func (host *HostV2) GetID() libp2p_peer.ID {
	return host.h.ID()
}

// GetSelfPeer gets self peer
func (host *HostV2) GetSelfPeer() p2p.Peer {
	return host.self
}

// Close closes the host
func (host *HostV2) Close() error {
	return host.h.Close()
}

// GetP2PHost returns the p2p.Host
func (host *HostV2) GetP2PHost() libp2p_host.Host {
	return host.h
}

// GetPeerCount ...
func (host *HostV2) GetPeerCount() int {
	return host.h.Peerstore().Peers().Len()
}

// ConnectHostPeer connects to peer host
func (host *HostV2) ConnectHostPeer(peer p2p.Peer) {
	ctx := context.Background()
	addr := fmt.Sprintf("/ip4/%s/tcp/%s/ipfs/%s", peer.IP, peer.Port, peer.PeerID.Pretty())
	peerAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		host.logger.Error().Err(err).Interface("peer", peer).Msg("ConnectHostPeer")
		return
	}
	peerInfo, err := libp2p_peerstore.InfoFromP2pAddr(peerAddr)
	if err != nil {
		host.logger.Error().Err(err).Interface("peer", peer).Msg("ConnectHostPeer")
		return
	}
	if err := host.h.Connect(ctx, *peerInfo); err != nil {
		host.logger.Warn().Err(err).Interface("peer", peer).Msg("can't connect to peer")
	} else {
		host.logger.Info().Interface("node", *peerInfo).Msg("connected to peer host")
	}
}
