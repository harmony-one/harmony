package node

import (
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/abool"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/knownpeers"
	"github.com/harmony-one/harmony/node/worker"

	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/tikv/redis_helper"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/internal/utils/crosslinks"
	"github.com/harmony-one/harmony/internal/utils/lrucache"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/harmony-one/harmony/webhooks"
)

// New creates a new node.
func New(
	host p2p.Host,
	consensusObj *consensus.Consensus,
	engine engine.Engine,
	collection *shardchain.CollectionImpl,
	blacklist map[common.Address]struct{},
	allowedTxs map[common.Address]core.AllowedTxData,
	localAccounts []common.Address,
	isArchival map[uint32]bool,
	harmonyconfig *harmonyconfig.HarmonyConfig,
	registry *registry.Registry,
) *Node {
	node := Node{
		registry:             registry,
		unixTimeAtNodeStart:  time.Now().Unix(),
		TransactionErrorSink: types.NewTransactionErrorSink(),
		crosslinks:           crosslinks.New(),
		syncID:               GenerateSyncID(),
		keysToAddrs:          lrucache.NewCache[uint64, map[string]common.Address](10),
		knownPeers:           knownpeers.NewKnownHostsThreadSafe(),
	}
	if consensusObj == nil {
		panic("consensusObj is nil")
	}
	// Get the node config that's created in the harmony.go program.
	node.NodeConfig = nodeconfig.GetShardConfig(consensusObj.ShardID)
	node.HarmonyConfig = harmonyconfig

	if host != nil {
		node.host = host
		node.SelfPeer = host.GetSelfPeer()
	}

	networkType := node.NodeConfig.GetNetworkType()
	chainConfig := networkType.ChainConfig()
	node.chainConfig = chainConfig
	node.shardChains = collection
	node.IsSynchronized = abool.NewBool(false)

	if host != nil {
		// Consensus and associated channel to communicate blocks
		node.Consensus = consensusObj

		// Load the chains.
		blockchain := node.Blockchain() // this also sets node.isFirstTime if the DB is fresh
		var beaconChain core.BlockChain
		if blockchain.ShardID() == shard.BeaconChainShardID {
			beaconChain = node.Beaconchain()
		} else {
			beaconChain = node.EpochChain()
		}

		if b1, b2 := beaconChain == nil, blockchain == nil; b1 || b2 {
			// in tikv mode, not need BeaconChain
			if !(node.HarmonyConfig != nil && node.HarmonyConfig.General.RunElasticMode) || node.HarmonyConfig.General.ShardID == shard.BeaconChainShardID {
				var err error
				if b2 {
					shardID := node.NodeConfig.ShardID
					// HACK get the real error reason
					_, err = node.shardChains.ShardChain(shardID)
				} else {
					_, err = node.shardChains.ShardChain(shard.BeaconChainShardID)
				}
				fmt.Fprintf(os.Stderr, "Cannot initialize node: %v\n", err)
				os.Exit(-1)
			}
		}

		node.BeaconBlockChannel = make(chan *types.Block)
		txPoolConfig := core.DefaultTxPoolConfig

		if harmonyconfig != nil {
			txPoolConfig.AccountSlots = harmonyconfig.TxPool.AccountSlots
			txPoolConfig.GlobalSlots = harmonyconfig.TxPool.GlobalSlots
			txPoolConfig.Locals = append(txPoolConfig.Locals, localAccounts...)
			txPoolConfig.AccountQueue = harmonyconfig.TxPool.AccountQueue
			txPoolConfig.GlobalQueue = harmonyconfig.TxPool.GlobalQueue
			txPoolConfig.Lifetime = harmonyconfig.TxPool.Lifetime
			txPoolConfig.PriceLimit = uint64(harmonyconfig.TxPool.PriceLimit)
			txPoolConfig.PriceBump = harmonyconfig.TxPool.PriceBump
		}
		// Temporarily not updating other networks to make the rpc tests pass
		if node.NodeConfig.GetNetworkType() != nodeconfig.Mainnet && node.NodeConfig.GetNetworkType() != nodeconfig.Testnet {
			txPoolConfig.PriceLimit = 1e9
			txPoolConfig.PriceBump = 10
		}

		txPoolConfig.Blacklist = blacklist
		txPoolConfig.AllowedTxs = allowedTxs
		txPoolConfig.Journal = fmt.Sprintf("%v/%v", node.NodeConfig.DBDir, txPoolConfig.Journal)
		txPoolConfig.AddEvent = func(tx types.PoolTransaction, local bool) {
			// in tikv mode, writer will publish tx pool update to all reader
			if node.Blockchain().IsTikvWriterMaster() {
				err := redis_helper.PublishTxPoolUpdate(uint32(harmonyconfig.General.ShardID), tx, local)
				if err != nil {
					utils.Logger().Warn().Err(err).Msg("redis publish txpool update error")
				}
			}
		}

		node.TxPool = core.NewTxPool(txPoolConfig, node.Blockchain().Config(), blockchain, node.TransactionErrorSink)
		node.registry.SetTxPool(node.TxPool)
		node.CxPool = core.NewCxPool(core.CxPoolSize)
		node.Worker = worker.New(node.Blockchain().Config(), blockchain, beaconChain, engine)

		node.pendingCXReceipts = map[string]*types.CXReceiptsProof{}
		node.Consensus.VerifiedNewBlock = make(chan *types.Block, 1)
		// the sequence number is the next block number to be added in consensus protocol, which is
		// always one more than current chain header block
		node.Consensus.SetBlockNum(blockchain.CurrentBlock().NumberU64() + 1)
	}

	utils.Logger().Info().
		Interface("genesis block header", node.Blockchain().GetHeaderByNumber(0)).
		Msg("Genesis block hash")
	// Setup initial state of syncing.
	node.peerRegistrationRecord = map[string]*syncConfig{}
	// Broadcast double-signers reported by consensus
	if node.Consensus != nil {
		go func() {
			for doubleSign := range node.Consensus.SlashChan {
				utils.Logger().Info().
					RawJSON("double-sign-candidate", []byte(doubleSign.String())).
					Msg("double sign notified by consensus leader")
				// no point to broadcast the slash if we aren't even in the right epoch yet
				if !node.Blockchain().Config().IsStaking(
					node.Blockchain().CurrentHeader().Epoch(),
				) {
					return
				}
				if hooks := node.NodeConfig.WebHooks.Hooks; hooks != nil {
					if s := hooks.Slashing; s != nil {
						url := s.OnNoticeDoubleSign
						go func() { webhooks.DoPost(url, &doubleSign) }()
					}
				}
				if !node.IsRunningBeaconChain() {
					go node.BroadcastSlash(&doubleSign)
				} else {
					records := slash.Records{doubleSign}
					if err := node.Blockchain().AddPendingSlashingCandidates(
						records,
					); err != nil {
						utils.Logger().Err(err).Msg("could not add new slash to ending slashes")
					}
				}
			}
		}()
	}

	// in tikv mode, not need BeaconChain
	if !(node.HarmonyConfig != nil && node.HarmonyConfig.General.RunElasticMode) || node.HarmonyConfig.General.ShardID == shard.BeaconChainShardID {
		// update reward values now that node is ready
		node.updateInitialRewardValues()
	}

	// init metrics
	initMetrics()
	nodeStringCounterVec.WithLabelValues("version", nodeconfig.GetVersion()).Inc()

	node.serviceManager = service.NewManager()

	return &node
}
