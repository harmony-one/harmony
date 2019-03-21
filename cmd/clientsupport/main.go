package main

import (
	"fmt"
	"math/big"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
)

// Service is the client support service.
type Service struct {
	server      *msg_pb.Server
	messageChan chan *msg_pb.Message
}

// NewServer --
func NewServer(
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

// CreateTransactionForEnterMethod --
func CreateTransactionForEnterMethod(int64, string) error {
	fmt.Println("created transaction")
	return nil
}

// GetResult --
func GetResult(string) ([]string, []*big.Int) {
	return []string{"12340", "12341", "12342", "12343"}, []*big.Int{big.NewInt(1), big.NewInt(1), big.NewInt(1), big.NewInt(1)}
}

// CreateTransactionForPickWinner --
func CreateTransactionForPickWinner() error {
	fmt.Println("winner picked")
	return nil
}

func main() {
	// NewServer --
	s := NewServer(CreateTransactionForEnterMethod, GetResult, CreateTransactionForPickWinner)
	s.StartService()
	fmt.Println("Server started")
	select {}
}
