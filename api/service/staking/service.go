package staking

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// Service is the staking service.
type Service struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
	peerChan    <-chan p2p.Peer
}

// New returns staking service.
func New(peerChan <-chan p2p.Peer) *Service {
	return &Service{
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		peerChan:    peerChan,
	}
}

// StartService starts staking service.
func (s *Service) StartService() {
	log.Info("Start Staking Service")
	s.Init()
	s.Run()
}

// Init initializes staking service.
func (s *Service) Init() {
}

// Run runs staking.
func (s *Service) Run() {
	// Wait until peer info of beacon chain is ready.
	go func() {
		defer close(s.stoppedChan)
		for {
			select {
			case peer := <-s.peerChan:
				utils.GetLogInstance().Info("Running role conversion")
				// TODO: Write some logic here.
				s.DoService(&peer)
			case <-s.stopChan:
				return
			}
		}
	}()
}

// DoService does staking.
func (s *Service) DoService(peer *p2p.Peer) {
	utils.GetLogInstance().Info("Staking with Peer")
}

// StopService stops staking service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping staking service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Role conversion stopped.")
}
