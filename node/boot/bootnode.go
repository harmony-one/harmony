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
	// Consensus          *consensus.Consensus // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BeaconBlockChannel chan *types.Block // The channel to send beacon blocks for non-beaconchain nodes

	// crosslinks *crosslinks.Crosslinks // Memory storage for crosslink processing.

	SelfPeer p2p.Peer
	// stateMutex sync.Mutex // mutex for change node state
	// TxPool           *core.TxPool
	// CxPool           *core.CxPool // pool for missing cross shard receipts resend
	// Worker           *worker.Worker
	// downloaderServer *downloader.Server
	// Syncing component.
	// syncID                 [SyncIDLength]byte // a unique ID for the node during the state syncing process with peers
	// stateSync              *legacysync.StateSync
	// epochSync              *legacysync.EpochSync
	// stateStagedSync        *stagedsync.StagedSync
	// peerRegistrationRecord map[string]*syncConfig // record registration time (unixtime) of peers begin in syncing
	// SyncingPeerProvider    SyncingPeerProvider
	// The p2p host used to send/receive p2p messages
	host p2p.Host
	// Service manager.
	serviceManager *service.Manager
	// ContractDeployerCurrentNonce uint64 // The nonce of the deployer contract at current block
	// ContractAddresses            []common.Address
	HarmonyConfig *harmonyconfig.HarmonyConfig
	// node configuration, including group ID, shard ID, etc
	NodeConfig *nodeconfig.ConfigType
	// Chain configuration.
	// chainConfig         params.ChainConfig
	unixTimeAtNodeStart int64

	// TransactionErrorSink contains error messages for any failed transaction, in memory only
	TransactionErrorSink *types.TransactionErrorSink
	// BroadcastInvalidTx flag is considered when adding pending tx to tx-pool
	// BroadcastInvalidTx bool

	// InSync flag indicates the node is in-sync or not
	// IsSynchronized *abool.AtomicBool

	// deciderCache   *lru.Cache
	// committeeCache *lru.Cache

	Metrics metrics.Registry

	// context control for pub-sub handling
	// psCtx    context.Context
	// psCancel func()
	// registry *registry.Registry
}

// New creates a new node.
func New(
	host p2p.Host,
	harmonyconfig *harmonyconfig.HarmonyConfig,
) *BootNode {
	node := BootNode{
		//registry:             registry.SetAddressToBLSKey(NewAddressToBLSKey(consensusObj.ShardID)),
		unixTimeAtNodeStart:  time.Now().Unix(),
		TransactionErrorSink: types.NewTransactionErrorSink(),
	}

	// Get the node config that's created in the harmony.go program.
	// node.NodeConfig = nodeconfig.GetShardConfig(consensusObj.ShardID)
	node.HarmonyConfig = harmonyconfig

	if host != nil {
		node.host = host
		node.SelfPeer = host.GetSelfPeer()
	}

	// networkType := node.NodeConfig.GetNetworkType()
	// chainConfig := networkType.ChainConfig()
	// node.chainConfig = chainConfig
	// node.IsSynchronized = abool.NewBool(false)

	// init metrics
	initMetrics()
	nodeStringCounterVec.WithLabelValues("version", nodeconfig.GetVersion()).Inc()

	node.serviceManager = service.NewManager()

	return &node
}

func (node *BootNode) init() (service.NodeConfig, chan p2p.Peer, error) {
	chanPeer := make(chan p2p.Peer)
	nodeConfig := service.NodeConfig{
		Beacon:       nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID),
		ShardGroupID: node.NodeConfig.GetShardGroupID(),
		Actions:      map[nodeconfig.GroupID]nodeconfig.ActionType{},
	}

	groups := []nodeconfig.GroupID{
		node.NodeConfig.GetShardGroupID(),
		node.NodeConfig.GetClientGroupID(),
	}

	// force the side effect of topic join
	if err := node.host.SendMessageToGroups(groups, []byte{}); err != nil {
		return nodeConfig, nil, err
	}

	return nodeConfig, chanPeer, nil
}

// ServiceManager ...
func (node *BootNode) ServiceManager() *service.Manager {
	return node.serviceManager
}

// ShutDown gracefully shut down the node server and dump the in-memory blockchain state into DB.
func (node *BootNode) ShutDown() {
	if err := node.StopRPC(); err != nil {
		utils.Logger().Error().Err(err).Msg("failed to stop RPC")
	}

	utils.Logger().Info().Msg("stopping services")
	if err := node.StopServices(); err != nil {
		utils.Logger().Error().Err(err).Msg("failed to stop services")
	}

	utils.Logger().Info().Msg("stopping host")
	if err := node.host.Close(); err != nil {
		utils.Logger().Error().Err(err).Msg("failed to stop p2p host")
	}

	// node.Blockchain().Stop()
	// node.Beaconchain().Stop()

	// if node.HarmonyConfig.General.RunElasticMode {
	// 	_, _ = node.Blockchain().RedisPreempt().Unlock()
	// 	_, _ = node.Beaconchain().RedisPreempt().Unlock()

	// 	_ = redis_helper.Close()
	// 	time.Sleep(time.Second)

	// 	if storage := tikv_manage.GetDefaultTiKVFactory(); storage != nil {
	// 		storage.CloseAllDB()
	// 	}
	// }

	const msg = "Successfully shut down!\n"
	utils.Logger().Print(msg)
	fmt.Print(msg)
	os.Exit(0)
}

// IsRunningBeaconChain returns whether the node is running on beacon chain.
func (node *BootNode) IsRunningBeaconChain() bool {
	return node.NodeConfig.ShardID == shard.BeaconChainShardID
}
