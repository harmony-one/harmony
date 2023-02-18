package message

import (
	"context"
	"log"
	"math/big"
	"net"

	"github.com/harmony-one/harmony/internal/utils"
	"google.golang.org/grpc"
)

// Constants for client service port.
const (
	IP   = "127.0.0.1"
	Port = "30000"
)

// Server is the Server struct for client service package.
type Server struct {
	UnimplementedClientServiceServer
	server                          *grpc.Server
	CreateTransactionForEnterMethod func(int64, string) error
	GetResult                       func(string) ([]string, []*big.Int)
	CreateTransactionForPickWinner  func() error
}

// Process processes the Message and returns Response
func (s *Server) Process(ctx context.Context, message *Message) (*Response, error) {
	return &Response{}, nil
}

// Start starts the Server on given ip and port.
func (s *Server) Start() (*grpc.Server, error) {
	addr := net.JoinHostPort(IP, Port)
	lis, err := net.Listen("tcp4", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s.server = grpc.NewServer()
	RegisterClientServiceServer(s.server, s)
	go func() {
		if err := s.server.Serve(lis); err != nil {
			utils.Logger().Warn().Err(err).Msg("server.Serve() failed")
		}
	}()
	return s.server, nil
}

// Stop stops the server.
func (s *Server) Stop() {
	s.server.Stop()
}

// NewServer creates new Server which implements ClientServiceServer interface.
func NewServer(
	CreateTransactionForEnterMethod func(int64, string) error,
	GetResult func(string) ([]string, []*big.Int),
	CreateTransactionForPickWinner func() error) *Server {
	return &Server{
		CreateTransactionForEnterMethod: CreateTransactionForEnterMethod,
		CreateTransactionForPickWinner:  CreateTransactionForPickWinner,
		GetResult:                       GetResult,
	}
}
