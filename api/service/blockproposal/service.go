package blockproposal

import (
	"github.com/harmony-one/harmony/internal/utils"
)

// Service is a block proposal service.
type Service struct {
	stopChan              chan struct{}
	stoppedChan           chan struct{}
	readySignal           chan struct{}
	waitForConsensusReady func(readySignal chan struct{}, stopChan chan struct{}, stoppedChan chan struct{})
}

// New returns a block proposal service.
func New(readySignal chan struct{}, waitForConsensusReady func(readySignal chan struct{}, stopChan chan struct{}, stoppedChan chan struct{})) *Service {
	return &Service{readySignal: readySignal, waitForConsensusReady: waitForConsensusReady}
}

// StartService starts block proposal service.
func (s *Service) StartService() {
	s.stopChan = make(chan struct{})
	s.stoppedChan = make(chan struct{})

	s.Init()
	s.Run(s.stopChan, s.stoppedChan)
}

// Init initializes block proposal service.
func (s *Service) Init() {
}

// Run runs block proposal.
func (s *Service) Run(stopChan chan struct{}, stoppedChan chan struct{}) {
	s.waitForConsensusReady(s.readySignal, s.stopChan, s.stoppedChan)
}

// StopService stops block proposal service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping block proposal service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Role conversion stopped.")
}

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {
	return
}
