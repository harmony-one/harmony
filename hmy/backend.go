package hmy

import (
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/core"
)

// Harmony implements the Harmony full node service.
type Harmony struct {
	blockchain     *core.BlockChain
	txPool         *core.TxPool
	accountManager *accounts.Manager
	eventMux       *event.TypeMux
	// DB interfaces
	chainDb ethdb.Database // Block chain database

	bloomIndexer *core.ChainIndexer // Bloom indexer operating during block imports
	APIBackend   *APIBackend
}

// New creates a new Harmony object (including the
// initialisation of the common Ethereum object)
func New(blockchain *core.BlockChain, txPool *core.TxPool, accountManager *accounts.Manager, eventMux *event.TypeMux) (*Harmony, error) {
	chainDb := blockchain.ChainDB()
	hmy := &Harmony{
		blockchain:     blockchain,
		txPool:         txPool,
		accountManager: accountManager,
		eventMux:       eventMux,
		chainDb:        chainDb,
		bloomIndexer:   nil, // TODO(ricl): implement
	}

	hmy.APIBackend = &APIBackend{hmy}

	return hmy, nil
}

// TxPool ...
func (s *Harmony) TxPool() *core.TxPool { return s.txPool }

// BlockChain ...
func (s *Harmony) BlockChain() *core.BlockChain { return s.blockchain }
