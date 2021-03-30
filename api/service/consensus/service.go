package consensus

import (
	"github.com/ethereum/go-ethereum/rpc"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
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
	messageChan  chan *msg_pb.Message
}

// New returns consensus service.
func New(blockChannel chan *types.Block, consensus *consensus.Consensus, startChan chan struct{}) *Service {
	return &Service{blockChannel: blockChannel, consensus: consensus, startChan: startChan}
}

// Start starts consensus service.
func (s *Service) Start() error {
	utils.Logger().Info().Msg("[consensus/service] Starting consensus service.")
	s.stopChan = make(chan struct{})
	s.stoppedChan = make(chan struct{})
	s.consensus.Start(s.blockChannel, s.stopChan, s.stoppedChan, s.startChan)
	s.consensus.WaitForNewRandomness()
	return nil
}

// Stop stops consensus service.
func (s *Service) Stop() error {
	utils.Logger().Info().Msg("Stopping consensus service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.Logger().Info().Msg("Consensus service stopped.")
	return s.consensus.Close()
}

// APIs for the services.
func (s *Service) APIs() []rpc.API {
	return nil
}
