package blockproposal

import (
	"github.com/harmony-one/harmony/internal/utils"
)

// Service is the new block service.
type Service struct {
	stopChan              chan struct{}
	stoppedChan           chan struct{}
	readySignal           chan struct{}
	waitForConsensusReady func(readySignal chan struct{}, stopChan chan struct{}, stoppedChan chan struct{})
}

// NewService returns new block service.
func NewService(readySignal chan struct{}, waitForConsensusReady func(readySignal chan struct{}, stopChan chan struct{}, stoppedChan chan struct{})) *Service {
	return &Service{readySignal: readySignal, waitForConsensusReady: waitForConsensusReady}
}

// StartService starts new block service.
func (s *Service) StartService() {
	s.stopChan = make(chan struct{})
	s.stoppedChan = make(chan struct{})

	s.Init()
	s.Run(s.stopChan, s.stoppedChan)
}

// Init initializes new block service.
func (s *Service) Init() {
}

// Run runs new block.
func (s *Service) Run(stopChan chan struct{}, stoppedChan chan struct{}) {
	s.waitForConsensusReady(s.readySignal, s.stopChan, s.stoppedChan)
}

// StopService stops new block service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping new block service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Role conversion stopped.")
}
