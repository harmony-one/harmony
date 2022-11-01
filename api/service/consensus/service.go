package consensus

import (
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/utils"
)

// Service is the consensus service.
type Service struct {
	consensus   *consensus.Consensus
	stopChan    chan struct{}
	messageChan chan *msg_pb.Message
}

// New returns consensus service.
func New(consensus *consensus.Consensus) *Service {
	return &Service{consensus: consensus}
}

// Start starts consensus service.
func (s *Service) Start() error {
	utils.Logger().Info().Msg("[consensus/service] Starting consensus service.")
	s.stopChan = make(chan struct{})
	s.consensus.Start(s.stopChan)
	s.consensus.WaitForNewRandomness()
	return nil
}

// Stop stops consensus service.
func (s *Service) Stop() error {
	utils.Logger().Info().Msg("Stopping consensus service.")
	close(s.stopChan)
	utils.Logger().Info().Msg("Consensus service stopped.")
	return nil
}
