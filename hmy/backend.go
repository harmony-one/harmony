package hmy

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	staking "github.com/harmony-one/harmony/staking/types"
)

// Harmony implements the Harmony full node service.
type Harmony struct {
	// Channel for shutting down the service
	shutdownChan  chan bool                      // Channel for shutting down the Harmony
	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests

	blockchain     *core.BlockChain
	txPool         *core.TxPool
	cxPool         *core.CxPool
	accountManager *accounts.Manager
	eventMux       *event.TypeMux
	// DB interfaces
	chainDb ethdb.Database // Block chain database

	bloomIndexer *core.ChainIndexer // Bloom indexer operating during block imports
	APIBackend   *APIBackend

	nodeAPI NodeAPI

	// aka network version, which is used to identify which network we are using
	networkID uint64
	// TODO(ricl): put this into config object
	// TODO(ricl): this is never set. Will result in nil pointer bug
	// RPCGasCap is the global gas cap for eth-call variants.
	RPCGasCap *big.Int `toml:",omitempty"`
	shardID   uint32
}

// NodeAPI is the list of functions from node used to call rpc apis.
type NodeAPI interface {
	AddPendingStakingTransaction(*staking.StakingTransaction)
	AddPendingTransaction(newTx *types.Transaction)
	Blockchain() *core.BlockChain
	AccountManager() *accounts.Manager
	GetBalanceOfAddress(address common.Address) (*big.Int, error)
	GetNonceOfAddress(address common.Address) uint64
	GetTransactionsHistory(address string) ([]common.Hash, error)
	IsCurrentlyLeader() bool
}

// New creates a new Harmony object (including the
// initialisation of the common Harmony object)
func New(nodeAPI NodeAPI, txPool *core.TxPool, cxPool *core.CxPool, eventMux *event.TypeMux, shardID uint32) (*Harmony, error) {
	chainDb := nodeAPI.Blockchain().ChainDB()
	hmy := &Harmony{
		shutdownChan:   make(chan bool),
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		blockchain:     nodeAPI.Blockchain(),
		txPool:         txPool,
		cxPool:         cxPool,
		accountManager: nodeAPI.AccountManager(),
		eventMux:       eventMux,
		chainDb:        chainDb,
		bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks, params.BloomConfirms),
		nodeAPI:        nodeAPI,
		networkID:      1, // TODO(ricl): this should be from config
		shardID:        shardID,
	}

	hmy.APIBackend = &APIBackend{hmy}

	return hmy, nil
}

// TxPool ...
func (s *Harmony) TxPool() *core.TxPool { return s.txPool }

// CxPool is used to store the blockHashes, where the corresponding block contains the cross shard receipts to be sent
func (s *Harmony) CxPool() *core.CxPool { return s.cxPool }

// BlockChain ...
func (s *Harmony) BlockChain() *core.BlockChain { return s.blockchain }

// NetVersion returns the network version, i.e. network ID identifying which network we are using
func (s *Harmony) NetVersion() uint64 { return s.networkID }
