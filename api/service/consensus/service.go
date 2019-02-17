package consensus

import (
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

// Service is the consensus service.
type Service struct {
	blockChannel chan *types.Block // The channel to receive new blocks from Node
	consensus    *consensus.Consensus
	stopChan     chan struct{}
	stoppedChan  chan struct{}
	startChan    chan struct{}
}

// New returns consensus service.
func New(blockChannel chan *types.Block, consensus *consensus.Consensus, startChan chan struct{}) *Service {
	return &Service{blockChannel: blockChannel, consensus: consensus, startChan: startChan}
}

// StartService starts consensus service.
func (s *Service) StartService() {
	s.stopChan = make(chan struct{})
	s.stoppedChan = make(chan struct{})
	s.consensus.WaitForNewBlock(s.blockChannel, s.stopChan, s.stoppedChan, s.startChan)
}

// StopService stops consensus service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping consensus service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Consensus service stopped.")
}

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {
	return
}
