package bootnode

import (
	"fmt"
	"os"
	"time"

	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/core/types"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/rcrowley/go-metrics"
)

const (
	// NumTryBroadCast is the number of times trying to broadcast
	NumTryBroadCast = 3
	// MsgChanBuffer is the buffer of consensus message handlers.
	MsgChanBuffer = 1024
)

// BootNode represents a protocol-participating node in the network
type BootNode struct {
	SelfPeer p2p.Peer
	host     p2p.Host
	// Service manager.
	serviceManager *service.Manager
	// harmony configurations
	HarmonyConfig *harmonyconfig.HarmonyConfig
	// node configuration, including group ID, shard ID, etc
	NodeConfig *nodeconfig.ConfigType
	// node start time
	unixTimeAtNodeStart int64
	// TransactionErrorSink contains error messages for any failed transaction, in memory only
	TransactionErrorSink *types.TransactionErrorSink
	// metrics
	Metrics metrics.Registry
}

// New creates a new boot node.
func New(
	host p2p.Host,
	harmonyconfig *harmonyconfig.HarmonyConfig,
) *BootNode {
	node := BootNode{
		unixTimeAtNodeStart:  time.Now().Unix(),
		TransactionErrorSink: types.NewTransactionErrorSink(),
	}

	node.HarmonyConfig = harmonyconfig

	if host != nil {
		node.host = host
		node.SelfPeer = host.GetSelfPeer()
	}

	// init metrics
	initMetrics()
	nodeStringCounterVec.WithLabelValues("version", nodeconfig.GetVersion()).Inc()

	node.serviceManager = service.NewManager()

	return &node
}

// ServiceManager ...
func (bootnode *BootNode) ServiceManager() *service.Manager {
	return bootnode.serviceManager
}

// ShutDown gracefully shut down the node server and dump the in-memory blockchain state into DB.
func (bootnode *BootNode) ShutDown() {
	if err := bootnode.StopRPC(); err != nil {
		utils.Logger().Error().Err(err).Msg("failed to stop RPC")
	}

	utils.Logger().Info().Msg("stopping services")
	if err := bootnode.StopServices(); err != nil {
		utils.Logger().Error().Err(err).Msg("failed to stop services")
	}

	utils.Logger().Info().Msg("stopping host")
	if err := bootnode.host.Close(); err != nil {
		utils.Logger().Error().Err(err).Msg("failed to stop p2p host")
	}

	const msg = "Successfully shut down!\n"
	utils.Logger().Print(msg)
	fmt.Print(msg)
	os.Exit(0)
}

// IsRunningBeaconChain returns whether the node is running on beacon chain.
func (bootnode *BootNode) IsRunningBeaconChain() bool {
	return bootnode.NodeConfig.ShardID == shard.BeaconChainShardID
}
