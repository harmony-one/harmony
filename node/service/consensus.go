package service

import (
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

// ConsensusService is the consensus service.
type ConsensusService struct {
	blockChannel chan *types.Block // The channel to receive new blocks from Node
	consensus    *consensus.Consensus
	stopChan     chan struct{}
	stoppedChan  chan struct{}
}

// NewConsensusService returns consensus service.
func NewConsensusService(blockChannel chan *types.Block, consensus *consensus.Consensus) *ConsensusService {
	return &ConsensusService{blockChannel: blockChannel, consensus: consensus}
}

// StartService starts service.
func (cs *ConsensusService) StartService() {
	cs.stopChan = make(chan struct{})
	cs.stoppedChan = make(chan struct{})
	cs.consensus.WaitForNewBlock(cs.blockChannel, cs.stopChan, cs.stoppedChan)
}

// StopService stops service.
func (cs *ConsensusService) StopService() {
	utils.GetLogInstance().Info("Stopping consensus service.")
	cs.stopChan <- struct{}{}
	<-cs.stoppedChan
	utils.GetLogInstance().Info("Consensus service stopped.")
}
