package networkinfo

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

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
	ctx         context.Context
	cancel      context.CancelFunc
	stopChan    chan struct{}
	stoppedChan chan struct{}
	peerChan    chan p2p.Peer
	peerInfo    <-chan peerstore.PeerInfo
	discovery   *libp2pdis.RoutingDiscovery
	messageChan chan *msg_pb.Message
}

// New returns role conversion service.
func New(h p2p.Host, rendezvous p2p.GroupID, peerChan chan p2p.Peer, bootnodes utils.AddrList) *Service {
	timeout := 30 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	dht, err := libp2pdht.New(ctx, h.GetP2PHost())
	if err != nil {
		panic(err)
	}

	return &Service{
		Host:        h,
		dht:         dht,
		Rendezvous:  rendezvous,
		ctx:         ctx,
		cancel:      cancel,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		peerChan:    peerChan,
		bootnodes:   bootnodes,
	}
}

// StartService starts network info service.
func (s *Service) StartService() {
	s.Init()
	s.Run()
}

// Init initializes role conversion service.
func (s *Service) Init() error {
	utils.GetLogInstance().Info("Init networkinfo service")

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	utils.GetLogInstance().Debug("Bootstrapping the DHT")
	if err := s.dht.Bootstrap(s.ctx); err != nil {
		return fmt.Errorf("error bootstrap dht")
	}

	var wg sync.WaitGroup
	if s.bootnodes == nil {
		s.bootnodes = utils.BootNodes
	}

	for _, peerAddr := range s.bootnodes {
		peerinfo, _ := peerstore.InfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Host.GetP2PHost().Connect(s.ctx, *peerinfo); err != nil {
				utils.GetLogInstance().Warn("can't connect to bootnode", "error", err)
			} else {
				utils.GetLogInstance().Info("connected to bootnode", "node", *peerinfo)
			}
		}()
	}
	wg.Wait()

	// We use a rendezvous point "shardID" to announce our location.
	utils.GetLogInstance().Info("Announcing ourselves...")
	s.discovery = libp2pdis.NewRoutingDiscovery(s.dht)
	libp2pdis.Advertise(s.ctx, s.discovery, string(s.Rendezvous))
	utils.GetLogInstance().Info("Successfully announced!")

	return nil
}

// Run runs network info.
func (s *Service) Run() {
	defer close(s.stoppedChan)
	var err error
	s.peerInfo, err = s.discovery.FindPeers(s.ctx, string(s.Rendezvous))
	if err != nil {
		utils.GetLogInstance().Error("FindPeers", "error", err)
	}

	go s.DoService()
}

// DoService does network info.
func (s *Service) DoService() {
	for {
		select {
		case peer := <-s.peerInfo:
			if peer.ID != s.Host.GetP2PHost().ID() && len(peer.ID) > 0 {
				utils.GetLogInstance().Info("Found Peer", "peer", peer.ID, "addr", peer.Addrs, "my ID", s.Host.GetP2PHost().ID())
				if err := s.Host.GetP2PHost().Connect(s.ctx, peer); err != nil {
					utils.GetLogInstance().Warn("can't connect to peer node", "error", err)
				} else {
					utils.GetLogInstance().Info("connected to peer node", "peer", peer)
				}
				// figure out the public ip/port
				ip := "127.0.0.1"
				var port string
				for _, addr := range peer.Addrs {
					netaddr, err := manet.ToNetAddr(addr)
					if err != nil {
						continue
					}
					nip := netaddr.(*net.TCPAddr).IP.String()
					if strings.Compare(nip, "127.0.0.1") != 0 {
						ip = nip
						port = fmt.Sprintf("%d", netaddr.(*net.TCPAddr).Port)
						break
					}
				}
				p := p2p.Peer{IP: ip, Port: port, PeerID: peer.ID, Addrs: peer.Addrs}
				utils.GetLogInstance().Info("Notify peerChan", "peer", p)
				if s.peerChan != nil {
					s.peerChan <- p
				}
			}
		case <-s.stopChan:
			return
		}
	}
}

// StopService stops network info service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping network info service.")
	defer s.cancel()

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
