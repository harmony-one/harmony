package p2p

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/Workiva/go-datastructures/trie/ctrie"
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
	IP              string         // IP address of the peer
	Port            string         // Port number of the peer
	ConsensusPubKey *bls.PublicKey // Public key of the peer, used for consensus signing
}

const Protocol = ipfsutil.Protocol

// Opts ..
type Opts struct {
	Bootstrap             []string
	RendezVousServerMAddr string
	Port                  uint
	RootDS                datastore.Batching
	Logger                *zerolog.Logger
}

// Host ..
type Host struct {
	CoreAPI    ipfs_interface.CoreAPI
	IPFSNode   *ipfs_core.IpfsNode
	log        *zerolog.Logger
	OwnPeer    *Peer
	joined     *ctrie.Ctrie
	swarmAddrs []string
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

// TODO this should take context as first arg

// SendMessageToGroups ..
func (h *Host) SendMessageToGroups(groups []nodeconfig.GroupID, msg []byte) error {
	ctx := context.Background()
	var g errgroup.Group

	for _, group := range groups {
		top := string(group)
		_, err := h.getTopic(ctx, top)
		if err != nil {
			return err
		}
		g.Go(func() error {
			return h.CoreAPI.PubSub().Publish(ctx, top, msg)
		})
	}

	return g.Wait()
}

func (h *Host) getTopic(
	ctx context.Context, topic string,
) (ipfs_interface.PubSubSubscription, error) {
	key := []byte(topic)
	pubsub, alreadyExisted := h.joined.Lookup(key)
	if alreadyExisted {
		return pubsub.(ipfs_interface.PubSubSubscription), nil
	}

	sub, err := h.CoreAPI.PubSub().Subscribe(ctx, topic)

	if err != nil {
		return nil, errors.Wrapf(err, "cannot join pubsub topic %x", topic)
	}

	h.joined.Insert(key, sub)
	return sub, nil
}

// NamedSub ..
type NamedSub struct {
	Topic string
	Sub   ipfs_interface.PubSubSubscription
}

// AllSubscriptions ..
func (h *Host) AllSubscriptions() []NamedSub {
	subs := []NamedSub{}
	for pair := range h.joined.Iterator(nil) {
		subs = append(subs, NamedSub{
			string(pair.Key),
			pair.Value.(ipfs_interface.PubSubSubscription),
		})
	}
	return subs
}

// GetPeerCount ..
func (h *Host) GetPeerCount() int {
	conns, _ := h.CoreAPI.Swarm().Peers(context.Background())
	return len(conns)
}

// DefaultLocal ..
const DefaultLocal = "127.0.0.1"

// NewHost ..
func NewHost(opts *Opts, own *Peer) (*Host, error) {
	swarmAddresses := []string{
		fmt.Sprintf("/ip4/%s/tcp/%s", DefaultLocal, own.Port),
		fmt.Sprintf("/ip6/%s/tcp/%s", DefaultLocal, own.Port),
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
		opts.Logger, rdvpeer,
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
	routing := <-crouting

	node.DHT = routing.IpfsDHT

	for {
		if err := node.PeerHost.Connect(ctx, *rdvpeer); err != nil {
			opts.Logger.Error().Err(err).Msg("cannot dial rendezvous point")
		} else {
			break
		}
	}

	if node.DHT == nil {
		panic("how can this be")
	}

	return &Host{
		CoreAPI:    api,
		IPFSNode:   node,
		log:        opts.Logger,
		OwnPeer:    own,
		joined:     ctrie.New(nil),
		swarmAddrs: swarmAddresses,
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
