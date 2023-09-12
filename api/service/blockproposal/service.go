package blockproposal

import (
	msg_pb "github.com/servprotocolorg/harmony/api/proto/message"
	"github.com/servprotocolorg/harmony/consensus"
	"github.com/servprotocolorg/harmony/internal/utils"
)

// Service is a block proposal service.
type Service struct {
	stopChan              chan struct{}
	stoppedChan           chan struct{}
	readySignal           chan consensus.ProposalType
	commitSigsChan        chan []byte
	messageChan           chan *msg_pb.Message
	waitForConsensusReady func(readySignal chan consensus.ProposalType, commitSigsChan chan []byte, stopChan chan struct{}, stoppedChan chan struct{})
}

// New returns a block proposal service.
func New(readySignal chan consensus.ProposalType, commitSigsChan chan []byte, waitForConsensusReady func(readySignal chan consensus.ProposalType, commitSigsChan chan []byte, stopChan chan struct{}, stoppedChan chan struct{})) *Service {
	return &Service{
		readySignal:           readySignal,
		commitSigsChan:        commitSigsChan,
		waitForConsensusReady: waitForConsensusReady,
		stopChan:              make(chan struct{}),
		stoppedChan:           make(chan struct{}),
	}
}

// Start starts block proposal service.
func (s *Service) Start() error {
	s.run()
	return nil
}

func (s *Service) run() {
	s.waitForConsensusReady(s.readySignal, s.commitSigsChan, s.stopChan, s.stoppedChan)
}

// Stop stops block proposal service.
func (s *Service) Stop() error {
	utils.Logger().Info().Msg("Stopping block proposal service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.Logger().Info().Msg("Role conversion stopped.")
	return nil
}
