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
	"github.com/harmony-one/harmony/p2p"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/rs/zerolog/log"
)

// Constants for metrics service.
const (
	ConnectionsNumberPush        int = 0
	BlockHeightPush              int = 1
	NodeBalancePush              int = 2
	LastConsensusPush            int = 3
	BlockRewardPush              int = 4
	metricsServicePortDifference     = 2000
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
	pusher          *push.Pusher
	messageChan     chan *msg_pb.Message
}

// init vars for prometheus
var (
	curBlockHeight     = uint64(0)
	curBalance         = big.NewInt(0)
	metricsPush        = make(chan int)
	blockHeightCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "block_height",
		Help: "Get current block height.",
	})
	blocksAcceptedGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "blocks_accepted",
		Help: "Get accepted blocks.",
	})
	connectionsNumberGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "connections_number",
		Help: "Get current connections number for a node.",
	})
	nodeBalanceCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "node_balance",
		Help: "Get current node balance.",
	})
	lastConsensusGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "last_consensus",
		Help: "Get last consensus time.",
	})
	blockRewardGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "block_reward",
		Help: "Get last block reward.",
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
	log.Info().Msg("Starting metrics service.")
	s.Run()
}

// StopService shutdowns metrics service.
func (s *Service) StopService() {
	log.Info().Msg("Shutting down metrics service.")
	metricsPush <- -1
}

// GetMetricsServicePort returns the port serving metrics service dashboard. This port is metricsServicePortDifference less than the node port.
func GetMetricsServicePort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-metricsServicePortDifference)
	}
	log.Error().Msg("Error on parsing.")
	return ""
}

// Run is to run http serving metrics service.
func (s *Service) Run() {
	// Init local storage for metrics.
	s.storage = GetStorageInstance(s.IP, s.Port, false)
	// Init address.
	addr := net.JoinHostPort("", GetMetricsServicePort(s.Port))

	registry := prometheus.NewRegistry()
	registry.MustRegister(blockHeightCounter, connectionsNumberGauge, nodeBalanceCounter, lastConsensusGauge, blockRewardGauge, blocksAcceptedGauge)

	s.pusher = push.New("http://"+s.PushgatewayIP+":"+s.PushgatewayPort, "node_metrics").Gatherer(registry).Grouping("instance", s.IP+":"+s.Port).Grouping("bls_key", s.BlsPublicKey)
	go s.PushMetrics()

	//s.router.Path("/connectionsstats").Queries("since", "{[0-9]*?}", "until", "{[0-9]*?}").HandlerFunc(s.GetConnectionsStats).Methods("GET")
	log.Info().Str("port", GetMetricsServicePort(s.Port)).Msg("Listening.")
	go func() {
		http.Handle("/node_metrics", promhttp.Handler())
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Warn().Err(err).Msg("http.ListenAndServe()")
		}
	}()
	return
}

// UpdateBlockHeight updates block height.
func UpdateBlockHeight(blockHeight uint64) {
	blockHeightCounter.Add(float64(blockHeight) - float64(curBlockHeight))
	blocksAcceptedGauge.Set(float64(blockHeight) - float64(curBlockHeight))
	s.storage.Dump(blockHeight, BlockHeightPrefix)
	curBlockHeight = blockHeight
	metricsPush <- BlockHeightPush
}

// UpdateNodeBalance updates node balance.
func UpdateNodeBalance(balance *big.Int) {
	nodeBalanceCounter.Add(float64(balance.Uint64()) - float64(curBalance.Uint64()))
	s.storage.Dump(balance, NodeBalancePrefix)
	curBalance = balance
	metricsPush <- NodeBalancePush
}

// UpdateBlockReward updates block reward.
func UpdateBlockReward(blockReward *big.Int) {
	blockRewardGauge.Set(float64(blockReward.Uint64()))
	s.storage.Dump(blockReward, BlockRewardPrefix)
	metricsPush <- BlockRewardPush
}

// UpdateLastConsensus updates last consensus time.
func UpdateLastConsensus(consensusTime int64) {
	lastConsensusGauge.Set(float64(consensusTime))
	s.storage.Dump(consensusTime, ConsensusTimePrefix)
	metricsPush <- LastConsensusPush
}

// UpdateConnectionsNumber updates connections number.
func UpdateConnectionsNumber(connectionsNumber int) {
	connectionsNumberGauge.Set(float64(connectionsNumber))
	s.storage.Dump(connectionsNumber, ConnectionsNumberPrefix)
	metricsPush <- ConnectionsNumberPush
}

// PushMetrics pushes metrics updates to prometheus pushgateway.
func (s *Service) PushMetrics() {
	for metricType := range metricsPush {
		if metricType == -1 {
			break
		}
		if err := s.pusher.Add(); err != nil {
			log.Error().Err(err).Msg("Could not push to a prometheus pushgateway.")
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
