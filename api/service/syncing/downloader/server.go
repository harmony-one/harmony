package downloader

import (
	"context"
	"log"
	"net"

	pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"google.golang.org/grpc"
)

// Constants for downloader server.
const (
	DefaultDownloadPort = "6666"
)

// Server is the Server struct for downloader package.
type Server struct {
	downloadInterface DownloadInterface
	GrpcServer        *grpc.Server
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
	addr := net.JoinHostPort("", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterDownloaderServer(grpcServer, s)
	go grpcServer.Serve(lis)
	s.GrpcServer = grpcServer

	return grpcServer, nil
}

// NewServer creates new Server which implements DownloadInterface.
func NewServer(dlInterface DownloadInterface) *Server {
	s := &Server{downloadInterface: dlInterface}
	return s
}
