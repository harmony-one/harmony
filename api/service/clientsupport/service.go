package clientsupport

import (
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	clientService "github.com/harmony-one/harmony/api/client/service"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/state"
	"google.golang.org/grpc"
)

const (
	// ClientServicePortDiff is the positive port diff for client service
	ClientServicePortDiff = 5555
)

// Service is the client support service.
type Service struct {
	server      *clientService.Server
	grpcServer  *grpc.Server
	ip          string
	port        string
	messageChan chan *msg_pb.Message
}

// New returns new client support service.
func New(stateReader func() (*state.DB, error),
	callFaucetContract func(common.Address) common.Hash,
	getDeployedStakingContract func() common.Address,
	ip, nodePort string) *Service {
	port, _ := strconv.Atoi(nodePort)
	return &Service{
		server: clientService.NewServer(stateReader, callFaucetContract, getDeployedStakingContract),
		ip:     ip,
		port:   strconv.Itoa(port + ClientServicePortDiff)}
}

// StartService starts client support service.
func (s *Service) StartService() {
	s.grpcServer, _ = s.server.Start(s.ip, s.port)
}

// StopService stops client support service.
func (s *Service) StopService() {
	s.grpcServer.Stop()
}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// NotifyService notify service
func (sc *Service) NotifyService(params map[string]interface{}) {
	return
}
