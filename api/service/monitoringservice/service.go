package monitoringservice

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	"google.golang.org/grpc"
)

// Constants for monitoring service.
const (
	ConnectionsNumberPush 			int = 0
	BlockHeightPush 				int = 1
	monitoringServicePortDifference 	= 900
	monitoringServiceHTTPPortDifference = 2000
	pushgatewayAddr 					= "http://127.0.0.1:26000"
)

// Service is the struct for monitoring service.
type Service struct {
	router      *mux.Router
	IP          string
	Port        string
	GetNodeIDs  func() []libp2p_peer.ID
	storage     *MetricsStorage
	server      *grpc.Server
	httpServer  *http.Server	
	pusher 		*push.Pusher
	messageChan chan *msg_pb.Message
}

// init vars for prometheus
var (
	metricsPush = make(chan int)
	blockHeightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "block_height",
		Help: "Get current block height.",
	})
	connectionsNumberGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "connections_number",
		Help: "Get current connections number for a node.",
	})
)

// ConnectionsLog struct for connections stats for prometheus
type ConnectionsLog struct {
	Time 			  int
	ConnectionsNumber int
}

// ConnectionsStatsHTTP struct for returning all connections logs
type ConnectionsStatsHTTP struct {
	ConnectionsLogs []ConnectionsLog
}

// New returns monitoring service.
func New(selfPeer *p2p.Peer, GetNodeIDs func() []libp2p_peer.ID) *Service {

	blockHeightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "block_height_" + selfPeer.PeerID.String(),
		Help: "Get current block height.",
	})

	connectionsNumberGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "connections_number_" + selfPeer.PeerID.String(),
		Help: "Get current connections number for a node.",
	})
	return &Service{
		IP:         selfPeer.IP,
		Port:       selfPeer.Port,
		GetNodeIDs: GetNodeIDs,
	}
}

// StartService starts monitoring service.
func (s *Service) StartService() {
	utils.Logger().Info().Msg("Starting monitoring service.")
	s.Run()
}

// StopService shutdowns monitoring service.
func (s *Service) StopService() {
	utils.Logger().Info().Msg("Shutting down monitoring service.")
	metricsPush <- -1
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
func (s *Service) Run() {
	// Init address.
	addr := net.JoinHostPort("", GetMonitoringServiceHTTPPort(s.Port))

	registry := prometheus.NewRegistry()
	registry.MustRegister(blockHeightGauge, connectionsNumberGauge)

	s.pusher = push.New(pushgatewayAddr, "metrics").Gatherer(registry)

	go s.PushMetrics()

	//s.router.Path("/connectionsstats").Queries("since", "{[0-9]*?}", "until", "{[0-9]*?}").HandlerFunc(s.GetConnectionsStats).Methods("GET")
	utils.Logger().Info().Str("port", GetMonitoringServiceHTTPPort(s.Port)).Msg("Listening")
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(addr, nil); err != nil {
			utils.Logger().Warn().Err(err).Msg("http.ListenAndServe()")
		}
	}()
	return
}

// UpdateBlockHeight updates block height.
func UpdateBlockHeight(blockHeight uint64, blockTime int64) {
	blockHeightGauge.Set(float64(blockHeight))
	metricsPush <- ConnectionsNumberPush
}

// UpdateConnectionsNumber updates connections number.
func UpdateConnectionsNumber(connectionsNumber int) {
	connectionsNumberGauge.Set(float64(connectionsNumber))
	metricsPush <- BlockHeightPush
}

// PushMetrics pushes metrics updates to prometheus pushgateway.
func (s *Service) PushMetrics() {
	for metricType := range metricsPush {
		if metricType == -1 {
			break
		}
		if err := s.pusher.Add(); err != nil {
			fmt.Println("Could not push to Pushgateway:", err)
		}
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

// Run is to run http serving monitoring service.
/*
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
}*/

// Run is to run serving monitoring service.
/*func (s *Service) Run(remove bool) (*grpc.Server, error) {
	s.storage = utils.GetMetricsStorageInstance(s.IP, s.Port, remove)
	port, err := strconv.Atoi(s.Port)
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
}*/

// For http/rpc
// GetConnectionsLog serves end-point /ConnectionsLog
/*func (s *Service) GetConnectionsStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	since := r.FormValue("since")
	until := r.FormValue("until")

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


	connectionsNumberMetric := s.storage.
	if err = json.NewEncoder(w).Encode(connectionsLogs); err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot JSON-encode connections number metric")
	}

	return
}*/
