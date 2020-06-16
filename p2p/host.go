package p2p

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/harmony-one/bls/ffi/go/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	libp2p "github.com/libp2p/go-libp2p"
	libp2p_crypto "github.com/libp2p/go-libp2p-core/crypto"
	libp2p_host "github.com/libp2p/go-libp2p-core/host"
	libp2p_network "github.com/libp2p/go-libp2p-core/network"
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
	AddPeer(*Peer) error
	GetID() libp2p_peer.ID
	GetP2PHost() libp2p_host.Host
	GetPeerCount() int
	ConnectHostPeer(Peer) error
	// SendMessageToGroups sends a message to one or more multicast groups.
	SendMessageToGroups(groups []nodeconfig.GroupID, msg []byte) error
	PubSub() *libp2p_pubsub.PubSub
	C() (int, int, int)
	GetOrJoin(topic string) (*libp2p_pubsub.Topic, error)
}

// Peer is the object for a p2p peer (node)
type Peer struct {
	IP              string         // IP address of the peer
	Port            string         // Port number of the peer
	ConsensusPubKey *bls.PublicKey // Public key of the peer, used for consensus signing
	Addrs           []ma.Multiaddr // MultiAddress of the peer
	PeerID          libp2p_peer.ID // PeerID, the pubkey for communication
}

const (
	// SetAsideForConsensus set the number of active validation goroutines for the consensus topic
	SetAsideForConsensus = 1 << 13
	// SetAsideOtherwise set the number of active validation goroutines for other topic
	SetAsideOtherwise = 1 << 12
	// MaxMessageHandlers ..
	MaxMessageHandlers = SetAsideForConsensus + SetAsideOtherwise
	// MaxMessageSize is 2Mb
	MaxMessageSize = 1 << 21
)

// NewHost ..
func NewHost(self *Peer, key libp2p_crypto.PrivKey) (Host, error) {
	listenAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", self.Port))
	if err != nil {
		return nil, errors.Wrapf(err,
			"cannot create listen multiaddr from port %#v", self.Port)
	}

	ctx := context.Background()
	p2pHost, err := libp2p.New(ctx,
		libp2p.ListenAddrs(listenAddr),
		libp2p.Identity(key),
		libp2p.EnableNATService(),
		libp2p.ForceReachabilityPublic(),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot initialize libp2p host")
	}
	options := []libp2p_pubsub.Option{
		// WithValidateQueueSize sets the buffer of validate queue. Defaults to 32. When queue is full, validation is throttled and new messages are dropped.
		libp2p_pubsub.WithValidateQueueSize(64),
		// WithPeerOutboundQueueSize is an option to set the buffer size for outbound messages to a peer. We start dropping messages to a peer if the outbound queue if full.
		libp2p_pubsub.WithPeerOutboundQueueSize(64),
		// WithValidateWorkers sets the number of synchronous validation worker goroutines. Defaults to NumCPU.  We are using async validation, so NumCPU * 2.
		libp2p_pubsub.WithValidateWorkers(runtime.NumCPU() * 2),
		// WithValidateThrottle sets the upper bound on the number of active validation goroutines across all topics. The default is 8192.
		libp2p_pubsub.WithValidateThrottle(MaxMessageHandlers),
		// WithMaxMessageSize sets the global maximum message size for pubsub wire messages. The default value is 1MiB (DefaultMaxMessageSize).
		libp2p_pubsub.WithMaxMessageSize(MaxMessageSize),
	}

	traceFile := os.Getenv("P2P_TRACEFILE")
	if len(traceFile) > 0 {
		var tracer libp2p_pubsub.EventTracer
		var tracerErr error
		if strings.HasPrefix(traceFile, "file:") {
			tracer, tracerErr = libp2p_pubsub.NewJSONTracer(strings.TrimPrefix(traceFile, "file:"))
		} else {
			pi, err := libp2p_peer.AddrInfoFromP2pAddr(ma.StringCast(traceFile))
			if err == nil {
				tracer, tracerErr = libp2p_pubsub.NewRemoteTracer(ctx, p2pHost, *pi)
			}
		}
		if tracerErr == nil && tracer != nil {
			options = append(options, libp2p_pubsub.WithEventTracer(tracer))
		} else {
			utils.Logger().Warn().
				Str("Tracer", traceFile).
				Msg("can't add event tracer from P2P_TRACEFILE")
		}
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
		pubsub: pubsub,
		joined: map[string]*libp2p_pubsub.Topic{},
		self:   *self,
		priKey: key,
		logger: &subLogger,
	}

	if err != nil {
		return nil, err
	}

	utils.Logger().Info().
		Str("self", net.JoinHostPort(self.IP, self.Port)).
		Interface("PeerID", self.PeerID).
		Str("PubKey", self.ConsensusPubKey.SerializeToHexStr()).
		Msg("libp2p host ready")
	return h, nil
}

// HostV2 is the version 2 p2p host
type HostV2 struct {
	h      libp2p_host.Host
	pubsub *libp2p_pubsub.PubSub
	joined map[string]*libp2p_pubsub.Topic
	self   Peer
	priKey libp2p_crypto.PrivKey
	lock   sync.Mutex
	logger *zerolog.Logger
}

// PubSub ..
func (host *HostV2) PubSub() *libp2p_pubsub.PubSub {
	return host.pubsub
}

// C .. -> (total known peers, connected, not connected)
func (host *HostV2) C() (int, int, int) {
	connected, not := 0, 0
	peers := host.h.Peerstore().Peers()
	for _, peer := range peers {
		result := host.h.Network().Connectedness(peer)
		if result == libp2p_network.Connected {
			connected++
		} else if result == libp2p_network.NotConnected {
			not++
		}
	}
	return len(peers), connected, not
}

// GetOrJoin ..
func (host *HostV2) GetOrJoin(topic string) (*libp2p_pubsub.Topic, error) {
	host.lock.Lock()
	defer host.lock.Unlock()
	if t, ok := host.joined[topic]; ok {
		return t, nil
	} else if t, err := host.pubsub.Join(topic); err != nil {
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

	if len(msg) == 0 {
		return errors.New("cannot send out empty message")
	}

	for _, group := range groups {
		t, e := host.GetOrJoin(string(group))
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

// GetID returns ID.Pretty
func (host *HostV2) GetID() libp2p_peer.ID {
	return host.h.ID()
}

// GetSelfPeer gets self peer
func (host *HostV2) GetSelfPeer() Peer {
	return host.self
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

// NamedTopic represents pubsub topic
// Name is the human readable topic, groupID
type NamedTopic struct {
	Name  string
	Topic *libp2p_pubsub.Topic
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

// DefaultBootNodeAddrStrings is a list of Harmony
// bootnodes address. Used to find other peers in the network.
var DefaultBootNodeAddrStrings = []string{
	"/ip4/127.0.0.1/tcp/19876/p2p/Qmc1V6W7BwX8Ugb42Ti8RnXF1rY5PF7nnZ6bKBryCgi6cv",
}

// BootNodes is a list of boot nodes.
// It is populated either from default or from user CLI input.
var BootNodes AddrList
