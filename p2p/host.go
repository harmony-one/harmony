package p2p

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/harmony-one/harmony/p2p/ipfsutil"
	"github.com/ipfs/go-datastore"
	sync_ds "github.com/ipfs/go-datastore/sync"
	ipfs_cfg "github.com/ipfs/go-ipfs-config"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
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
	AllTopics() []*libp2p_pubsub.Topic
}

var DefaultBootstrap = ipfs_cfg.DefaultBootstrapAddresses

type Opts struct {
	Bootstrap             []string
	RendezVousServerMAddr string
	GroupInvitation       string
	Port                  uint
	RootDS                datastore.Batching
	Logger                *zap.Logger
}

// NewHost ..
func NewHost(self *Peer, key libp2p_crypto.PrivKey) (Host, error) {
	listenAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", self.Port))
	if err != nil {
		return nil, errors.Wrapf(err,
			"cannot create listen multiaddr from port %#v", self.Port)
	}
	ctx := context.Background()
	p2pHost, err := libp2p.New(ctx,
		libp2p.ListenAddrs(listenAddr), libp2p.Identity(key),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot initialize libp2p host")
	}
	traceFile := os.Getenv("P2P_TRACEFILE")

	// const MaxSize = 2_145_728
	options := []libp2p_pubsub.Option{
		libp2p_pubsub.WithPeerOutboundQueueSize(64),
		// libp2p_pubsub.WithMaxMessageSize(MaxSize),
	}
	if len(traceFile) > 0 {
		tracer, _ := libp2p_pubsub.NewJSONTracer(traceFile)
		options = append(options, libp2p_pubsub.WithEventTracer(tracer))
	}
	pubsub, err := libp2p_pubsub.NewGossipSub(ctx, p2pHost, options...)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot initialize libp2p pubsub")
	}

	opts := Opts{
		Bootstrap:             DefaultBootstrap,
		RendezVousServerMAddr: DevRendezVousPoint,
		Port:                  0,
		RootDS:                baseDS,
		Logger:                log,
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

type topicJoiner struct {
	pubsub *libp2p_pubsub.PubSub
}

func (tj topicJoiner) JoinTopic(topic string) (*libp2p_pubsub.Topic, error) {
	th, err := tj.pubsub.Join(topic)
	if err != nil {
		return nil, err
	}
	return th, nil
}

// HostV2 is the version 2 p2p host
type HostV2 struct {
	h      libp2p_host.Host
	joiner topicJoiner
	joined map[string]*libp2p_pubsub.Topic
	self   Peer
	priKey libp2p_crypto.PrivKey
	lock   sync.Mutex
	// logger
	logger *zerolog.Logger
}

func (host *HostV2) getTopic(topic string) (*libp2p_pubsub.Topic, error) {
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
	} else {
		swarmAddresses = []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic",
			"/ip6/0.0.0.0/udp/0/quic",
		}
	}

	mardv, err := multiaddr.NewMultiaddr(opts.RendezVousServerMAddr)
	if err != nil {
		return nil, nil, err

	}

	rdvpeer, err := peer.AddrInfoFromP2pAddr(mardv)
	if err != nil {
		return nil, nil, err

	}

	ctx := context.Background()
	rootDS := sync_ds.MutexWrap(opts.RootDS)
	ipfsDS := ipfsutil.NewNamespacedDatastore(rootDS, datastore.NewKey("ipfs"))
	cfg, err := ipfsutil.CreateBuildConfigWithDatastore(&ipfsutil.BuildOpts{
		SwarmAddresses: swarmAddresses,
	}, ipfsDS)
	if err != nil {
		return nil, nil, err
	}

	routingOpt, crouting := ipfsutil.NewTinderRouting(
		opts.Logger, rdvpeer.ID, false,
	)
	cfg.Routing = routingOpt

	ipfsConfig, err := cfg.Repo.Config()
	if err != nil {
		return nil, nil, err
	}

	ipfsConfig.Bootstrap = append(opts.Bootstrap, opts.RendezVousServerMAddr)
	if err := cfg.Repo.SetConfig(ipfsConfig); err != nil {
		return nil, nil, err
	}

	api, node, err := ipfsutil.NewConfigurableCoreAPI(
		ctx,
		cfg,
		ipfsutil.OptionMDNSDiscovery,
	)
	if err != nil {
		return nil, nil, err
	}
	// wait to get routing
	routing := <-crouting
	routing.RoutingTable().Print()
	return api, node, nil
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
