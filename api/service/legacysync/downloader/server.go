package downloader

import (
	"context"
	"net"
	"strconv"

	pb "github.com/harmony-one/harmony/api/service/legacysync/downloader/proto"
	"github.com/harmony-one/harmony/internal/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// Constants for downloader server.
const (
	DefaultDownloadPort = "6666"
)

// Server is the Server struct for downloader package.
type Server struct {
	pb.UnimplementedDownloaderServer
	downloadInterface DownloadInterface
	GrpcServer        *grpc.Server
	Port              int
}

// Query returns the feature at the given point.
func (s *Server) Query(ctx context.Context, request *pb.DownloaderRequest) (*pb.DownloaderResponse, error) {
	var pinfo string
	// retrieve ip/port information; used for debug only
	p, ok := peer.FromContext(ctx)
	if !ok {
		pinfo = ""
	} else {
		pinfo = p.Addr.String()
	}
	response, err := s.downloadInterface.CalculateResponse(request, pinfo)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// Start starts the Server on given ip and port.
func (s *Server) Start() (*grpc.Server, error) {
	addr := net.JoinHostPort("", strconv.Itoa(s.Port))
	lis, err := net.Listen("tcp4", addr)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[SYNC] failed to start sync server")
		return nil, err
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterDownloaderServer(grpcServer, s)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			utils.Logger().Warn().Err(err).Msg("[SYNC] (*grpc.Server).Serve failed")
		}
	}()

	s.GrpcServer = grpcServer

	return grpcServer, nil
}

// NewServer creates new Server which implements DownloadInterface.
func NewServer(dlInterface DownloadInterface, port int) *Server {
	s := &Server{downloadInterface: dlInterface, Port: port}
	return s
}
