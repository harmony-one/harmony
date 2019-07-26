package monitoringservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
	"strconv"
	"net/http"


	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"github.com/ethereum/go-ethereum/rpc"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// Constants for monitoring service.
const (
	monitoringServicePortDifference = 900
	monitoringServiceHTTPPortDifference = 2000
)

// Service is the struct for monitoring service.
type Service struct {
	router            *mux.Router
	IP                string
	Port              string
	GetNodeIDs        func() []libp2p_peer.ID
	storage           *utils.MetricsStorage
	server            *grpc.Server
	httpServer        *http.Server
	messageChan       chan *msg_pb.Message
}

type ConnectionsLog struct {
	Time int
	ConnectionsNumber int
}

type ConnectionsStatsHTTP struct {
	ConnectionsLogs []ConnectionsLog
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
	utils.Logger().Info().Msg("Starting monitoring service.")
	s.Run(true)
	s.httpServer = s.RunHTTP()
}

// StopService shutdowns monitoring service.
func (s *Service) StopService() {
	utils.Logger().Info().Msg("Shutting down monitoring service.")
	s.server.Stop()
}

// GetMonitoringServicePort returns the port serving monitorign service dashboard. This port is monitoringServicePortDifference less than the node port.
func GetMonitoringServicePort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-monitoringServicePortDifference)
	}
	utils.Logger().Error().Msg("error on parsing.")
	return ""
}

// GetMonitoringServiceHTTPPort returns the port serving monitorign service dashboard. This port is monitoringServicePortDifference less than the node port.
func GetMonitoringServiceHTTPPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-monitoringServiceHTTPPortDifference)
	}
	utils.Logger().Error().Msg("error on parsing.")
	return ""
}

// Run is to run http serving monitoring service.
func (s *Service) RunHTTP() *http.Server {
	// Init address.
	addr := net.JoinHostPort("", GetMonitoringServiceHTTPPort(s.Port))

	s.router = mux.NewRouter()
	// Set up router for connections stats.
	s.router.Path("/connectionsstats").Queries("since", "{[0-9]*?}", "until", "{[0-9]*?}").HandlerFunc(s.GetConnectionsStats).Methods("GET")
	s.router.Path("/connectionsstats").HandlerFunc(s.GetConnectionsStats)

	// Do serving now.
	utils.Logger().Info().Str("port", GetMonitoringServiceHTTPPort(s.Port)).Msg("Listening")
	httpServer := &http.Server{Addr: addr, Handler: s.router}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			utils.Logger().Warn().Err(err).Msg("httpServer.ListenAndServe()")
		}
	}()
	return httpServer
}



// Run is to run serving monitoring service.
func (s *Service) Run(remove bool) (*grpc.Server, error) {
	s.storage = utils.GetMetricsStorageInstance(s.IP, s.Port, remove)
	port, err := strconv.Atoi(s.Port); 
	if err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(s.IP, strconv.Itoa(port-monitoringServicePortDifference))
	lis, err := net.Listen("tcp", addr)
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



// Process processes the Request and returns Response
func (s *Service) Process(ctx context.Context, request *Request) (*Response, error) {
	if request.GetMetricsType() != MetricsType_CONNECTIONS_STATS {
		return &Response{}, nil
	}
	ConnectionsStatsRequest := request.GetConnectionsStatsRequest()
	since := int(ConnectionsStatsRequest.GetSince())
	until := int(ConnectionsStatsRequest.GetUntil())

	connectionsNumbers := s.storage.ReadConnectionsNumbersFromDB(since, until)
	parsedConnectionsStats := []*ConnectionsStats{}
	for currentTime, connectionsNumber := range connectionsNumbers {
		parsedConnectionsStats = append(parsedConnectionsStats, &ConnectionsStats{Time: int32(currentTime), ConnectionsNumber: int32(connectionsNumber)})
	}

	ret := &Response{
		MetricsType: MetricsType_CONNECTIONS_STATS,
		Response: &Response_ConnectionsStatsResponse{
			ConnectionsStatsResponse: &ConnectionsStatsResponse{
				ConnectionsStats: parsedConnectionsStats,
			},
		},
	}
	return ret, nil
}


// For http/rpc
// GetConnectionsLog serves end-point /ConnectionsLog
func (s *Service) GetConnectionsStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	since := r.FormValue("since")
	until := r.FormValue("until")

	connectionsLogs := []*ConnectionsLog{}
	defer func() {
		if err := json.NewEncoder(w).Encode(connectionsLogs); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot JSON-encode connections logs")
		}
	}()
	var err error
	sinceInt := 0
	if (since != "") {
		sinceInt, err = strconv.Atoi(since)
	}
	if err != nil {
		utils.Logger().Warn().Err(err).Str("since", since).Msg("invalid since parameter")
		return
	}
	untilInt := int(time.Now().Unix())
	if until != "" {
		untilInt, err = strconv.Atoi(until)
	}
	if err != nil {
		utils.Logger().Warn().Err(err).Str("until", until).Msg("invalid until parameter")
		return
	}

	connectionsNumbers := s.storage.ReadConnectionsNumbersFromDB(sinceInt, untilInt)
	for currentTime, connectionsNumber := range connectionsNumbers {
		connectionsLogs = append(connectionsLogs, &ConnectionsLog{Time: currentTime, ConnectionsNumber: connectionsNumber})
	}

	return
}

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
