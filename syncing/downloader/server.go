package downloader

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	pb "github.com/harmony-one/harmony/syncing/downloader/proto"
)

// Server ...
type Server struct {
	downloadInterface DownloadInterface
}

// Query returns the feature at the given point.
func (s *Server) Query(ctx context.Context, request *pb.DownloaderRequest) (*pb.DownloaderResponse, error) {
	response, err := s.downloadInterface.CalculateResponse(request)
	if err != nil {
		return nil, err
	}
	// response := pb.DownloaderResponse{}
	// response.Payload = [][]byte{{0, 0, 2}}
	return response, nil
}

// Start ...
func (s *Server) Start(ip, port string) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%s", ip, port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterDownloaderServer(grpcServer, s)
	go grpcServer.Serve(lis)
	return grpcServer, nil
}

// NewServer ...
func NewServer(dlInterface DownloadInterface) *Server {
	s := &Server{downloadInterface: dlInterface}
	return s
}
