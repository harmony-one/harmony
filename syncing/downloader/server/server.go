package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"

	pb "github.com/harmony-one/harmony/syncing/downloader/proto"
)

type downloaderServer struct {
}

// GetFeature returns the feature at the given point.
func (s *downloaderServer) Query(ctx context.Context, request *pb.DownloaderRequest) (*pb.DownloaderResponse, error) {
	response := pb.DownloaderResponse{}
	response.Payload = [][]byte{{0, 0, 2}}
	return &response, nil
}

func newServer() *downloaderServer {
	s := &downloaderServer{}
	return s
}

func main() {
	lis, err := net.Listen("tcp", "localhost:9999")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterDownloaderServer(grpcServer, newServer())
	grpcServer.Serve(lis)
}
