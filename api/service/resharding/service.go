package resharding

import (
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constants for resharding service.
const (
	ReshardingCheckTime = time.Second
)

// Service is the role conversion service.
type Service struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
	messageChan chan *msg_pb.Message
	beaconChain *core.BlockChain
}

// New returns role conversion service.
func New(beaconChain *core.BlockChain) *Service {
	return &Service{beaconChain: beaconChain}
}

// StartService starts role conversion service.
func (s *Service) StartService() {
	s.stopChan = make(chan struct{})
	s.stoppedChan = make(chan struct{})

	s.Init()
	s.Run(s.stopChan, s.stoppedChan)
}

// Init initializes role conversion service.
func (s *Service) Init() {
}

// Run runs role conversion.
func (s *Service) Run(stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		defer close(stoppedChan)
		for {
			select {
			default:
				utils.GetLogInstance().Info("Running role conversion")
				// TODO: Write some logic here.
				s.DoService()
			case <-stopChan:
				return
			}
		}
	}()
}

// DoService does role conversion.
func (s *Service) DoService() {
	tick := time.NewTicker(ReshardingCheckTime)
	// Get current shard state hash.
	currentShardStateHash := s.beaconChain.CurrentBlock().Header().ShardStateHash
	for {
		select {
		case <-tick.C:
			LatestShardStateHash := s.beaconChain.CurrentBlock().Header().ShardStateHash
			if currentShardStateHash != LatestShardStateHash {
				// TODO(minhdoan): Add resharding logic later after modifying the resharding func as it current doesn't calculate the role (leader/validator)
			}
		}
	}
}

// StopService stops role conversion service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping role conversion service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Role conversion stopped.")
}

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {
	return
}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// APIs for the services.
func (s *Service) APIs() []rpc.API {
	return nil
}
