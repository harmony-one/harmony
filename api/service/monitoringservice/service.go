package monitoringservice

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"

	"google.golang.org/grpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// Constants for monitoring service.
const (
	monitoringServicePortDifference = 29000
)

// Service is the struct for monitoring service.
type Service struct {
	IP                string
	Port              string
	GetNodeIDs        func() []libp2p_peer.ID
	storage           *Storage
	server            *grpc.Server
	messageChan       chan *msg_pb.Message
}

// New returns monitoring service.
func New(selfPeer *p2p.Peer, GetNodeIDs func() []libp2p_peer.ID) *Service {
	return &Service{
		IP:                selfPeer.IP,
		Port:              selfPeer.Port,
		GetNodeIDs:        GetNodeIDs,
	}
}

// StartService starts monitoring service.
func (s *Service) StartService() {
	utils.Logger().Info().Msg("Starting explorer service.")
	s.Run(true)
}

// StopService shutdowns monitoring service.
func (s *Service) StopService() {
	utils.Logger().Info().Msg("Shutting down monitoring service.")
	s.server.Stop()
}

// GetMonitoringServicePort returns the port serving monitorign service dashboard. This port is monitoringServicePortDifference less than the node port.
func GetMonitoringSerivcePort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", nodePort-monitoringServicePortDifference)
	}
	utils.Logger().Error().Msg("error on parsing.")
	return ""
}


// Run is to run serving monitoring service.
func (s *Service) Run(remove bool) (*Server, error) {
	s.storage = GetStorageInstance(s.IP, s.Port, remove)
	addr := net.JoinHostPort(s.IP, s.Port-monitoringServicePortDifference)
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

// ReadConnectionsNumberFromDB returns a list of connections numbers to server connections number end-point.
func (s *Service) ReadConnectionsNumberFromDB(since, until uint) []uint {
	connectionsNumbers := []uint
	for i := since; i <= until; i++ {
		if i < 0 {
			connectionsNumbers = append(connectionsNumbers, nil)
			continue
		}
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
	return connectionsNumbers
}


// Process processes the Request and returns Response
func (s *Service) Process(ctx context.Context, request *Request) (*Response, error) {
	if request.GetMetricsType() != MetricsType_CONNECTIONS_NUMBER {
		return &Response{}, ErrWrongMessage
	}
	connectionsNumbersRequest := request.GetConnectionsNumbersRequest()
	since := connectionsNumbersRequest.GetSince() == nil ? 0 : connectionsNumbersRequest.GetSince()
	until := connectionsNumbersRequest.GetUntil() == nil ? 0 : connectionsNumbersRequest.GetUntil()


	connectionsNumbers := s.ReadConnectionsNumbersDB(since, until)
	parsedConnectionsNumbers := []*ConnectionsNumber{}
	for currentTime, connectionsNumber := range connectionsNumbers {
		parsedConnectionsNumbers = append(parsedConnectionsNumbers, ConnectionsNumber{Time: currentTime, ConnectionsNumber: connectionsNumber})
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


// For rpc/http later
// GetMonitoringServiceConnectionsNumber serves end-point /connectionsNumber
/*func (s *Service) GetMonitoringSerivceConnectionsNumber(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	since := r.FormValue("since")
	until := r.FormValue("until")

	data := &Data{
		ConnectionsNumbers: []*ConnectionLog,
	}
	defer func() {
		if err := json.NewEncoder(w).Encode(data.Connections); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot JSON-encode connections")
		}
	}()
	var sinceInt int
	var err error
	if (since == "") {
		since = 0, err = nil
	} else {
		sinceInt, err = strconv.Atoi(since)
	}
	if err != nil {
		utils.Logger().Warn().Err(err).Str("since", since).Msg("invalid since parameter")
		return
	}
	var untilInt int
	if until == "" {
		untilInt = time.Now().Unix()
	} else {
		untilInt, err = strconv.Atoi(until)
	}
	if err != nil {
		utils.Logger().Warn().Err(err).Str("until", until).Msg("invalid until parameter")
		return
	}

	connectionsNumbers := s.ReadConnectionsNumbersDB(sinceInt, untilInt)
	for currentTime, connectionsNumber := range connectionsNumbers {
		data.ConnectionsNumbers = append(data.ConnectionsNumbers, ConnectionsLog{Time: currentTime, ConnectionsNumber: connectionsNumber})
	}

	return
}*/

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {
	return
}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// APIs for the services.
func (s *Service) APIs() []rpc.API {
	return nil
}
