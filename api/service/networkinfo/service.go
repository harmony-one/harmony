package networkinfo

import (
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// Service is the network info service.
type Service struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
	peerChan    chan *p2p.Peer
}

// New returns network info service.
func New(peerChan chan *p2p.Peer) *Service {
	return &Service{
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		peerChan:    peerChan,
	}
}

// StartService starts network info service.
func (s *Service) StartService() {
	s.Init()
	s.Run()
}

// Init initializes network info service.
func (s *Service) Init() {
}

// Run runs network info.
func (s *Service) Run() {
	go func() {
		defer close(s.stoppedChan)
		for {
			select {
			default:
				utils.GetLogInstance().Info("Running network info")
				// TODO: Write some logic here.
				s.DoService()
			case <-s.stopChan:
				return
			}
		}
	}()
}

// DoService does network info.
func (s *Service) DoService() {
	// At the end, send Peer info to peer channel
	s.peerChan <- &p2p.Peer{}
}

// StopService stops network info service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping network info service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Role conversion stopped.")
}
