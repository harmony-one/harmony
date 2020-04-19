package p2p

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"

	"github.com/harmony-one/bls/ffi/go/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p/ipfsutil"
	"github.com/ipfs/go-datastore"
	sync_ds "github.com/ipfs/go-datastore/sync"
	ipfs_cfg "github.com/ipfs/go-ipfs-config"
	ipfs_core "github.com/ipfs/go-ipfs/core"
	ipfs_interface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/juju/fslock"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Peer is the object for a p2p peer (node)
type Peer struct {
	IP string // IP address of the peer
	// TODO this should not be string
	Port            string         // Port number of the peer
	ConsensusPubKey *bls.PublicKey // Public key of the peer, used for consensus signing
	PeerID          libp2p_peer.ID // PeerID, the pubkey for communication
}

// Host is the client + server in p2p network.
type Host interface {
	GetSelfPeer() *Peer
	SendMessageToGroups(groups []nodeconfig.GroupID, msg []byte) error
	GetPeerCount() int
	AllSubscriptions() []ipfs_interface.PubSubSubscription
}

var DefaultBootstrap = ipfs_cfg.DefaultBootstrapAddresses

type Opts struct {
	Bootstrap             []string
	RendezVousServerMAddr string
	Port                  uint
	RootDS                datastore.Batching
	Logger                *zerolog.Logger
}

type hmyHost struct {
	// Temp hack to satisfy all methods
	Host
	coreAPI ipfs_interface.CoreAPI
	node    *ipfs_core.IpfsNode
	log     *zerolog.Logger
	ownPeer *Peer
	lock    sync.Mutex
	joined  map[string]ipfs_interface.PubSubSubscription
}

func unlockFS(l *fslock.Lock) {
	if l == nil {
		return
	}

	err := l.Unlock()
	if err != nil {
		panic(err)
	}
}

func panicUnlockFS(err error, l *fslock.Lock) {
	unlockFS(l)
	panic(err)
}

func fatal(err error) {
	fmt.Println("died, why", err.Error())
	panic("end")
}

func (h *hmyHost) GetSelfPeer() *Peer {
	return h.ownPeer
}

func (h *hmyHost) SendMessageToGroups(groups []nodeconfig.GroupID, msg []byte) (e error) {
	ctx := context.Background()

	for _, group := range groups {
		top := string(group)
		_, err := h.getTopic(top)
		if err != nil {
			e = err
			continue
		}
		if err := h.coreAPI.PubSub().Publish(ctx, top, msg); err != nil {
			e = err
			continue
		}
	}

	return e
}

func (h *hmyHost) getTopic(topic string) (ipfs_interface.PubSubSubscription, error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if t, ok := h.joined[topic]; ok {
		return t, nil
	}

	sub, err := h.coreAPI.PubSub().Subscribe(context.Background(), topic)

	if err != nil {
		return nil, errors.Wrapf(err, "cannot join pubsub topic %x", topic)
	}

	h.joined[topic] = sub
	return sub, nil
}

func (h *hmyHost) AllSubscriptions() []ipfs_interface.PubSubSubscription {
	subs := []ipfs_interface.PubSubSubscription{}
	h.lock.Lock()
	defer h.lock.Unlock()
	for _, g := range h.joined {
		subs = append(subs, g)
	}
	return subs
}

func (h *hmyHost) GetPeerCount() int {
	conns, _ := h.coreAPI.Swarm().Peers(context.Background())
	return len(conns)
}

// NewHost ..
func NewHost(opts *Opts, own *Peer) (Host, error) {
	var swarmAddresses []string

	if opts.Port != 0 {
		swarmAddresses = []string{
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", opts.Port),
			fmt.Sprintf("/ip6/0.0.0.0/tcp/%d", opts.Port),
			// TODO Hold up, need httpu
			// fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", opts.Port+1),
			// fmt.Sprintf("/ip6/0.0.0.0/udp/%d/quic", opts.Port+1),
		}
	} else {
		swarmAddresses = []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/0.0.0.0/tcp/0",
			// "/ip4/0.0.0.0/udp/0/quic",
			// "/ip6/0.0.0.0/udp/0/quic",
		}
	}

	mardv, err := multiaddr.NewMultiaddr(opts.RendezVousServerMAddr)
	if err != nil {
		fatal(err)
	}

	rdvpeer, err := libp2p_peer.AddrInfoFromP2pAddr(mardv)
	if err != nil {
		fatal(err)
	}

	ctx := context.Background()
	rootDS := sync_ds.MutexWrap(opts.RootDS)
	ipfsDS := ipfsutil.NewNamespacedDatastore(rootDS, datastore.NewKey("ipfs"))
	cfg, err := ipfsutil.CreateBuildConfigWithDatastore(&ipfsutil.BuildOpts{
		SwarmAddresses: swarmAddresses,
	}, ipfsDS)
	if err != nil {
		fatal(err)
	}

	routingOpt, crouting := ipfsutil.NewTinderRouting(
		opts.Logger, rdvpeer.ID, false,
	)
	cfg.Routing = routingOpt

	ipfsConfig, err := cfg.Repo.Config()
	if err != nil {
		fatal(err)
	}

	ipfsConfig.Bootstrap = append(opts.Bootstrap, opts.RendezVousServerMAddr)
	if err := cfg.Repo.SetConfig(ipfsConfig); err != nil {
		fatal(err)
	}

	api, node, err := ipfsutil.NewConfigurableCoreAPI(
		ctx,
		cfg,
		ipfsutil.OptionMDNSDiscovery,
	)
	if err != nil {
		fatal(err)
	}
	// wait to get routing
	routing := <-crouting
	routing.RoutingTable().Print()

	fmt.Println("node->",
		node.Identity,
		node.PeerHost.Addrs(),
	)

	for {
		if err := node.PeerHost.Connect(ctx, *rdvpeer); err != nil {
			fmt.Println("something busted, why", err.Error())
			opts.Logger.Error().Err(err).Msg("cannot dial rendezvous point")
		} else {
			fmt.Println(
				"was able to connect to the rendevous peer",
				rdvpeer.Loggable(),
				rdvpeer.String(),
			)
			break
		}
	}

	own.PeerID = node.PeerHost.ID()

	return &hmyHost{
		coreAPI: api,
		node:    node,
		log:     opts.Logger,
		ownPeer: own,
		lock:    sync.Mutex{},
		joined:  map[string]ipfs_interface.PubSubSubscription{},
	}, nil
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
type AddrList []multiaddr.Multiaddr

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
		addr, err := multiaddr.NewMultiaddr(a)
		if err != nil {
			return err
		}
		*al = append(*al, addr)
	}
	return nil
}
