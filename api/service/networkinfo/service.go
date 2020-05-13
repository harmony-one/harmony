package networkinfo

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	badger "github.com/ipfs/go-ds-badger"
	coredis "github.com/libp2p/go-libp2p-core/discovery"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	libp2pdis "github.com/libp2p/go-libp2p-discovery"
	libp2pdht "github.com/libp2p/go-libp2p-kad-dht"
	libp2pdhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/pkg/errors"
)

// Service is the network info service.
type Service struct {
	Host        p2p.Host
	Rendezvous  nodeconfig.GroupID
	bootnodes   p2p.AddrList
	dht         *libp2pdht.IpfsDHT
	cancel      context.CancelFunc
	stopChan    chan struct{}
	stoppedChan chan struct{}
	peerChan    chan p2p.Peer
	peerInfo    <-chan libp2p_peer.AddrInfo
	discovery   *libp2pdis.RoutingDiscovery
	messageChan chan *msg_pb.Message
	started     bool
}

// ConnectionRetry set the number of retry of connection to bootnode in case the initial connection is failed
var (
	// retry for 30s and give up then
	ConnectionRetry = 15
)

const (
	waitInRetry       = 2 * time.Second
	connectionTimeout = 3 * time.Minute
	findPeerInterval  = 60 * time.Second

	// register to bootnode every ticker
	dhtTicker = 6 * time.Hour

	discoveryLimit = 32
)

// New returns role conversion service.  If dataStorePath is not empty, it
// points to a persistent database directory to use.
func New(
	h p2p.Host, rendezvous nodeconfig.GroupID,
	bootnodes p2p.AddrList, dataStorePath string,
) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())
	var dhtOpts []libp2pdhtopts.Option
	if dataStorePath != "" {
		dataStore, err := badger.NewDatastore(dataStorePath, nil)
		if err != nil {
			return nil, errors.Wrapf(err,
				"cannot open Badger datastore at %s", dataStorePath)
		}
		utils.Logger().Info().
			Str("dataStorePath", dataStorePath).
			Msg("backing DHT with Badger datastore")
		dhtOpts = append(dhtOpts, libp2pdhtopts.Datastore(dataStore))
	}

	dht, err := libp2pdht.New(ctx, h.GetP2PHost(), dhtOpts...)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create DHT")
	}

	return &Service{
		Host:        h,
		dht:         dht,
		Rendezvous:  rendezvous,
		cancel:      cancel,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		bootnodes:   bootnodes,
		discovery:   nil,
		started:     false,
	}, nil
}

// MustNew is a panic-on-error version of New.
func MustNew(
	h p2p.Host, rendezvous nodeconfig.GroupID,
	bootnodes p2p.AddrList, dataStorePath string,
) *Service {
	service, err := New(h, rendezvous, bootnodes, dataStorePath)
	if err != nil {
		panic(err)
	}
	return service
}

// StartService starts network info service.
func (s *Service) StartService() {
	err := s.Init()
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Service Init Failed")
		return
	}
	s.Run()
	s.started = true
}

// Init initializes role conversion service.
func (s *Service) Init() error {
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()
	utils.Logger().Info().Msg("Init networkinfo service")

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	utils.Logger().Debug().Msg("Bootstrapping the DHT")
	if err := s.dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("error bootstrap dht: %s", err)
	}

	var wg sync.WaitGroup
	if s.bootnodes == nil {
		// TODO: should've passed in bootnodes through constructor.
		s.bootnodes = p2p.BootNodes
	}

	connected := false
	for _, peerAddr := range s.bootnodes {
		peerinfo, _ := libp2p_peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < ConnectionRetry; i++ {
				if err := s.Host.GetP2PHost().Connect(ctx, *peerinfo); err != nil {
					utils.Logger().Warn().Err(err).Int("try", i).Msg("can't connect to bootnode")
					time.Sleep(waitInRetry)
				} else {
					utils.Logger().Info().Int("try", i).Interface("node", *peerinfo).Msg("connected to bootnode")
					// it is okay if any bootnode is connected
					connected = true
					break
				}
			}
		}()
	}
	wg.Wait()

	if !connected {
		return fmt.Errorf("[FATAL] error connecting to bootnodes")
	}

	// We use a rendezvous point "shardID" to announce our location.
	utils.Logger().Info().Str("Rendezvous", string(s.Rendezvous)).Msg("Announcing ourselves...")
	s.discovery = libp2pdis.NewRoutingDiscovery(s.dht)
	libp2pdis.Advertise(ctx, s.discovery, string(s.Rendezvous))

	// Everyone is beacon client, which means everyone is connected via beacon client topic
	// 0 is beacon chain FIXME: use a constant
	libp2pdis.Advertise(ctx, s.discovery, string(nodeconfig.NewClientGroupIDByShardID(0)))
	utils.Logger().Info().Msg("Successfully announced!")

	return nil
}

// Run runs network info.
func (s *Service) Run() {
	defer close(s.stoppedChan)
	if s.discovery == nil {
		utils.Logger().Error().Msg("discovery is not initialized")
		return
	}

	go s.DoService()
}

// DoService does network info.
func (s *Service) DoService() {
	tick := time.NewTicker(dhtTicker)
	defer tick.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		select {
		case <-s.stopChan:
			return
		case <-tick.C:
			libp2pdis.Advertise(ctx, s.discovery, string(s.Rendezvous))
			// 0 is beacon chain FIXME: use a constant
			libp2pdis.Advertise(ctx, s.discovery, string(nodeconfig.NewClientGroupIDByShardID(0)))
			utils.Logger().Info().
				Str("Rendezvous", string(s.Rendezvous)).
				Msg("Successfully announced!")
		default:
			var err error
			s.peerInfo, err = s.discovery.FindPeers(
				ctx, string(s.Rendezvous), coredis.Limit(discoveryLimit),
			)
			if err != nil {
				utils.Logger().Error().Err(err).Msg("FindPeers")
				return
			}

			s.findPeers(ctx)
			time.Sleep(findPeerInterval)
		}
	}
}

func (s *Service) findPeers(ctx context.Context) {
	_, cgnPrefix, err := net.ParseCIDR("100.64.0.0/10")
	if err != nil {
		utils.Logger().Error().Err(err).Msg("can't parse CIDR")
		return
	}
	for peer := range s.peerInfo {
		if peer.ID != s.Host.GetP2PHost().ID() && len(peer.ID) > 0 {
			if err := s.Host.GetP2PHost().Connect(ctx, peer); err != nil {
				utils.Logger().Warn().Err(err).Interface("peer", peer).Msg("can't connect to peer node")
				// break if the node can't connect to peers, waiting for another peer
				break
			} else {
				utils.Logger().Info().Interface("peer", peer).Msg("connected to peer node")
			}
			// figure out the public ip/port
			var ip, port string

			for _, addr := range peer.Addrs {
				netaddr, err := manet.ToNetAddr(addr)
				if err != nil {
					continue
				}
				nip := netaddr.(*net.TCPAddr).IP
				if (nip.IsGlobalUnicast() && !utils.IsPrivateIP(nip)) || cgnPrefix.Contains(nip) {
					ip = nip.String()
					port = fmt.Sprintf("%d", netaddr.(*net.TCPAddr).Port)
					break
				}
			}
			p := p2p.Peer{IP: ip, Port: port, PeerID: peer.ID, Addrs: peer.Addrs}
			utils.Logger().Info().Interface("peer", p).Msg("Notify peerChan")
			if s.peerChan != nil {
				s.peerChan <- p
			}
		}
	}

	utils.Logger().Info().Msg("PeerInfo Channel Closed")
}

// StopService stops network info service.
func (s *Service) StopService() {
	utils.Logger().Info().Msg("Stopping network info service")
	defer s.cancel()

	if !s.started {
		utils.Logger().Info().Msg("Service didn't started. Exit")
		return
	}

	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.Logger().Info().Msg("Network info service stopped")
}

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// APIs for the services.
func (s *Service) APIs() []rpc.API {
	return nil
}
