package beaconchain

import (
	"context"
	"github.com/harmony-one/harmony/proto/bcconn"
	"log"
	"net"

	"google.golang.org/grpc"

	proto "github.com/harmony-one/harmony/beaconchain/rpc/proto"
)

// Server is the Server struct for beacon chain package.
type Server struct {
	shardLeaderMap func() map[int]*bcconn.NodeInfo
}

// FetchLeaders implements the FetchLeaders interface to return current leaders.
func (s *Server) FetchLeaders(ctx context.Context, request *proto.FetchLeadersRequest) (*proto.FetchLeadersResponse, error) {
	log.Println("Returning FetchLeadersResponse")

	leaders := []*proto.FetchLeadersResponse_Leader{}
	for shardID, leader := range s.shardLeaderMap() {
		leaders = append(leaders, &proto.FetchLeadersResponse_Leader{Ip: leader.Self.IP, Port: leader.Self.Port, ShardId: uint32(shardID)})
	}
	log.Println(leaders)
	return &proto.FetchLeadersResponse{Leaders: leaders}, nil
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
	proto.RegisterBeaconChainServiceServer(grpcServer, s)
	go grpcServer.Serve(lis)
	return grpcServer, nil
}

// NewServer creates new Server which implements BeaconChainServiceServer interface.
func NewServer(shardLeaderMap func() map[int]*bcconn.NodeInfo) *Server {
	s := &Server{shardLeaderMap}
	return s
}
