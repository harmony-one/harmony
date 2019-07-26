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
	CreateTransactionForPickWinner  func() error
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

		key, err := crypto.HexToECDSA(priKey)
		if err != nil {
			utils.Logger().Error().Msg("Error when HexToECDSA")
		}
		address := crypto.PubkeyToAddress(key.PublicKey)

		utils.Logger().Info().Int64("amount", amount).Bytes("address", address[:]).Msg("Enter")
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
		utils.Logger().Info().Strs("players", players).Strs("balances", stringBalances).Msg("getPlayers")
		ret := &Response{
			Response: &Response_LotteryResponse{
				LotteryResponse: &LotteryResponse{
					Players:  players,
					Balances: stringBalances,
				},
			},
		}
		return ret, nil
	} else if lotteryRequest.GetType() == LotteryRequest_PICK_WINNER {
		if s.CreateTransactionForPickWinner() != nil {
			return nil, ErrWhenPickingWinner
		}
		return &Response{}, nil
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
