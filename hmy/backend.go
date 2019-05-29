package hmy

import (
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
)

// Harmony implements the Harmony full node service.
type Harmony struct {
	// Channel for shutting down the service
	shutdownChan  chan bool                      // Channel for shutting down the Harmony
	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests

	blockchain     *core.BlockChain
	txPool         *core.TxPool
	accountManager *accounts.Manager
	eventMux       *event.TypeMux
	// DB interfaces
	chainDb ethdb.Database // Block chain database

	bloomIndexer *core.ChainIndexer // Bloom indexer operating during block imports
	APIBackend   *APIBackend
	nodeAPI      NodeAPIFunctions
}

// NodeAPIFunctions is the list of functions from node used to call rpc apis.
type NodeAPIFunctions interface {
	AddPendingTransaction(newTx *types.Transaction)
}

// New creates a new Harmony object (including the
// initialisation of the common Harmony object)
func New(blockchain *core.BlockChain, txPool *core.TxPool, accountManager *accounts.Manager, eventMux *event.TypeMux, nodeAPI NodeAPIFunctions) (*Harmony, error) {
	chainDb := blockchain.ChainDB()
	hmy := &Harmony{
		shutdownChan:   make(chan bool),
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		blockchain:     blockchain,
		txPool:         txPool,
		nodeAPI:        nodeAPI,
		accountManager: accountManager,
		eventMux:       eventMux,
		chainDb:        chainDb,
		bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks, params.BloomConfirms),
	}

	hmy.APIBackend = &APIBackend{hmy}

	return hmy, nil
}

// TxPool ...
func (s *Harmony) TxPool() *core.TxPool { return s.txPool }

// BlockChain ...
func (s *Harmony) BlockChain() *core.BlockChain { return s.blockchain }
