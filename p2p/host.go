package p2p

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/harmony-one/bls/ffi/go/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p/ipfsutil"
	"github.com/ipfs/go-datastore"
	sync_ds "github.com/ipfs/go-datastore/sync"
	ipfs_core "github.com/ipfs/go-ipfs/core"
	ipfs_interface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/juju/fslock"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
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
	GetID() libp2p_peer.ID
	AllSubscriptions() []NamedSub
}

type Opts struct {
	Bootstrap             []string
	RendezVousServerMAddr string
	Port                  uint
	RootDS                datastore.Batching
	Logger                *zerolog.Logger
}

type hmyHost struct {
	// Temp hack to satisfy all methods
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

func (h *hmyHost) GetID() libp2p_peer.ID {
	return h.node.PeerHost.ID()
	// return h.ownPeer.IP
}

func (h *hmyHost) SendMessageToGroups(groups []nodeconfig.GroupID, msg []byte) error {
	ctx := context.Background()
	var g errgroup.Group

	for _, group := range groups {
		top := string(group)
		_, err := h.getTopic(top)
		if err != nil {
			return err
		}
		g.Go(func() error {
			return h.coreAPI.PubSub().Publish(ctx, top, msg)
		})
	}

	return g.Wait()
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

type NamedSub struct {
	Topic string
	Sub   ipfs_interface.PubSubSubscription
}

func (h *hmyHost) AllSubscriptions() []NamedSub {
	subs := []NamedSub{}
	h.lock.Lock()
	defer h.lock.Unlock()
	for name, g := range h.joined {
		subs = append(subs, NamedSub{name, g})
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
			fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", opts.Port),
			fmt.Sprintf("/ip6/127.0.0.1/tcp/%d", opts.Port),
			// TODO Hold up, need httpu
			// fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", opts.Port+1),
			// fmt.Sprintf("/ip6/0.0.0.0/udp/%d/quic", opts.Port+1),
		}
	} else {
		swarmAddresses = []string{
			"/ip4/127.0.0.1/tcp/0",
			"/ip6/127.0.0.1/tcp/0",
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

	// block till have network connectivity
	<-crouting

	for {
		if err := node.PeerHost.Connect(ctx, *rdvpeer); err != nil {
			opts.Logger.Error().Err(err).Msg("cannot dial rendezvous point")
		} else {
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
