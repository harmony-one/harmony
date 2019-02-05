package staking

import (
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// Service is the role conversion service.
type Service struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
	peerChan    chan *p2p.Peer
}

// New returns role conversion service.
func New(peerChan chan *p2p.Peer) *Service {
	return &Service{
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		peerChan:    peerChan,
	}
}

// StartService starts role conversion service.
func (s *Service) StartService() {
	s.Init()
	s.Run()
}

// Init initializes role conversion service.
func (s *Service) Init() {
}

// Run runs role conversion.
func (s *Service) Run() {
	// Wait until peer info of beacon chain is ready.
	peer := <-s.peerChan
	go func() {
		defer close(s.stoppedChan)
		for {
			select {
			default:
				utils.GetLogInstance().Info("Running role conversion")
				// TODO: Write some logic here.
				s.DoService(peer)
			case <-s.stopChan:
				return
			}
		}
	}()
}

// DoService does role conversion.
func (s *Service) DoService(peer *p2p.Peer) {
}

// StopService stops role conversion service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping role conversion service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Role conversion stopped.")
}
