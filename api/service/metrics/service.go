package metrics

import (
	"fmt"
	"math/big"
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

// Constants for metrics service.
const (
	ConnectionsNumberPush            int = 0
	BlockHeightPush                  int = 1
	NodeBalancePush                  int = 2
	LastConsensusPush                int = 3
	BlockRewardPush                  int = 4
	metricsServicePortDifference         = 900
	metricsServiceHTTPPortDifference     = 2000
	defaultPushgatewayAddr               = "http://127.0.0.1:26000"
)

// Service is the struct for metrics service.
type Service struct {
	router          *mux.Router
	BlsPublicKey    string
	IP              string
	Port            string
	PushgatewayIP   string
	PushgatewayPort string
	GetNodeIDs      func() []libp2p_peer.ID
	storage         *Storage
	server          *grpc.Server
	httpServer      *http.Server
	pusher          *push.Pusher
	messageChan     chan *msg_pb.Message
}

// init vars for prometheus
var (
	curBlockHeight     = uint64(0)
	metricsPush        = make(chan int)
	blockHeightCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "block_height",
		Help: "Get current block height.",
	})
	blockHeightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "block_height",
		Help: "Get current block height.",
	})
	connectionsNumberGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "connections_number",
		Help: "Get current connections number for a node.",
	})
	nodeBalanceGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "node_balance",
		Help: "Get current node balance",
	})
	lastConsensusGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "last_consensus",
		Help: "Get last consensus time",
	})
	blockRewardGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "block_reward",
		Help: "Get last block reward",
	})
)

// ConnectionsLog struct for connections stats for prometheus
type ConnectionsLog struct {
	Time              int
	ConnectionsNumber int
}

// ConnectionsStatsHTTP struct for returning all connections logs
type ConnectionsStatsHTTP struct {
	ConnectionsLogs []ConnectionsLog
}

// New returns metrics service.
func New(selfPeer *p2p.Peer, blsPublicKey, pushgatewayIP, pushgatewayPort string, GetNodeIDs func() []libp2p_peer.ID) *Service {
	blockHeightCounter = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "block_height_" + blsPublicKey,
		Help: "Get current block height.",
	})
	blockHeightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "block_height_" + blsPublicKey,
		Help: "Get current block height.",
	})
	connectionsNumberGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "connections_number_" + blsPublicKey,
		Help: "Get current connections number for a node.",
	})
	nodeBalanceGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "node_balance_" + blsPublicKey,
		Help: "Get current node balance",
	})
	lastConsensusGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "last_consensus_" + blsPublicKey,
		Help: "Get last consensus time",
	})
	blockRewardGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "block_reward_" + blsPublicKey,
		Help: "Get last block reward",
	})
	return &Service{
		BlsPublicKey:    blsPublicKey,
		IP:              selfPeer.IP,
		Port:            selfPeer.Port,
		PushgatewayIP:   pushgatewayIP,
		PushgatewayPort: pushgatewayPort,
		GetNodeIDs:      GetNodeIDs,
	}
}

// StartService starts metrics service.
func (s *Service) StartService() {
	utils.Logger().Info().Msg("Starting metrics service.")
	s.Run()
}

// StopService shutdowns metrics service.
func (s *Service) StopService() {
	utils.Logger().Info().Msg("Shutting down metrics service.")
	metricsPush <- -1
	s.server.Stop()
}

// GetMetricsServicePort returns the port serving metrics service dashboard. This port is metricsServicePortDifference less than the node port.
func GetMetricsServicePort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-metricsServicePortDifference)
	}
	utils.Logger().Error().Msg("error on parsing.")
	return ""
}

// GetMetricsServiceHTTPPort returns the port serving metrics service dashboard. This port is metricsServicePortDifference less than the node port.
func GetMetricsServiceHTTPPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-metricsServiceHTTPPortDifference)
	}
	utils.Logger().Error().Msg("error on parsing.")
	return ""
}

// Run is to run http serving metrics service.
func (s *Service) Run() {
	// Init local storage for metrics.
	//s.storage = GetStorageInstance(s.IP, s.Port, false)
	// Init address.
	addr := net.JoinHostPort("", GetMetricsServiceHTTPPort(s.Port))

	registry := prometheus.NewRegistry()
	registry.MustRegister(blockHeightCounter, connectionsNumberGauge, nodeBalanceGauge, lastConsensusGauge, blockRewardGauge, blockHeightGauge)

	s.pusher = push.New("http://"+s.PushgatewayIP+":"+s.PushgatewayPort, "metrics").Gatherer(registry)

	go s.PushMetrics()

	//s.router.Path("/connectionsstats").Queries("since", "{[0-9]*?}", "until", "{[0-9]*?}").HandlerFunc(s.GetConnectionsStats).Methods("GET")
	utils.Logger().Info().Str("port", GetMetricsServiceHTTPPort(s.Port)).Msg("Listening")
	go func() {
		http.Handle("/node_metrics", promhttp.Handler())
		if err := http.ListenAndServe(addr, nil); err != nil {
			utils.Logger().Warn().Err(err).Msg("http.ListenAndServe()")
		}
	}()
	return
}

// UpdateBlockHeight updates block height.
func UpdateBlockHeight(blockHeight uint64) {
	blockHeightCounter.Add(float64(blockHeight) - float64(curBlockHeight))
	blockHeightGauge.Set(float64(blockHeight) - float64(curBlockHeight))
	curBlockHeight = blockHeight
	metricsPush <- ConnectionsNumberPush
}

// UpdateNodeBalance updates node balance.
func UpdateNodeBalance(balance *big.Int) {
	nodeBalanceGauge.Set(float64(balance.Uint64()))
	metricsPush <- NodeBalancePush
}

// UpdateBlockReward updates block reward.
func UpdateBlockReward(blockReward *big.Int) {
	blockRewardGauge.Set(float64(blockReward.Uint64()))
	metricsPush <- BlockRewardPush
}

// UpdateLastConsensus updates last consensus time.
func UpdateLastConsensus(consensusTime int64) {
	lastConsensusGauge.Set(float64(consensusTime))
	metricsPush <- LastConsensusPush
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
