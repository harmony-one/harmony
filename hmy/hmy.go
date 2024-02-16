package hmy

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	commonRPC "github.com/harmony-one/harmony/rpc/common"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
)

const (
	// BloomBitsBlocks is the number of blocks a single bloom bit section vector
	// contains on the server side.
	BloomBitsBlocks                 uint64 = 4096
	leaderCacheSize                        = 250  // Approx number of BLS keys in committee
	undelegationPayoutsCacheSize           = 500  // max number of epochs to store in cache
	preStakingBlockRewardsCacheSize        = 1024 // max number of block rewards to store in cache
	totalStakeCacheDuration                = 20   // number of blocks where the returned total stake will remain the same
	// max number of blocks for which the map "validator address -> total delegation to validator" is stored
	stakeByBlockNumberCacheSize = 250
)

var (
	// ErrFinalizedTransaction is returned if the transaction to be submitted is already on-chain
	ErrFinalizedTransaction = errors.New("transaction already finalized")
)

// Harmony implements the Harmony full node service.
type Harmony struct {
	// Channel for shutting down the service
	ShutdownChan  chan bool                      // Channel for shutting down the Harmony
	BloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	BlockChain    core.BlockChain
	BeaconChain   core.BlockChain
	TxPool        *core.TxPool
	CxPool        *core.CxPool // CxPool is used to store the blockHashes of blocks containing cx receipts to be sent
	// DB interfaces
	BloomIndexer *core.ChainIndexer // Bloom indexer operating during block imports
	NodeAPI      NodeAPI
	// ChainID is used to identify which network we are using
	ChainID uint64
	// EthCompatibleChainID is used to identify the Ethereum compatible chain ID
	EthChainID uint64
	// RPCGasCap is the global gas cap for eth-call variants.
	RPCGasCap *big.Int `toml:",omitempty"`
	ShardID   uint32

	// Gas price suggestion oracle
	gpo *Oracle

	// Internals
	eventMux *event.TypeMux
	chainDb  ethdb.Database // Block chain database
	// group for units of work which can be executed with duplicate suppression.
	group singleflight.Group
	// leaderCache to save on recomputation every epoch.
	leaderCache *lru.Cache
	// undelegationPayoutsCache to save on recomputation every epoch
	undelegationPayoutsCache *lru.Cache
	// preStakingBlockRewardsCache to save on recomputation for commonly checked blocks in epoch < staking epoch
	preStakingBlockRewardsCache *lru.Cache
	// totalStakeCache to save on recomputation for `totalStakeCacheDuration` blocks.
	totalStakeCache *totalStakeCache
	// stakeByBlockNumberCache to save on recomputation for `totalStakeCacheDuration` blocks
	stakeByBlockNumberCache *lru.Cache
}

// NodeAPI is the list of functions from node used to call rpc apis.
type NodeAPI interface {
	AddPendingStakingTransaction(*staking.StakingTransaction) error
	AddPendingTransaction(newTx *types.Transaction) error
	Blockchain() core.BlockChain
	Beaconchain() core.BlockChain
	GetTransactionsHistory(address, txType, order string) ([]common.Hash, error)
	GetStakingTransactionsHistory(address, txType, order string) ([]common.Hash, error)
	GetTransactionsCount(address, txType string) (uint64, error)
	GetStakingTransactionsCount(address, txType string) (uint64, error)
	GetTraceResultByHash(hash common.Hash) (json.RawMessage, error)
	IsCurrentlyLeader() bool
	IsOutOfSync(shardID uint32) bool
	SyncStatus(shardID uint32) (bool, uint64, uint64)
	SyncPeers() map[string]int
	ReportStakingErrorSink() types.TransactionErrorReports
	ReportPlainErrorSink() types.TransactionErrorReports
	PendingCXReceipts() []*types.CXReceiptsProof
	GetNodeBootTime() int64
	PeerConnectivity() (int, int, int)
	ListPeer(topic string) []peer.ID
	ListTopic() []string
	ListBlockedPeer() []peer.ID

	GetConsensusInternal() commonRPC.ConsensusInternal
	IsBackup() bool
	SetNodeBackupMode(isBackup bool) bool

	// debug API
	GetConsensusMode() string
	GetConsensusPhase() string
	GetConsensusViewChangingID() uint64
	GetConsensusCurViewID() uint64
	GetConfig() commonRPC.Config
	ShutDown()
	GetLastSigningPower() (float64, error)
	GetLastSigningPower2() (float64, error)
}

// New creates a new Harmony object (including the
// initialisation of the common Harmony object)
func New(
	nodeAPI NodeAPI, txPool *core.TxPool, cxPool *core.CxPool, shardID uint32,
) *Harmony {
	leaderCache, _ := lru.New(leaderCacheSize)
	undelegationPayoutsCache, _ := lru.New(undelegationPayoutsCacheSize)
	stakeByBlockNumberCache, _ := lru.New(stakeByBlockNumberCacheSize)
	preStakingBlockRewardsCache, _ := lru.New(preStakingBlockRewardsCacheSize)
	totalStakeCache := newTotalStakeCache(totalStakeCacheDuration)
	bloomIndexer := NewBloomIndexer(nodeAPI.Blockchain(), params.BloomBitsBlocks, params.BloomConfirms)
	bloomIndexer.Start(nodeAPI.Blockchain())

	backend := &Harmony{
		ShutdownChan:                make(chan bool),
		BloomRequests:               make(chan chan *bloombits.Retrieval),
		BloomIndexer:                bloomIndexer,
		BlockChain:                  nodeAPI.Blockchain(),
		BeaconChain:                 nodeAPI.Beaconchain(),
		TxPool:                      txPool,
		CxPool:                      cxPool,
		eventMux:                    new(event.TypeMux),
		chainDb:                     nodeAPI.Blockchain().ChainDb(),
		NodeAPI:                     nodeAPI,
		ChainID:                     nodeAPI.Blockchain().Config().ChainID.Uint64(),
		EthChainID:                  nodeAPI.Blockchain().Config().EthCompatibleChainID.Uint64(),
		ShardID:                     shardID,
		leaderCache:                 leaderCache,
		totalStakeCache:             totalStakeCache,
		undelegationPayoutsCache:    undelegationPayoutsCache,
		preStakingBlockRewardsCache: preStakingBlockRewardsCache,
		stakeByBlockNumberCache:     stakeByBlockNumberCache,
	}

	// Setup gas price oracle
	config := nodeAPI.GetConfig().HarmonyConfig.GPO
	gpo := NewOracle(backend, &config)
	backend.gpo = gpo

	return backend
}

// SingleFlightRequest ..
func (hmy *Harmony) SingleFlightRequest(
	key string,
	fn func() (interface{}, error),
) (interface{}, error) {
	res, err, _ := hmy.group.Do(key, fn)
	return res, err
}

// SingleFlightForgetKey ...
func (hmy *Harmony) SingleFlightForgetKey(key string) {
	hmy.group.Forget(key)
}

// ProtocolVersion ...
func (hmy *Harmony) ProtocolVersion() int {
	return proto.ProtocolVersion
}

// IsLeader exposes if node is currently leader.
func (hmy *Harmony) IsLeader() bool {
	return hmy.NodeAPI.IsCurrentlyLeader()
}

// GetNodeMetadata returns the node metadata.
func (hmy *Harmony) GetNodeMetadata() commonRPC.NodeMetadata {
	var (
		header     = hmy.CurrentHeader()
		cfg        = nodeconfig.GetShardConfig(header.ShardID())
		blockEpoch *uint64
		blsKeys    []string
		c          = commonRPC.C{}
	)

	if header.ShardID() == shard.BeaconChainShardID {
		sched := shard.Schedule.InstanceForEpoch(header.Epoch())
		b := sched.BlocksPerEpoch()
		blockEpoch = &b
	}

	if cfg.ConsensusPriKey != nil {
		for _, key := range cfg.ConsensusPriKey {
			blsKeys = append(blsKeys, key.Pub.Bytes.Hex())
		}
	}

	c.TotalKnownPeers, c.Connected, c.NotConnected = hmy.NodeAPI.PeerConnectivity()

	syncPeers := hmy.NodeAPI.SyncPeers()
	consensusInternal := hmy.NodeAPI.GetConsensusInternal()

	return commonRPC.NodeMetadata{
		BLSPublicKey:    blsKeys,
		Version:         nodeconfig.GetVersion(),
		NetworkType:     string(cfg.GetNetworkType()),
		ChainConfig:     *hmy.ChainConfig(),
		IsLeader:        hmy.IsLeader(),
		ShardID:         hmy.ShardID,
		CurrentBlockNum: header.Number().Uint64(),
		CurrentEpoch:    header.Epoch().Uint64(),
		BlocksPerEpoch:  blockEpoch,
		Role:            cfg.Role().String(),
		DNSZone:         cfg.DNSZone,
		Archival:        cfg.GetArchival(),
		IsBackup:        hmy.NodeAPI.IsBackup(),
		NodeBootTime:    hmy.NodeAPI.GetNodeBootTime(),
		PeerID:          nodeconfig.GetPeerID(),
		Consensus:       consensusInternal,
		C:               c,
		SyncPeers:       syncPeers,
	}
}

// GetEVM returns a new EVM entity
func (hmy *Harmony) GetEVM(ctx context.Context, msg core.Message, state *state.DB, header *block.Header) (*vm.EVM, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	vmCtx := core.NewEVMContext(msg, header, hmy.BlockChain, nil)
	return vm.NewEVM(vmCtx, state, hmy.BlockChain.Config(), *hmy.BlockChain.GetVMConfig()), nil
}

// ChainDb ..
func (hmy *Harmony) ChainDb() ethdb.Database {
	return hmy.chainDb
}

// EventMux ..
func (hmy *Harmony) EventMux() *event.TypeMux {
	return hmy.eventMux
}

// BloomStatus ...
// TODO: this is not implemented or verified yet for harmony.
func (hmy *Harmony) BloomStatus() (uint64, uint64) {
	sections, _, _ := hmy.BloomIndexer.Sections()
	return BloomBitsBlocks, sections
}
