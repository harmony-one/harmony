package randomness

import (
	"github.com/harmony-one/harmony/drand"
	"github.com/harmony-one/harmony/internal/utils"
)

// Service is the randomness generation service.
type Service struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
	DRand       *drand.DRand
}

// New returns randomness generation service.
func New(dRand *drand.DRand) *Service {
	return &Service{DRand: dRand}
}

// StartService starts randomness generation service.
func (s *Service) StartService() {
	s.stopChan = make(chan struct{})
	s.stoppedChan = make(chan struct{})
	s.DRand.WaitForEpochBlock(s.DRand.ConfirmedBlockChannel, s.stopChan, s.stoppedChan)
}

// StopService stops randomness generation service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping random generation service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Random generation stopped.")
}
