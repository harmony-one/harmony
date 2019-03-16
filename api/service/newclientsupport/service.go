package newclientsupport

import (
	"math/big"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
)

// Service is the client support service.
type Service struct {
	server      *msg_pb.Server
	messageChan chan *msg_pb.Message
}

// New returns new client support service.
func New(
	CreateTransactionForEnterMethod func(int64, string) error,
	GetResult func(string) ([]string, []*big.Int),
	CreateTransactionForPickWinner func() error,
) *Service {
	return &Service{
		server: msg_pb.NewServer(CreateTransactionForEnterMethod, GetResult, CreateTransactionForPickWinner),
	}
}

// StartService starts client support service.
func (s *Service) StartService() {
	s.server.Start()
}

// StopService stops client support service.
func (s *Service) StopService() {
	s.server.Stop()
}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {
	return
}
