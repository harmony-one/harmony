package networkinfo

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	libp2pdis "github.com/libp2p/go-libp2p-discovery"
	libp2pdht "github.com/libp2p/go-libp2p-kad-dht"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	manet "github.com/multiformats/go-multiaddr-net"
)

// Service is the network info service.
type Service struct {
	Host        p2p.Host
	Rendezvous  p2p.GroupID
	bootnodes   utils.AddrList
	dht         *libp2pdht.IpfsDHT
	cancel      context.CancelFunc
	stopChan    chan struct{}
	stoppedChan chan struct{}
	peerChan    chan p2p.Peer
	peerInfo    <-chan peerstore.PeerInfo
	discovery   *libp2pdis.RoutingDiscovery
	messageChan chan *msg_pb.Message
	started     bool
}

// ConnectionRetry set the number of retry of connection to bootnode in case the initial connection is failed
var (
	// retry for 10 minutes and give up then
	ConnectionRetry = 300

	// context
	ctx context.Context
)

const (
	waitInRetry       = 2 * time.Second
	connectionTimeout = 3 * time.Minute

	// register to bootnode every ticker
	dhtTicker = 6 * time.Hour

	// wait for peerinfo.
	peerInfoWait = time.Second
)

// New returns role conversion service.
func New(h p2p.Host, rendezvous p2p.GroupID, peerChan chan p2p.Peer, bootnodes utils.AddrList) *Service {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(context.Background(), connectionTimeout)
	dht, err := libp2pdht.New(ctx, h.GetP2PHost())
	if err != nil {
		panic(err)
	}

	return &Service{
		Host:        h,
		dht:         dht,
		Rendezvous:  rendezvous,
		cancel:      cancel,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		peerChan:    peerChan,
		bootnodes:   bootnodes,
		discovery:   nil,
		started:     false,
	}
}

// StartService starts network info service.
func (s *Service) StartService() {
	err := s.Init()
	if err != nil {
		utils.GetLogInstance().Error("Service Init Failed", "error", err)
		return
	}
	s.Run()
	s.started = true
}

// Init initializes role conversion service.
func (s *Service) Init() error {
	utils.GetLogInstance().Info("Init networkinfo service")

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	utils.GetLogInstance().Debug("Bootstrapping the DHT")
	if err := s.dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("error bootstrap dht: %s", err)
	}

	var wg sync.WaitGroup
	if s.bootnodes == nil {
		// TODO: should've passed in bootnodes through constructor.
		s.bootnodes = utils.BootNodes
	}

	connected := false
	for _, peerAddr := range s.bootnodes {
		peerinfo, _ := peerstore.InfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < ConnectionRetry; i++ {
				if err := s.Host.GetP2PHost().Connect(ctx, *peerinfo); err != nil {
					utils.GetLogInstance().Warn("can't connect to bootnode", "error", err, "try", i)
					time.Sleep(waitInRetry)
				} else {
					utils.GetLogInstance().Info("connected to bootnode", "node", *peerinfo, "try", i)
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
	utils.GetLogInstance().Info("Announcing ourselves...", "Rendezvous", string(s.Rendezvous))
	s.discovery = libp2pdis.NewRoutingDiscovery(s.dht)
	libp2pdis.Advertise(ctx, s.discovery, string(s.Rendezvous))
	utils.GetLogInstance().Info("Successfully announced!")

	return nil
}

// Run runs network info.
func (s *Service) Run() {
	defer close(s.stoppedChan)
	if s.discovery == nil {
		utils.GetLogInstance().Error("discovery is not initialized")
		return
	}

	var err error
	s.peerInfo, err = s.discovery.FindPeers(ctx, string(s.Rendezvous))
	if err != nil {
		utils.GetLogInstance().Error("FindPeers", "error", err)
		return
	}

	go s.DoService()
}

// DoService does network info.
func (s *Service) DoService() {
	_, cgnPrefix, err := net.ParseCIDR("100.64.0.0/10")
	if err != nil {
		utils.GetLogInstance().Error("can't parse CIDR", "error", err)
		return
	}
	tick := time.NewTicker(dhtTicker)
	peerInfoTick := time.NewTicker(peerInfoWait)
	for {
		select {
		case <-peerInfoTick.C:
			select {
			case peer := <-s.peerInfo:
				if peer.ID != s.Host.GetP2PHost().ID() && len(peer.ID) > 0 {
					//	utils.GetLogInstance().Info("Found Peer", "peer", peer.ID, "addr", peer.Addrs, "my ID", s.Host.GetP2PHost().ID())
					if err := s.Host.GetP2PHost().Connect(ctx, peer); err != nil {
						utils.GetLogInstance().Warn("can't connect to peer node", "error", err, "peer", peer)
						// break if the node can't connect to peers, waiting for another peer
						break
					} else {
						utils.GetLogInstance().Info("connected to peer node", "peer", peer)
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
					if ip != "" {
						p := p2p.Peer{IP: ip, Port: port, PeerID: peer.ID, Addrs: peer.Addrs}
						utils.GetLogInstance().Info("Notify peerChan", "peer", p)
						if s.peerChan != nil {
							s.peerChan <- p
						}
					}
				}
			default:
				utils.GetLogInstance().Info("Got no peer from peerInfo")
			}
		case <-s.stopChan:
			return
		case <-tick.C:
			libp2pdis.Advertise(ctx, s.discovery, string(s.Rendezvous))
			utils.GetLogInstance().Info("Successfully announced!", "Rendezvous", string(s.Rendezvous))
		}
	}
}

// StopService stops network info service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping network info service.")
	defer s.cancel()

	if !s.started {
		utils.GetLogInstance().Info("Service didn't started. Exit.")
		return
	}

	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Network info service stopped.")
}

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {
	return
}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// APIs for the services.
func (s *Service) APIs() []rpc.API {
	return nil
}
