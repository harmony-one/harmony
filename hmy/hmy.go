package hmy

import (
	"golang.org/x/sync/singleflight"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	staking "github.com/harmony-one/harmony/staking/types"
	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p-core/peer"
)

const (
	leaderCacheSize         = 250 // Approx number of BLS keys in committee
	totalStakeCacheDuration = 20  // number of blocks where the returned total stake will remain the same
)

// TODO: fuse api_backend into Harmony.
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
		NetVersion:      1, // TODO(ricl): this should be from config
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
