package discovery

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"

	peerstore "github.com/libp2p/go-libp2p-peerstore"

	libp2pdis "github.com/libp2p/go-libp2p-discovery"
	libp2pdht "github.com/libp2p/go-libp2p-kad-dht"
)

// Constants for discovery service.
const (
	numIncoming = 128
	numOutgoing = 16
)

// Service is the struct for discovery service.
type Service struct {
	Host       p2p.Host
	DHT        *libp2pdht.IpfsDHT
	Rendezvous string
	ctx        context.Context
	peerChan   <-chan peerstore.PeerInfo
}

// New returns discovery service.
// h is the p2p host
// r is the rendezvous string, we use shardID to start (TODO: leo, build two overlays of network)
func New(h p2p.Host, r string) *Service {
	ctx := context.Background()
	dht, err := libp2pdht.New(ctx, h.GetP2PHost())
	if err != nil {
		panic(err)
	}

	return &Service{
		Host:       h,
		DHT:        dht,
		Rendezvous: r,
		ctx:        ctx,
		peerChan:   make(<-chan peerstore.PeerInfo),
	}
}

// StartService starts discovery service.
func (s *Service) StartService() {
	log.Info("Starting discovery service.")
	err := s.Init()
	if err != nil {
		log.Error("StartService Aborted", "Error", err)
		return
	}

	// We use a rendezvous point "shardID" to announce our location.
	log.Info("Announcing ourselves...")
	routingDiscovery := libp2pdis.NewRoutingDiscovery(s.DHT)
	libp2pdis.Advertise(s.ctx, routingDiscovery, s.Rendezvous)
	log.Debug("Successfully announced!")

	log.Debug("Searching for other peers...")
	s.peerChan, err = routingDiscovery.FindPeers(s.ctx, s.Rendezvous)
	if err != nil {
		log.Error("FindPeers", "error", err)
	}
}

// StopService shutdowns discovery service.
func (s *Service) StopService() {
	log.Info("Shutting down discovery service.")
}

func (s *Service) foundPeers() {
	for {
		select {
		case peer := <-s.peerChan:
			if peer.ID != s.Host.GetP2PHost().ID() && len(peer.ID) > 0 {
				log.Debug("Found Peer", "peer", peer.ID, "addr", peer.Addrs, "len", len(peer.ID))
				p := p2p.Peer{PeerID: peer.ID, Addrs: peer.Addrs}
				s.Host.AddPeer(&p)
				// TODO: stop ping if pinged before
				s.pingPeer(p)
			}
		}
	}
}

// Init is to initialize for discoveryService.
func (s *Service) Init() error {
	log.Info("Init discovery service")

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	log.Debug("Bootstrapping the DHT")
	if err := s.DHT.Bootstrap(s.ctx); err != nil {
		return ErrDHTBootstrap
	}

	var wg sync.WaitGroup
	for _, peerAddr := range utils.BootNodes {
		peerinfo, _ := peerstore.InfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Host.GetP2PHost().Connect(s.ctx, *peerinfo); err != nil {
				log.Warn("can't connect to bootnode", "error", err)
			} else {
				log.Info("connected to bootnode", "node", *peerinfo)
			}
		}()
	}
	wg.Wait()

	go s.foundPeers()

	return nil
}

func (s *Service) pingPeer(peer p2p.Peer) {
	ping := proto_discovery.NewPingMessage(s.Host.GetSelfPeer())
	buffer := ping.ConstructPingMessage()
	log.Debug("Sending Ping Message to", "peer", peer)
	content := host.ConstructP2pMessage(byte(0), buffer)
	s.Host.SendMessage(peer, content)
	log.Debug("Sent Ping Message to", "peer", peer)
}
