package metrics

import (
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/rpc"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
)

// Constants for metrics service.
const (
	BalanceScale                 int = 18
	BalancePrecision             int = 13
	ConnectionsNumberPush        int = 0
	BlockHeightPush              int = 1
	NodeBalancePush              int = 2
	LastConsensusPush            int = 3
	BlockRewardPush              int = 4
	TxPoolPush                   int = 5
	metricsServicePortDifference     = 2000
)

// Service is the struct for metrics service.
type Service struct {
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
	curTxPoolSize        = uint64(0)
	curBlockHeight       = uint64(0)
	curBlocks            = uint64(0)
	curBalance           = big.NewInt(0)
	curConnectionsNumber = 0
	lastBlockReward      = big.NewInt(0)
	lastConsensusTime    = int64(0)
	metricsPush          = make(chan int)
	blockHeightCounter   = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "block_height",
		Help: "Get current block height.",
	})
	txPoolGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tx_pool_size",
		Help: "Get current tx pool size.",
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
	utils.Logger().Info().Msg("Starting metrics service.")
	s.Run()
}

// StopService shutdowns metrics service.
func (s *Service) StopService() {
	utils.Logger().Info().Msg("Shutting down metrics service.")
	metricsPush <- -1
}

// GetMetricsServicePort returns the port serving metrics service dashboard. This port is metricsServicePortDifference less than the node port.
func GetMetricsServicePort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-metricsServicePortDifference)
	}
	utils.Logger().Error().Msg("Error on parsing.")
	return ""
}

// Run is to run http serving metrics service.
func (s *Service) Run() {
	// Init local storage for metrics.
	s.storage = GetStorageInstance(s.IP, s.Port, true)
	// Init address.
	addr := net.JoinHostPort("", GetMetricsServicePort(s.Port))

	registry := prometheus.NewRegistry()
	registry.MustRegister(blockHeightCounter, connectionsNumberGauge, nodeBalanceCounter, lastConsensusGauge, blockRewardGauge, blocksAcceptedGauge, txPoolGauge)

	s.pusher = push.New("http://"+s.PushgatewayIP+":"+s.PushgatewayPort, "node_metrics").Gatherer(registry).Grouping("instance", s.IP+":"+s.Port).Grouping("bls_key", s.BlsPublicKey)
	go s.PushMetrics()
	// Pull metrics http server
	utils.Logger().Info().Str("port", GetMetricsServicePort(s.Port)).Msg("Listening.")
	go func() {
		http.Handle("/node_metrics", promhttp.Handler())
		if err := http.ListenAndServe(addr, nil); err != nil {
			utils.Logger().Warn().Err(err).Msg("http.ListenAndServe()")
		}
	}()
	return
}

// FormatBalance formats big.Int balance with precision.
func FormatBalance(balance *big.Int) float64 {
	stringBalance := balance.String()
	if len(stringBalance) < BalanceScale {
		return 0.0
	}
	if len(stringBalance) == BalanceScale {
		stringBalance = "0." + stringBalance[len(stringBalance)-BalanceScale:len(stringBalance)-BalancePrecision]
	} else {
		stringBalance = stringBalance[:len(stringBalance)-BalanceScale] + "." + stringBalance[len(stringBalance)-BalanceScale:len(stringBalance)-BalancePrecision]
	}
	if res, err := strconv.ParseFloat(stringBalance, 64); err == nil {
		return res
	}
	return 0.0
}

// UpdateBlockHeight updates block height.
func UpdateBlockHeight(blockHeight uint64) {
	blockHeightCounter.Add(float64(blockHeight) - float64(curBlockHeight))
	blocksAcceptedGauge.Set(float64(blockHeight) - float64(curBlockHeight))
	curBlockHeight = blockHeight
	metricsPush <- BlockHeightPush
}

// UpdateNodeBalance updates node balance.
func UpdateNodeBalance(balance *big.Int) {
	nodeBalanceCounter.Add(FormatBalance(balance) - FormatBalance(curBalance))
	curBalance = balance
	metricsPush <- NodeBalancePush
}

// UpdateTxPoolSize updates tx pool size.
func UpdateTxPoolSize(txPoolSize uint64) {
	txPoolGauge.Set(float64(txPoolSize))
	curTxPoolSize = txPoolSize
	metricsPush <- TxPoolPush
}

// UpdateBlockReward updates block reward.
func UpdateBlockReward(blockReward *big.Int) {
	blockRewardGauge.Set(FormatBalance(blockReward))
	lastBlockReward = blockReward
	metricsPush <- BlockRewardPush
}

// UpdateLastConsensus updates last consensus time.
func UpdateLastConsensus(consensusTime int64) {
	lastConsensusGauge.Set(float64(consensusTime))
	lastConsensusTime = consensusTime
	metricsPush <- LastConsensusPush
}

// UpdateConnectionsNumber updates connections number.
func UpdateConnectionsNumber(connectionsNumber int) {
	connectionsNumberGauge.Set(float64(connectionsNumber))
	curConnectionsNumber = connectionsNumber
	metricsPush <- ConnectionsNumberPush
}

// PushMetrics pushes metrics updates to prometheus pushgateway.
func (s *Service) PushMetrics() {
	for metricType := range metricsPush {
		if metricType == -1 {
			break
		}
		if err := s.pusher.Add(); err != nil {
			utils.Logger().Error().Err(err).Msg("Could not push to a prometheus pushgateway.")
		}
		/*switch metricType {
		case ConnectionsNumberPush:
			s.storage.Dump(curConnectionsNumber, ConnectionsNumberPrefix)
		case BlockHeightPush:
			fmt.Println("LOL")
			s.storage.Dump(curBlockHeight, BlockHeightPrefix)
			s.storage.Dump(curBlocks, BlocksPrefix)
		case BlockRewardPush:
			s.storage.Dump(lastBlockReward, BlockHeightPrefix)
		case NodeBalancePush:
			s.storage.Dump(curBalance, BalancePrefix)
		case LastConsensusPush:
			s.storage.Dump(lastConsensusTime, ConsensusTimePrefix)
		case TxPoolPush:
			s.storage.Dump(curTxPoolSize, TxPoolPrefix)
		}*/
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
