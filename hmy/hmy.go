package hmy

import (
	"context"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	commonRPC "github.com/harmony-one/harmony/internal/hmyapi/common"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/sync/singleflight"
)

const (
	// BloomBitsBlocks is the number of blocks a single bloom bit section vector
	// contains on the server side.
	BloomBitsBlocks         uint64 = 4096
	leaderCacheSize                = 250 // Approx number of BLS keys in committee
	totalStakeCacheDuration        = 20  // number of blocks where the returned total stake will remain the same
)

// Harmony implements the Harmony full node service.
type Harmony struct {
	// Channel for shutting down the service
	ShutdownChan  chan bool                      // Channel for shutting down the Harmony
	BloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	BlockChain    *core.BlockChain
	BeaconChain   *core.BlockChain
	TxPool        *core.TxPool
	// CxPool is used to store the blockHashes, where the corresponding block contains the cx receipts to be sent
	CxPool   *core.CxPool
	EventMux *event.TypeMux
	// DB interfaces
	ChainDb      ethdb.Database     // Block chain database
	BloomIndexer *core.ChainIndexer // Bloom indexer operating during block imports
	APIBackend   *APIBackend
	NodeAPI      NodeAPI
	// NetVersion is used to identify which network we are using
	NetVersion uint64
	// RPCGasCap is the global gas cap for eth-call variants.
	RPCGasCap *big.Int `toml:",omitempty"`
	ShardID   uint32

	// Internals
	// group for units of work which can be executed with duplicate suppression.
	group singleflight.Group
	// leaderCache to save on recomputation every epoch.
	leaderCache *lru.Cache
	// totalStakeCache to save on recomputation for `totalStakeCacheDuration` blocks.
	totalStakeCache *totalStakeCache
}

// NodeAPI is the list of functions from node used to call rpc apis.
type NodeAPI interface {
	AddPendingStakingTransaction(*staking.StakingTransaction) error
	AddPendingTransaction(newTx *types.Transaction) error
	Blockchain() *core.BlockChain
	Beaconchain() *core.BlockChain
	GetTransactionsHistory(address, txType, order string) ([]common.Hash, error)
	GetStakingTransactionsHistory(address, txType, order string) ([]common.Hash, error)
	GetTransactionsCount(address, txType string) (uint64, error)
	GetStakingTransactionsCount(address, txType string) (uint64, error)
	IsCurrentlyLeader() bool
	ReportStakingErrorSink() types.TransactionErrorReports
	ReportPlainErrorSink() types.TransactionErrorReports
	PendingCXReceipts() []*types.CXReceiptsProof
	GetNodeBootTime() int64
	PeerConnectivity() (int, int, int)
	ListPeer(topic string) []peer.ID
	ListTopic() []string
	ListBlockedPeer() []peer.ID
}

// New creates a new Harmony object (including the
// initialisation of the common Harmony object)
func New(
	nodeAPI NodeAPI, txPool *core.TxPool,
	cxPool *core.CxPool, eventMux *event.TypeMux, shardID uint32,
) (*Harmony, error) {
	chainDb := nodeAPI.Blockchain().ChainDB()
	leaderCache, _ := lru.New(leaderCacheSize)
	totalStakeCache := newTotalStakeCache(totalStakeCacheDuration)
	hmy := &Harmony{
		ShutdownChan:    make(chan bool),
		BloomRequests:   make(chan chan *bloombits.Retrieval),
		BlockChain:      nodeAPI.Blockchain(),
		BeaconChain:     nodeAPI.Beaconchain(),
		TxPool:          txPool,
		CxPool:          cxPool,
		EventMux:        eventMux,
		ChainDb:         chainDb,
		NodeAPI:         nodeAPI,
		NetVersion:      1, // TODO(dm): this should be from config
		ShardID:         shardID,
		leaderCache:     leaderCache,
		totalStakeCache: totalStakeCache,
	}
	hmy.APIBackend = &APIBackend{
		hmy: hmy,
		TotalStakingCache: struct {
			sync.Mutex
			BlockHeight  int64
			TotalStaking *big.Int
		}{
			BlockHeight:  -1,
			TotalStaking: big.NewInt(0),
		},
		LeaderCache: leaderCache,
	}
	return hmy, nil
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

// IsLeader exposes if node is currently leader
func (hmy *Harmony) IsLeader() bool {
	return hmy.NodeAPI.IsCurrentlyLeader()
}

// GetNodeMetadata ..
func (hmy *Harmony) GetNodeMetadata() commonRPC.NodeMetadata {
	cfg := nodeconfig.GetDefaultConfig()
	header := hmy.CurrentBlock().Header()
	var blockEpoch *uint64

	if header.ShardID() == shard.BeaconChainShardID {
		sched := shard.Schedule.InstanceForEpoch(header.Epoch())
		b := sched.BlocksPerEpoch()
		blockEpoch = &b
	}

	blsKeys := []string{}
	if cfg.ConsensusPriKey != nil {
		for _, key := range cfg.ConsensusPriKey {
			blsKeys = append(blsKeys, key.Pub.Bytes.Hex())
		}
	}
	c := commonRPC.C{}
	c.TotalKnownPeers, c.Connected, c.NotConnected = hmy.NodeAPI.PeerConnectivity()

	return commonRPC.NodeMetadata{
		BLSPublicKey:   blsKeys,
		Version:        nodeconfig.GetVersion(),
		NetworkType:    string(cfg.GetNetworkType()),
		ChainConfig:    *hmy.ChainConfig(),
		IsLeader:       hmy.IsLeader(),
		ShardID:        hmy.ShardID,
		CurrentEpoch:   header.Epoch().Uint64(),
		BlocksPerEpoch: blockEpoch,
		Role:           cfg.Role().String(),
		DNSZone:        cfg.DNSZone,
		Archival:       cfg.GetArchival(),
		NodeBootTime:   hmy.NodeAPI.GetNodeBootTime(),
		PeerID:         nodeconfig.GetPeerID(),
		C:              c,
	}
}

// GetEVM returns a new EVM entity
func (hmy *Harmony) GetEVM(ctx context.Context, msg core.Message, state *state.DB, header *block.Header) (*vm.EVM, func() error, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	vmError := func() error { return nil }
	vmCtx := core.NewEVMContext(msg, header, hmy.BlockChain, nil)
	return vm.NewEVM(vmCtx, state, hmy.BlockChain.Config(), *hmy.BlockChain.GetVMConfig()), vmError, nil
}

// GetRPCGasCap returns the gas cap of rpc
func (hmy *Harmony) GetRPCGasCap() *big.Int {
	return hmy.RPCGasCap // TODO(dm): should be hmy.config.RPCGasCap
}

// BloomStatus ...
// TODO: this is not implemented or verified yet for harmony.
func (hmy *Harmony) BloomStatus() (uint64, uint64) {
	sections, _, _ := hmy.BloomIndexer.Sections()
	return BloomBitsBlocks, sections
}
