package blockproposal

import (
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/utils"
)

// Service is a block proposal service.
type Service struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
	c           *consensus.Proposer
	messageChan chan *msg_pb.Message
}

// New returns a block proposal service.
func New(proposer *consensus.Proposer) *Service {
	return &Service{
		c:           proposer,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
	}
}

// Start starts block proposal service.
func (s *Service) Start() error {
	s.run()
	return nil
}

func (s *Service) run() {
	s.c.WaitForConsensusReadyV2(s.stopChan, s.stoppedChan)
}

// Stop stops block proposal service.
func (s *Service) Stop() error {
	utils.Logger().Info().Msg("Stopping block proposal service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.Logger().Info().Msg("Role conversion stopped.")
	return nil
}
