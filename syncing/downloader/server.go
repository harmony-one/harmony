package downloader

import (
	"context"
	"fmt"
	"net"

	"github.com/harmony-one/harmony/node"
	pb "github.com/harmony-one/harmony/syncing/downloader/proto"
	"google.golang.org/grpc"
)

// Server ...
type Server struct {
	node *node.Node
}

// Query returns the feature at the given point.
func (s *Server) Query(ctx context.Context, request *pb.DownloaderRequest) (*pb.DownloaderResponse, error) {
	res := &pb.DownloaderResponse{}
	if request.Type == pb.DownloaderRequest_HEADER {
	} else {
		res.Payload = append(res.Payload, []byte{1})
	}
	return res, nil
}

// Start ...
func (s *Server) Start(port string) error {
	if s.node == nil {
		return ErrDownloaderWithNoNode
	}
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%s", port))
	if err != nil {

	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterDownloaderServer(grpcServer, s)
	grpcServer.Serve(lis)
	return nil
}

// NewServer ...
func NewServer(node *node.Node) *Server {
	s := &Server{node: node}
	return s
}
