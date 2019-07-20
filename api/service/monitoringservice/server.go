package monitoringservice

// This client service will replace the other client service.
// This client service will use unified Request.
// TODO(minhdoan): Refactor and clean up the other client service.
import (
	"context"
	"log"
	"math/big"
	"net"
	"strconv"

	"github.com/harmony-one/harmony/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/internal/utils"

	"google.golang.org/grpc"
)

// Constants for client service port.
const (
	IP   = "127.0.0.1"
	Port = "50000"
)

// Server is the Server struct for client service package.
type Server struct {
	server                          *grpc.Server
}

func (s *Server) ReadConnectionsNumberFromDB(since, until uint) []uint {
	connectionsNumbers := []uint
	for i := since; i <= until; i++ {
		key := GetConnectionsNumberKey(i)
		data, err := storage.db.Get([]byte(key))
		if err != nil {
			connectionsNumbers = append(connectionsNumbers, nil)
			continue
		}
		connectionsNumber := uint(0)
		if rlp.DecodeBytes(data, block) != nil {
			utils.Logger().Error().Msg("Error on getting from db")
			os.Exit(1)
		}
		connectionsNumbers = append(connectionsNumbers, connectionsNumber)
	}
}


// Process processes the Request and returns Response
func (s *Server) Process(ctx context.Context, request *Request) (*Response, error) {
	if request.GetMetricsType() != MetricsType_CONNECTIONS_NUMBER {
		return &Response{}, ErrWrongMessage
	}
	connectionsNumbersRequest := request.GetConnectionsNumbersRequest()
	since := connectionsNumbersRequest.GetSince() == nil ? 0 : connectionsNumbersRequest.GetSince()
	until := connectionsNumbersRequest.GetUntil() == nil ? 0 : connectionsNumbersRequest.GetUntil()


	connectionsNumbers := s.ReadConnectionsNumbersDB(since, until)
	parsedConnectionsNumbers := []*ConnectionsNumber{}
	for currentTime, connectionsNumber := range connectionsNumbers {
		parsedConnectionsNumbers = append(data.ConnectionsNumbers, ConnectionsNumber{Time: currentTime, ConnectionsNumber: connectionsNumber})
	}

	ret := &Response{
		Response: &Response_ConnectionsNumbersResponse{
			MetricsType: MetricsType_CONNECTIONS_NUMBER,
			ConnectionsNumbersResponse: &ConnectionsNumbersResponse{
				ConnnectionsNumbers: parsedConnectionsNumbers
			},
		},
	}
	return ret, nil
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
