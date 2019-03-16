package message

// This client service will replace the other client service.
// This client service will use unified Message.
// TODO(minhdoan): Refactor and clean up the other client service.
import (
	"context"
	"log"
	"math/big"
	"net"

	"github.com/ethereum/go-ethereum/crypto"
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
	server                          *grpc.Server
	CreateTransactionForEnterMethod func(int64, string) error
	GetResult                       func(string) ([]string, []*big.Int)
}

// Process processes the Message and returns Response
func (s *Server) Process(ctx context.Context, message *Message) (*Response, error) {
	if message.GetType() != MessageType_LOTTERY_REQUEST {
		return &Response{}, ErrWrongMessage
	}
	utils.GetLogInstance().Info("Recieved:", "message", message)
	lotteryRequest := message.GetLotteryRequest()
	if lotteryRequest.GetType() == LotteryRequest_ENTER {
		if s.CreateTransactionForEnterMethod == nil {
			return nil, ErrEnterProcessorNotReady
		}
		amount := lotteryRequest.Amount
		priKey := lotteryRequest.PrivateKey

		key, err := crypto.HexToECDSA(priKey)
		if err != nil {
			utils.GetLogInstance().Error("Error when HexToECDSA")
		}
		address := crypto.PubkeyToAddress(key.PublicKey)

		utils.GetLogInstance().Info("Enter:", "amount", amount, "for address", address)
		if err := s.CreateTransactionForEnterMethod(amount, priKey); err != nil {
			return nil, ErrEnterMethod
		}
		return &Response{}, nil
	} else if lotteryRequest.GetType() == LotteryRequest_RESULT {
		players, balances := s.GetResult(lotteryRequest.PrivateKey)
		stringBalances := []string{}
		for _, balance := range balances {
			stringBalances = append(stringBalances, balance.String())
		}
		utils.GetLogInstance().Info("getPlayers", "players", players, "balances", stringBalances)
		ret := &Response{
			Response: &Response_LotteryResponse{
				LotteryResponse: &LotteryResponse{
					Players:  players,
					Balances: stringBalances,
				},
			},
		}
		return ret, nil
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
	GetResult func(string) ([]string, []*big.Int)) *Server {
	return &Server{
		CreateTransactionForEnterMethod: CreateTransactionForEnterMethod,
		GetResult:                       GetResult,
	}
}
