package metrics

import (
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/rpc"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/prometheus/client_golang/prometheus"
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
	IsLeaderPush                 int = 6
	metricsServicePortDifference     = 2000
)

// Service is the struct for metrics service.
type Service struct {
	BlsPublicKey    string
	IP              string
	Port            string
	PushgatewayIP   string
	PushgatewayPort string
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
	curIsLeader          = false
	lastBlockReward      = big.NewInt(0)
	lastConsensusTime    = int64(0)
	metricsPush          = make(chan int)
	blockHeightGauge     = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "block_height",
		Help: "Get current block height.",
	})
	txPoolGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tx_pool_size",
		Help: "Get current tx pool size.",
	})
	isLeaderGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "is_leader",
		Help: "Is node a leader now.",
	})
	blocksAcceptedGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "blocks_accepted",
		Help: "Get accepted blocks.",
	})
	connectionsNumberGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "connections_number",
		Help: "Get current connections number for a node.",
	})
	nodeBalanceGauge = prometheus.NewGauge(prometheus.GaugeOpts{
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

// New returns metrics service.
func New(selfPeer *p2p.Peer, blsPublicKey, pushgatewayIP, pushgatewayPort string) *Service {
	return &Service{
		BlsPublicKey:    blsPublicKey,
		IP:              selfPeer.IP,
		Port:            selfPeer.Port,
		PushgatewayIP:   pushgatewayIP,
		PushgatewayPort: pushgatewayPort,
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
	registry := prometheus.NewRegistry()
	registry.MustRegister(blockHeightGauge, connectionsNumberGauge, nodeBalanceGauge, lastConsensusGauge, blockRewardGauge, blocksAcceptedGauge, txPoolGauge, isLeaderGauge)

	s.pusher = push.New("http://"+s.PushgatewayIP+":"+s.PushgatewayPort, "node_metrics").Gatherer(registry).Grouping("instance", s.IP+":"+s.Port).Grouping("bls_key", s.BlsPublicKey)
	go s.PushMetrics()
}

// FormatBalance formats big.Int balance with precision.
func FormatBalance(balance *big.Int) float64 {
	scaledBalance := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetFloat64(math.Pow10(BalanceScale)))
	floatBalance, _ := scaledBalance.Float64()
	return floatBalance
}

// UpdateBlockHeight updates block height.
func UpdateBlockHeight(blockHeight uint64) {
	blockHeightGauge.Set(float64(blockHeight))
	blocksAcceptedGauge.Set(float64(blockHeight) - float64(curBlockHeight))
	curBlockHeight = blockHeight
	metricsPush <- BlockHeightPush
}

// UpdateNodeBalance updates node balance.
func UpdateNodeBalance(balance *big.Int) {
	nodeBalanceGauge.Set(FormatBalance(balance))
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

// UpdateIsLeader updates if node is a leader.
func UpdateIsLeader(isLeader bool) {
	if isLeader {
		isLeaderGauge.Set(1.0)
	} else {
		isLeaderGauge.Set(0.0)
	}
	curIsLeader = isLeader
	metricsPush <- IsLeaderPush
}

// PushMetrics pushes metrics updates to prometheus pushgateway.
func (s *Service) PushMetrics() {
	for metricType := range metricsPush {
		if metricType == -1 {
			break
		}
		if err := s.pusher.Add(); err != nil {
			utils.Logger().Error().Err(err).Msg("Could not push to a prometheus pushgateway.")
			// No dump for now, not necessarily for metrics and consumes memory, doesn't restore from db anyway.
			/* switch metricType {
			case ConnectionsNumberPush:
				s.storage.Dump(curConnectionsNumber, ConnectionsNumberPrefix)
			case BlockHeightPush:
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
			case IsLeaderPush:
				s.storage.Dump(curIsLeader, IsLeaderPrefix)
			}*/
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
