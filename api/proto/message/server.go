package message

// This client service will replace the other client service.
// This client service will use unified Message.
// TODO(minhdoan): Refactor and clean up the other client service.
import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
)

// Constants for client service port.
const (
	IP   = "127.0.0.1"
	Port = "30000"
)

// Server is the Server struct for client service package.
type Server struct {
	server                          *grpc.Server
	CreateTransactionForEnterMethod func(int64, string) error
	GetResult                       func() ([]string, []uint64)
}

// Process processes the Message and returns Response
func (s *Server) Process(ctx context.Context, message *Message) (*Response, error) {
	if message.GetType() != MessageType_LOTTERY_REQUEST {
		return &Response{}, ErrWrongMessage
	}
	lotteryRequest := message.GetLotteryRequest()
	if lotteryRequest.GetType() == LotteryRequest_ENTER {
		if s.CreateTransactionForEnterMethod == nil {
			return nil, ErrEnterProcessorNotReady
		}
		amount := lotteryRequest.Amount
		priKey := lotteryRequest.PrivateKey
		if err := s.CreateTransactionForEnterMethod(amount, priKey); err != nil {
			return nil, ErrEnterMethod
		}
		return &Response{}, nil

	} else if lotteryRequest.GetType() == LotteryRequest_RESULT {
		// if err := s.GetResult(); err != nil {
		// 	return &Response{}, ErrResultMethod
		// } else {
		// 	return &Response{}, nil
		// }
	}
	return &Response{}, nil
}

// Start starts the Server on given ip and port.
func (s *Server) Start() (*grpc.Server, error) {
	addr := net.JoinHostPort(IP, Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	s.server = grpc.NewServer(opts...)
	RegisterClientServiceServer(s.server, s)
	go s.server.Serve(lis)
	return s.server, nil
}

// Stop stops the server.
func (s *Server) Stop() {
	s.server.Stop()
}

// NewServer creates new Server which implements ClientServiceServer interface.
func NewServer(
	CreateTransactionForEnterMethod func(int64, string) error,
	GetResult func() ([]string, []uint64)) *Server {
	return &Server{
		CreateTransactionForEnterMethod: CreateTransactionForEnterMethod,
		GetResult:                       GetResult,
	}
}
