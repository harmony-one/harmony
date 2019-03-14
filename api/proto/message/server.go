package message

// This client service will replace the other client service.
// This client service will use unified Message.
// TODO(minhdoan): Refactor and clean up the other client service.
import (
	context "context"
	"log"
	"net"

	"google.golang.org/grpc"
)

// Constants for client service port.
const (
	Port = "30000"
)

// Server is the Server struct for client service package.
type Server struct {
	Port   string
	server *grpc.Server
}

func (s *Server) Process(ctx context.Context, message *Message) (*Response, error) {
	return &Response{}, nil
}

// Start starts the Server on given ip and port.
func (s *Server) Start(ip, port string) (*grpc.Server, error) {
	addr := net.JoinHostPort("", s.Port)
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

// Start starts the Server on given ip and port.
func (s *Server) Stop() {
	s.server.Stop()
}

// New creates new Server which implements ClientServiceServer interface.
func NewServer() *Server {
	return &Server{Port: Port}
}
