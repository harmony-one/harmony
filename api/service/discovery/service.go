package discovery

import (
	"time"

	"github.com/ethereum/go-ethereum/log"
	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// Service is the struct for discovery service.
type Service struct {
	host        p2p.Host
	Rendezvous  string
	peerChan    chan p2p.Peer
	stakingChan chan p2p.Peer
	stopChan    chan struct{}
}

// New returns discovery service.
// h is the p2p host
// r is the rendezvous string, we use shardID to start (TODO: leo, build two overlays of network)
func New(h p2p.Host, r string, peerChan chan p2p.Peer, stakingChan chan p2p.Peer) *Service {
	return &Service{
		host:        h,
		Rendezvous:  r,
		peerChan:    peerChan,
		stakingChan: stakingChan,
		stopChan:    make(chan struct{}),
	}
}

// StartService starts discovery service.
func (s *Service) StartService() {
	log.Info("Starting discovery service.")
	s.Init()
	s.Run()
}

// StopService shutdowns discovery service.
func (s *Service) StopService() {
	log.Info("Shutting down discovery service.")
	s.stopChan <- struct{}{}
	log.Info("discovery service stopped.")
}

// Run is the main function of the service
func (s *Service) Run() {
	go s.contactP2pPeers()
	//	go s.pingPeer()
}

func (s *Service) contactP2pPeers() {
	tick := time.NewTicker(5 * time.Second)
	ping := proto_discovery.NewPingMessage(s.host.GetSelfPeer())
	buffer := ping.ConstructPingMessage()
	content := host.ConstructP2pMessage(byte(0), buffer)
	for {
		select {
		case peer, ok := <-s.peerChan:
			if !ok {
				log.Debug("end of info", "peer", peer.PeerID)
				return
			}
			s.host.AddPeer(&peer)
			// Add to outgoing peer list
			s.host.AddOutgoingPeer(peer)
			log.Debug("[DISCOVERY]", "add outgoing peer", peer)
			// TODO: stop ping if pinged before
			// TODO: call staking servcie here if it is a new node
			if s.stakingChan != nil {
				s.stakingChan <- peer
			}
		case <-s.stopChan:
			log.Debug("[DISCOVERY] stop")
			return
		case <-tick.C:
			err := s.host.SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, content)
			if err != nil {
				log.Error("Failed to send ping message", "group", p2p.GroupIDBeacon)
			}
		}
	}
}

// Init is to initialize for discoveryService.
func (s *Service) Init() {
	log.Info("Init discovery service")
}

func (s *Service) pingPeer() {
	tick := time.NewTicker(5 * time.Second)
	ping := proto_discovery.NewPingMessage(s.host.GetSelfPeer())
	buffer := ping.ConstructPingMessage()
	content := host.ConstructP2pMessage(byte(0), buffer)

	for {
		select {
		case <-tick.C:
			err := s.host.SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, content)
			if err != nil {
				log.Error("Failed to send ping message", "group", p2p.GroupIDBeacon)
			}
		case <-s.stopChan:
			log.Info("Stop sending ping message")
			return
		}
	}
	//	s.host.SendMessage(peer, content)
	//   log.Debug("Sent Ping Message via unicast to", "peer", peer)
}
