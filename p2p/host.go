package p2p

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/harmony-one/bls/ffi/go/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p/ipfsutil"
	"github.com/ipfs/go-datastore"
	sync_ds "github.com/ipfs/go-datastore/sync"
	ipfs_cfg "github.com/ipfs/go-ipfs-config"
	ipfs_core "github.com/ipfs/go-ipfs/core"
	ipfs_interface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/juju/fslock"
	libp2p_host "github.com/libp2p/go-libp2p-core/host"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

// Peer is the object for a p2p peer (node)
type Peer struct {
	IP              string                // IP address of the peer
	Port            string                // Port number of the peer
	ConsensusPubKey *bls.PublicKey        // Public key of the peer, used for consensus signing
	Addrs           []multiaddr.Multiaddr // MultiAddress of the peer
	PeerID          libp2p_peer.ID        // PeerID, the pubkey for communication
}

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

// NewHost ..
func NewHost(opts *Opts) (Host, error) {
	var swarmAddresses []string

	if opts.Port != 0 {
		swarmAddresses = []string{
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", opts.Port),
			fmt.Sprintf("/ip6/0.0.0.0/tcp/%d", opts.Port),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", opts.Port+1),
			fmt.Sprintf("/ip6/0.0.0.0/udp/%d/quic", opts.Port+1),
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

		time.Sleep(time.Second)
	}

	const topic = "hello-world"

	sub, err := api.PubSub().Subscribe(ctx, topic)

	if err != nil {
		fatal(err)
	}

	go func() {
		for range time.NewTicker(time.Second * 5).C {
			fmt.Println("I am publishing now", node.PeerHost.ID())
			if err := api.PubSub().Publish(
				ctx, topic, []byte("some junk data, what"),
			); err != nil {
				fmt.Println("why couldnt publish the message?")
			}
		}
	}()

	for range time.NewTicker(time.Second * 1).C {
		msg, err := sub.Next(ctx)
		if err != nil {
			fmt.Println("nothing")
		}
		sender := msg.From()
		if sender != node.PeerHost.ID() {
			fmt.Println(
				"got real data", string(msg.Data()), msg.From(), msg.Topics(),
			)
			fmt.Println(
				"who am i connected to",
				node.PubSub.ListPeers(topic),
				"my peer ID =>", node.PeerHost.ID(),
			)
		}
	}

	return &hmyHost{
		coreAPI: api,
		node:    node,
		log:     utils.NetworkLogger(),
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
