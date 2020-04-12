package p2p

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/harmony-one/bls/ffi/go/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	libp2p "github.com/libp2p/go-libp2p"
	libp2p_crypto "github.com/libp2p/go-libp2p-core/crypto"
	libp2p_host "github.com/libp2p/go-libp2p-core/host"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	libp2p_peerstore "github.com/libp2p/go-libp2p-core/peerstore"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Host is the client + server in p2p network.
type Host interface {
	GetSelfPeer() Peer
	Close() error
	AddPeer(*Peer) error
	GetID() libp2p_peer.ID
	GetP2PHost() libp2p_host.Host
	GetPeerCount() int
	ConnectHostPeer(Peer) error
	// SendMessageToGroups sends a message to one or more multicast groups.
	SendMessageToGroups(groups []nodeconfig.GroupID, msg []byte) error
	// GroupReceiver returns a receiver of messages sent to a multicast group.
	// Each call creates a new receiver.
	// If multiple receivers are created for the same group,
	// a message sent to the group will be delivered to all of the receivers.
	GroupReceiver(nodeconfig.GroupID) (receiver GroupReceiver, err error)
}

// Peer is the object for a p2p peer (node)
type Peer struct {
	IP              string         // IP address of the peer
	Port            string         // Port number of the peer
	ConsensusPubKey *bls.PublicKey // Public key of the peer, used for consensus signing
	Addrs           []ma.Multiaddr // MultiAddress of the peer
	PeerID          libp2p_peer.ID // PeerID, the pubkey for communication
}

func (p Peer) String() string {
	BLSPubKey := "nil"
	if p.ConsensusPubKey != nil {
		BLSPubKey = p.ConsensusPubKey.SerializeToHexStr()
	}
	return fmt.Sprintf(
		"BLSPubKey:%s-%s/%s[%d]", BLSPubKey,
		net.JoinHostPort(p.IP, p.Port), p.PeerID, len(p.Addrs),
	)
}

// GroupReceiver is a multicast group message receiver interface.
type GroupReceiver interface {
	// Close closes this receiver.
	io.Closer

	// Receive a message.
	Receive(ctx context.Context) (msg []byte, sender libp2p_peer.ID, err error)
}

// NewHost ..
func NewHost(self *Peer, key libp2p_crypto.PrivKey) (Host, error) {
	h, err := New(self, key)
	if err != nil {
		return nil, err
	}

	utils.Logger().Info().
		Str("self", net.JoinHostPort(self.IP, self.Port)).
		Interface("PeerID", self.PeerID).
		Str("PubKey", self.ConsensusPubKey.SerializeToHexStr()).
		Msg("NewHost")

	return h, nil
}

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
	self   Peer
	priKey libp2p_crypto.PrivKey
	lock   sync.Mutex
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
	receiver GroupReceiver, err error,
) {
	top := string(group)
	t, err := host.getTopic(top)
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
func (host *HostV2) AddPeer(p *Peer) error {
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

// Peerstore returns the peer store
func (host *HostV2) Peerstore() libp2p_peerstore.Peerstore {
	return host.h.Peerstore()
}

// New creates a host for p2p communication
func New(self *Peer, priKey libp2p_crypto.PrivKey) (*HostV2, error) {
	listenAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", self.Port))
	if err != nil {
		return nil, errors.Wrapf(err,
			"cannot create listen multiaddr from port %#v", self.Port)
	}
	ctx := context.Background()
	p2pHost, err := libp2p.New(ctx,
		libp2p.ListenAddrs(listenAddr), libp2p.Identity(priKey),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot initialize libp2p host")
	}
	traceFile := os.Getenv("P2P_TRACEFILE")

	const MaxSize = 2_145_728
	options := []libp2p_pubsub.Option{
		libp2p_pubsub.WithPeerOutboundQueueSize(64),
		libp2p_pubsub.WithMaxMessageSize(MaxSize),
	}
	if len(traceFile) > 0 {
		tracer, _ := libp2p_pubsub.NewJSONTracer(traceFile)
		options = append(options, libp2p_pubsub.WithEventTracer(tracer))
	}
	pubsub, err := libp2p_pubsub.NewGossipSub(ctx, p2pHost, options...)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot initialize libp2p pubsub")
	}

	self.PeerID = p2pHost.ID()
	subLogger := utils.Logger().With().Str("hostID", p2pHost.ID().Pretty()).Logger()
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

	return h, nil
}

// GetID returns ID.Pretty
func (host *HostV2) GetID() libp2p_peer.ID {
	return host.h.ID()
}

// GetSelfPeer gets self peer
func (host *HostV2) GetSelfPeer() Peer {
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
func (host *HostV2) ConnectHostPeer(peer Peer) error {
	ctx := context.Background()
	addr := fmt.Sprintf("/ip4/%s/tcp/%s/ipfs/%s", peer.IP, peer.Port, peer.PeerID.Pretty())
	peerAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		host.logger.Error().Err(err).Interface("peer", peer).Msg("ConnectHostPeer")
		return err
	}
	peerInfo, err := libp2p_peer.AddrInfoFromP2pAddr(peerAddr)
	if err != nil {
		host.logger.Error().Err(err).Interface("peer", peer).Msg("ConnectHostPeer")
		return err
	}
	if err := host.h.Connect(ctx, *peerInfo); err != nil {
		host.logger.Warn().Err(err).Interface("peer", peer).Msg("can't connect to peer")
		return err
	}
	host.logger.Info().Interface("node", *peerInfo).Msg("connected to peer host")
	return nil
}

// ConstructMessage constructs the p2p message as [messageType, contentSize, content]
func ConstructMessage(content []byte) []byte {
	message := make([]byte, 5+len(content))
	message[0] = 17 // messageType 0x11
	binary.BigEndian.PutUint32(message[1:5], uint32(len(content)))
	copy(message[5:], content)
	return message
}

// AddrList is a list of multiaddress
type AddrList []ma.Multiaddr

// String is a function to print a string representation of the AddrList
func (al *AddrList) String() string {
	strs := make([]string, len(*al))
	for i, addr := range *al {
		strs[i] = addr.String()
	}
	return strings.Join(strs, ",")
}

// Set is a function to set the value of AddrList based on a string
func (al *AddrList) Set(value string) error {
	if len(*al) > 0 {
		return fmt.Errorf("AddrList is already set")
	}
	for _, a := range strings.Split(value, ",") {
		addr, err := ma.NewMultiaddr(a)
		if err != nil {
			return err
		}
		*al = append(*al, addr)
	}
	return nil
}

// StringsToAddrs convert a list of strings to a list of multiaddresses
func StringsToAddrs(addrStrings []string) (maddrs []ma.Multiaddr, err error) {
	for _, addrString := range addrStrings {
		addr, err := ma.NewMultiaddr(addrString)
		if err != nil {
			return maddrs, err
		}
		maddrs = append(maddrs, addr)
	}
	return
}

// StringsToPeers converts a string to a list of Peers
// addr is a string of format "ip:port,ip:port"
func StringsToPeers(input string) []Peer {
	addrs := strings.Split(input, ",")
	peers := []Peer{}
	for _, addr := range addrs {
		data := strings.Split(addr, ":")
		if len(data) >= 2 {
			peer := Peer{}
			peer.IP = data[0]
			peer.Port = data[1]
			peers = append(peers, peer)
		}
	}
	return peers
}

// DefaultBootNodeAddrStrings is a list of Harmony bootnodes address. Used to find other peers in the network.
var DefaultBootNodeAddrStrings = []string{
	"/ip4/127.0.0.1/tcp/19876/p2p/Qmc1V6W7BwX8Ugb42Ti8RnXF1rY5PF7nnZ6bKBryCgi6cv",
}

// BootNodes is a list of boot nodes. It is populated either from default or from user CLI input.
var BootNodes AddrList
