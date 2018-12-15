package client

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/state"
	"log"
	"net"

	"google.golang.org/grpc"

	proto "github.com/harmony-one/harmony/client/service/proto"
)

// Server is the Server struct for client service package.
type Server struct {
	stateReader        func() (*state.StateDB, error)
	callFaucetContract func(common.Address) common.Hash
}

// FetchAccountState implements the FetchAccountState interface to return account state.
func (s *Server) FetchAccountState(ctx context.Context, request *proto.FetchAccountStateRequest) (*proto.FetchAccountStateResponse, error) {
	var address common.Address
	address.SetBytes(request.Address)
	log.Println("Returning FetchAccountStateResponse for address: ", address.Hex())
	state, err := s.stateReader()
	if err != nil {
		return nil, err
	}
	return &proto.FetchAccountStateResponse{Balance: state.GetBalance(address).Bytes(), Nonce: state.GetNonce(address)}, nil
}

// GetFreeToken implements the GetFreeToken interface to request free token.
func (s *Server) GetFreeToken(ctx context.Context, request *proto.GetFreeTokenRequest) (*proto.GetFreeTokenResponse, error) {
	var address common.Address
	address.SetBytes(request.Address)
	log.Println("Returning GetFreeTokenResponse for address: ", address.Hex())
	return &proto.GetFreeTokenResponse{TxId: s.callFaucetContract(address).Bytes()}, nil
}

// Start starts the Server on given ip and port.
func (s *Server) Start(ip, port string) (*grpc.Server, error) {
	// TODO(minhdoan): Currently not using ip. Fix it later.
	addr := net.JoinHostPort("", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	proto.RegisterClientServiceServer(grpcServer, s)
	go grpcServer.Serve(lis)
	return grpcServer, nil
}

// NewServer creates new Server which implements ClientServiceServer interface.
func NewServer(stateReader func() (*state.StateDB, error), callFaucetContract func(common.Address) common.Hash) *Server {
	s := &Server{stateReader: stateReader, callFaucetContract: callFaucetContract}
	return s
}
