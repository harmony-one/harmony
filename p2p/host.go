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
	"time"

	"github.com/harmony-one/bls/ffi/go/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/internal/utils/blockedpeers"
	"github.com/harmony-one/harmony/p2p/discovery"
	"github.com/harmony-one/harmony/p2p/security"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2p_config "github.com/libp2p/go-libp2p/config"
	libp2p_crypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2p_host "github.com/libp2p/go-libp2p/core/host"
	libp2p_network "github.com/libp2p/go-libp2p/core/network"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	libp2p_peerstore "github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/libp2p/go-libp2p/core/sec/insecure"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	mplex "github.com/libp2p/go-libp2p/p2p/muxer/mplex"
	yamux "github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	noise "github.com/libp2p/go-libp2p/p2p/security/noise"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"
	madns "github.com/multiformats/go-multiaddr-dns"
)

type ConnectCallback func(net libp2p_network.Network, conn libp2p_network.Conn) error

type DisconnectCallback func(conn libp2p_network.Conn) error

// Host is the client + server in p2p network.
type Host interface {
	Start() error
	Close() error
	GetSelfPeer() Peer
	AddPeer(*Peer) error
	GetID() libp2p_peer.ID
	GetP2PHost() libp2p_host.Host
	GetDiscovery() discovery.Discovery
	GetPeerCount() int
	ConnectHostPeer(Peer) error
	// AddStreamProtocol add the given protocol
	AddStreamProtocol(protocols ...sttypes.Protocol)
	// SendMessageToGroups sends a message to one or more multicast groups.
	SendMessageToGroups(groups []nodeconfig.GroupID, msg []byte) error
	PubSub() *libp2p_pubsub.PubSub
	PeerConnectivity() (int, int, int)
	GetOrJoin(topic string) (*libp2p_pubsub.Topic, error)
	ListPeer(topic string) []libp2p_peer.ID
	ListTopic() []string
	ListBlockedPeer() []libp2p_peer.ID
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
	SetAsideOtherwise = 1 << 11
	// MaxMessageHandlers ..
	MaxMessageHandlers = SetAsideForConsensus + SetAsideOtherwise
	// MaxMessageSize is 2Mb
	MaxMessageSize = 1 << 21
)

// Host Multiplexer's Type
type MuxerType int

const (
	Mplex MuxerType = iota
	Yamux
)

// HostConfig is the config structure to create a new host
type HostConfig struct {
	Self                     *Peer
	BLSKey                   libp2p_crypto.PrivKey
	BootNodes                []string
	DataStoreFile            *string
	DiscConcurrency          int
	MaxConnPerIP             int
	DisablePrivateIPScan     bool
	MaxPeers                 int64
	ConnManagerLowWatermark  int
	ConnManagerHighWatermark int
	WaitForEachPeerToConnect bool
	ForceReachabilityPublic  bool
	NoTransportSecurity      bool
	NAT                      bool
	UserAgent                string
	DialTimeout              time.Duration
	MuxerType                MuxerType
	NoRelay                  bool
}

func init() {
	libp2p_pubsub.GossipSubDlazy = 4
	libp2p_pubsub.GossipSubGossipFactor = 0.15
	libp2p_pubsub.GossipSubD = 5
	libp2p_pubsub.GossipSubDlo = 4
	libp2p_pubsub.GossipSubDhi = 8
	libp2p_pubsub.GossipSubHistoryLength = 2
	libp2p_pubsub.GossipSubHistoryGossip = 2
	libp2p_pubsub.GossipSubGossipRetransmission = 2
	libp2p_pubsub.GossipSubFanoutTTL = 10 * time.Second
	libp2p_pubsub.GossipSubMaxPendingConnections = 32
	libp2p_pubsub.GossipSubMaxIHaveLength = 1000
}

// NewHost ..
func NewHost(cfg HostConfig) (Host, error) {
	var (
		self = cfg.Self
		key  = cfg.BLSKey
	)

	addr := fmt.Sprintf("/ip4/%s/tcp/%s", self.IP, self.Port)
	listenAddr := libp2p.ListenAddrStrings(
		addr, // regular tcp connections
	)

	// transporters
	tcpTransport := libp2p.Transport(
		tcp.NewTCPTransport,
		tcp.WithConnectionTimeout(time.Minute*60)) // break unused connections

	// create NAT Manager; it takes care of setting NAT port mappings, and discovering external addresses
	var nat libp2p_config.NATManagerC // disabled if nil
	if cfg.NAT {
		nat = basichost.NewNATManager
	}

	ctx, cancel := context.WithCancel(context.Background())

	// create connection manager
	low := cfg.ConnManagerLowWatermark
	high := cfg.ConnManagerHighWatermark
	connMngr, err := connectionManager(low, high)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to open connection manager: %w", err)
	}

	// relay
	var relay libp2p_config.Option
	if cfg.NoRelay {
		relay = libp2p.DisableRelay() // No relay services, direct connections between peers only.
	} else {
		relay = libp2p.EnableRelayService()
	}

	// set dial timeout
	dto := cfg.DialTimeout
	if dto <= 0 {
		dto = time.Minute
	}

	// prepare host options
	var idht *dht.IpfsDHT
	p2pHostConfig := []libp2p.Option{
		libp2p.Identity(key),
		// Explicitly set the user-agent, so we can differentiate from other Go libp2p users.
		libp2p.UserAgent(cfg.UserAgent),
		// regular tcp connections
		tcpTransport,
		// dial timeout
		libp2p.WithDialTimeout(dto),
		// No relay services, direct connections between peers only.
		relay,
		// host will start and listen to network directly after construction from config.
		listenAddr,
		/*
			libp2p.ConnectionGater(connGtr), // TODO use connection gater to monitor the connections
			libp2p.ResourceManager(nil), // TODO use resource manager interface to manage resources per peer better
			libp2p.Peerstore(ps), // TODO add extended peer store
		*/
		// Connection manager
		connMngr,
		// NAT manager
		libp2p.NATManager(nat),
		// Band width Reporter
		libp2p.BandwidthReporter(newCounter()), // may be nil if disabled
		// Resolver
		libp2p.MultiaddrResolver(madns.DefaultResolver),
		// Ping is a small built-in libp2p protocol that helps us check/debug latency between peers.
		libp2p.Ping(true),
		// Help peers with their NAT reachability status, but throttle to avoid too much work.
		libp2p.EnableNATService(),
		// NAT Rate Limiter
		libp2p.AutoNATServiceRateLimit(10, 5, time.Second*60),
	}

	// Set host security
	if cfg.NoTransportSecurity {
		p2pHostConfig = append(p2pHostConfig, libp2p.Security(insecure.ID, insecure.NewWithIdentity))
	} else {
		p2pHostConfig = append(p2pHostConfig, NoiseC(), TlsC())
	}

	// Set Muxer (default: Mplex)
	switch cfg.MuxerType {
	case Yamux:
		p2pHostConfig = append(p2pHostConfig, YamuxC())
	case Mplex:
		p2pHostConfig = append(p2pHostConfig, MplexC())
	default:
		utils.Logger().Error().
			Interface("muxer", cfg.MuxerType).
			Msg("Muxer type is invalid, it is set on default value (mplex)")
		p2pHostConfig = append(p2pHostConfig, MplexC())
	}

	if cfg.ForceReachabilityPublic {
		// ForceReachabilityPublic overrides automatic reachability detection in the AutoNAT subsystem,
		// forcing the local node to believe it is reachable externally
		p2pHostConfig = append(p2pHostConfig, libp2p.ForceReachabilityPublic())
	}

	if cfg.DisablePrivateIPScan {
		// Prevent dialing of public addresses
		p2pHostConfig = append(p2pHostConfig, libp2p.ConnectionGater(NewGater(cfg.DisablePrivateIPScan)))
	}

	// create p2p host
	p2pHost, err := libp2p.New(p2pHostConfig...)
	if err != nil {
		cancel()
		return nil, errors.Wrapf(err, "cannot initialize libp2p host")
	}

	// DHT
	opt := discovery.DHTConfig{
		BootNodes:       cfg.BootNodes,
		DataStoreFile:   cfg.DataStoreFile,
		DiscConcurrency: cfg.DiscConcurrency,
	}
	opts, err := opt.GetLibp2pRawOptions()
	if err != nil {
		cancel()
		return nil, errors.Wrapf(err, "initialize libp2p raw options failed")
	}
	idht, errDHT := dht.New(ctx, p2pHost, opts...)
	if errDHT != nil {
		cancel()
		return nil, errors.Wrapf(errDHT, "cannot initialize libp2p DHT")
	}
	disc, err := discovery.NewDHTDiscovery(ctx, cancel, p2pHost, idht, opt)
	if err != nil {
		cancel()
		p2pHost.Close()
		return nil, errors.Wrap(err, "cannot create DHT discovery")
	}

	// Gossip Pub Sub
	options := []libp2p_pubsub.Option{
		// WithValidateQueueSize sets the buffer of validate queue. Defaults to 32. When queue is full, validation is throttled and new messages are dropped.
		libp2p_pubsub.WithValidateQueueSize(512),
		// WithPeerOutboundQueueSize is an option to set the buffer size for outbound messages to a peer. We start dropping messages to a peer if the outbound queue if full.
		libp2p_pubsub.WithPeerOutboundQueueSize(64),
		// WithValidateWorkers sets the number of synchronous validation worker goroutines. Defaults to NumCPU.
		libp2p_pubsub.WithValidateWorkers(runtime.NumCPU() * 2),
		// WithValidateThrottle sets the upper bound on the number of active validation goroutines across all topics. The default is 8192.
		libp2p_pubsub.WithValidateThrottle(MaxMessageHandlers),
		libp2p_pubsub.WithMaxMessageSize(MaxMessageSize),
		libp2p_pubsub.WithDiscovery(disc.GetRawDiscovery()),
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
		cancel()
		p2pHost.Close()
		return nil, errors.Wrapf(err, "cannot initialize libp2p pub-sub")
	}

	self.PeerID = p2pHost.ID()
	subLogger := utils.Logger().With().Str("hostID", p2pHost.ID().Pretty()).Logger()

	banned := blockedpeers.NewManager(1024)
	security := security.NewManager(cfg.MaxConnPerIP, int(cfg.MaxPeers), banned)
	// has to save the private key for host
	h := &HostV2{
		h:             p2pHost,
		pubsub:        pubsub,
		joined:        map[string]*libp2p_pubsub.Topic{},
		self:          *self,
		priKey:        key,
		discovery:     disc,
		security:      security,
		onConnections: ConnectCallbacks{},
		onDisconnects: DisconnectCallbacks{},
		logger:        &subLogger,
		ctx:           ctx,
		cancel:        cancel,
		banned:        banned,
	}

	utils.Logger().Info().
		Str("self", net.JoinHostPort(self.IP, self.Port)).
		Interface("PeerID", self.PeerID).
		Str("PubKey", self.ConsensusPubKey.SerializeToHexStr()).
		Msg("libp2p host ready")
	return h, nil
}

func YamuxC() libp2p.Option {
	return libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport)
}

func MplexC() libp2p.Option {
	return libp2p.Muxer("/mplex/6.7.0", mplex.DefaultTransport)
}

func NoiseC() libp2p.Option {
	return libp2p.Security(noise.ID, noise.New)
}

func TlsC() libp2p.Option {
	return libp2p.Security(tls.ID, tls.New)
}

// connectionManager creates a new connection manager and configures libp2p to use the
// given connection manager.
// lo and hi are watermarks governing the number of connections that'll be maintained.
// When the peer count exceeds the 'high watermark', as many peers will be pruned (and
// their connections terminated) until 'low watermark' peers remain.
func connectionManager(low int, high int) (libp2p_config.Option, error) {
	if low < 0 || high < low {
		utils.Logger().Error().
			Int("low", low).
			Int("high", high).
			Msg("connection manager watermarks are invalid")
		return nil, errors.New("invalid connection manager watermarks")
	}
	connmgr, err := connmgr.NewConnManager(
		low,  // Low Watermark
		high, // High Watermark
		connmgr.WithGracePeriod(time.Minute),
		connmgr.WithSilencePeriod(time.Minute),
		connmgr.WithEmergencyTrim(true))
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Int("low", low).
			Int("high", high).
			Msg("create connection manager failed")
		return nil, err
	}
	return libp2p.ConnectionManager(connmgr), nil
}

// HostV2 is the version 2 p2p host
type HostV2 struct {
	h             libp2p_host.Host
	pubsub        *libp2p_pubsub.PubSub
	joined        map[string]*libp2p_pubsub.Topic
	streamProtos  []sttypes.Protocol
	self          Peer
	priKey        libp2p_crypto.PrivKey
	lock          sync.Mutex
	discovery     discovery.Discovery
	security      security.Security
	logger        *zerolog.Logger
	blocklist     libp2p_pubsub.Blacklist
	onConnections ConnectCallbacks
	onDisconnects DisconnectCallbacks
	ctx           context.Context
	cancel        func()
	banned        *blockedpeers.Manager
}

// PubSub ..
func (host *HostV2) PubSub() *libp2p_pubsub.PubSub {
	return host.pubsub
}

// Start start the HostV2 discovery process
// TODO: move PubSub start handling logic here
func (host *HostV2) Start() error {
	host.h.Network().Notify(host)
	host.SetConnectCallback(host.security.OnConnectCheck)
	host.SetDisconnectCallback(host.security.OnDisconnectCheck)
	for _, proto := range host.streamProtos {
		proto.Start()
	}
	return host.discovery.Start()
}

// Close closes the HostV2.
func (host *HostV2) Close() error {
	for _, proto := range host.streamProtos {
		proto.Close()
	}
	host.discovery.Close()
	host.cancel()
	return host.h.Close()
}

// PeerConnectivity returns total number of known, connected and not connected peers.
func (host *HostV2) PeerConnectivity() (int, int, int) {
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

// AddStreamProtocol adds the stream protocols to the host to be started and closed
// when the host starts or close
func (host *HostV2) AddStreamProtocol(protocols ...sttypes.Protocol) {
	for _, proto := range protocols {
		host.streamProtos = append(host.streamProtos, proto)
		host.h.SetStreamHandlerMatch(protocol.ID(proto.ProtoID()), proto.Match, proto.HandleStream)
		// TODO: do we need to add handler match for shard proto id?
		// if proto.IsBeaconNode() {
		// 	host.h.SetStreamHandlerMatch(protocol.ID(proto.ShardProtoID()), proto.Match, proto.HandleStream)
		// }
	}
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

// GetDiscovery returns the underlying discovery
func (host *HostV2) GetDiscovery() discovery.Discovery {
	return host.discovery
}

// ListTopic returns the list of topic the node subscribed
func (host *HostV2) ListTopic() []string {
	host.lock.Lock()
	defer host.lock.Unlock()
	topics := make([]string, 0)
	for t := range host.joined {
		topics = append(topics, t)
	}
	return topics
}

// ListPeer returns list of peers in a topic
func (host *HostV2) ListPeer(topic string) []libp2p_peer.ID {
	host.lock.Lock()
	defer host.lock.Unlock()
	return host.joined[topic].ListPeers()
}

// ListBlockedPeer returns list of blocked peer
func (host *HostV2) ListBlockedPeer() []libp2p_peer.ID {
	return host.banned.Keys()
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

// called when network starts listening on an addr
func (host *HostV2) Listen(net libp2p_network.Network, addr ma.Multiaddr) {

}

// called when network stops listening on an addr
func (host *HostV2) ListenClose(net libp2p_network.Network, addr ma.Multiaddr) {

}

// called when a connection opened
func (host *HostV2) Connected(net libp2p_network.Network, conn libp2p_network.Conn) {
	host.logger.Debug().Interface("node", conn.RemotePeer()).Msg("peer connected")

	for _, function := range host.onConnections.GetAll() {
		if err := function(net, conn); err != nil {
			host.logger.Error().Err(err).Interface("node", conn.RemotePeer()).Msg("failed on peer connected callback")
		}
	}
}

// called when a connection closed
func (host *HostV2) Disconnected(net libp2p_network.Network, conn libp2p_network.Conn) {
	host.logger.Debug().Interface("node", conn.RemotePeer()).Msg("peer disconnected")

	for _, function := range host.onDisconnects.GetAll() {
		if err := function(conn); err != nil {
			host.logger.Error().Err(err).Interface("node", conn.RemotePeer()).Msg("failed on peer disconnected callback")
		}
	}
}

// called when a stream opened
func (host *HostV2) OpenedStream(net libp2p_network.Network, stream libp2p_network.Stream) {

}

// called when a stream closed
func (host *HostV2) ClosedStream(net libp2p_network.Network, stream libp2p_network.Stream) {

}

func (host *HostV2) SetConnectCallback(callback ConnectCallback) {
	host.onConnections.Add(callback)
}

func (host *HostV2) SetDisconnectCallback(callback DisconnectCallback) {
	host.onDisconnects.Add(callback)
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
