package downloader

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	pb "github.com/harmony-one/harmony/syncing/downloader/proto"
)

const (
	DefaultDownloadPort = "6666"
)

// Server is the Server struct for downloader package.
type Server struct {
	downloadInterface DownloadInterface
}

// Query returns the feature at the given point.
func (s *Server) Query(ctx context.Context, request *pb.DownloaderRequest) (*pb.DownloaderResponse, error) {
	response, err := s.downloadInterface.CalculateResponse(request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// Start starts the Server on given ip and port.
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

// NewServer creates new Server which implements DownloadInterface.
func NewServer(dlInterface DownloadInterface) *Server {
	s := &Server{downloadInterface: dlInterface}
	return s
}
