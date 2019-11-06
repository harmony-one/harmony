package shardchain

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/params"

	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
)

// Collection is a collection of per-shard blockchains.
type Collection interface {
	// ShardChain returns the blockchain for the given shard,
	// opening one as necessary.
	ShardChain(shardID uint32) (*core.BlockChain, error)

	// CloseShardChain closes the given shard chain.
	CloseShardChain(shardID uint32) error

	// DisableCache disables caching mode for newly opened chains.
	DisableCache()

	// Close closes all shard chains.
	Close() error
}

type collectionImpl struct {
	dbFactory    DBFactory
	dbInit       DBInitializer
	engine       engine.Engine
	mtx          sync.Mutex
	pool         map[uint32]*core.BlockChain
	disableCache bool
	chainConfig  *params.ChainConfig
}

// NewCollection creates and returns a new shard chain collection.
//
// dbFactory is the shard chain database factory to use.
//
// dbInit is the shard chain initializer to use when the database returned by
// the factory is brand new (empty).
func NewCollection(
	dbFactory DBFactory, dbInit DBInitializer, engine engine.Engine,
	chainConfig *params.ChainConfig,
) Collection {
	return &collectionImpl{
		dbFactory:   dbFactory,
		dbInit:      dbInit,
		engine:      engine,
		pool:        make(map[uint32]*core.BlockChain),
		chainConfig: chainConfig,
	}
}

// ShardChain returns the blockchain for the given shard,
// opening one as necessary.
func (sc *collectionImpl) ShardChain(shardID uint32) (*core.BlockChain, error) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	if bc, ok := sc.pool[shardID]; ok {
		return bc, nil
	}
	var db ethdb.Database
	defer func() {
		if db != nil {
			db.Close()
		}
	}()
	var err error
	if db, err = sc.dbFactory.NewChainDB(shardID); err != nil {
		// NewChainDB may return incompletely initialized DB;
		// avoid closing it.
		db = nil
		return nil, ctxerror.New("cannot open chain database").WithCause(err)
	}
	if rawdb.ReadCanonicalHash(db, 0) == (common.Hash{}) {
		utils.Logger().Info().
			Uint32("shardID", shardID).
			Msg("initializing a new chain database")
		if err := sc.dbInit.InitChainDB(db, shardID); err != nil {
			return nil, ctxerror.New("cannot initialize a new chain database").
				WithCause(err)
		}
	}
	var cacheConfig *core.CacheConfig
	if sc.disableCache {
		cacheConfig = &core.CacheConfig{Disabled: true}
	}

	bc, err := core.NewBlockChain(
		db, cacheConfig, sc.chainConfig, sc.engine, vm.Config{}, nil,
	)
	if err != nil {
		return nil, ctxerror.New("cannot create blockchain").WithCause(err)
	}
	db = nil // don't close
	sc.pool[shardID] = bc
	return bc, nil
}

// DisableCache disables caching mode for newly opened chains.
// It does not affect already open chains.  For best effect,
// use this immediately after creating collection.
func (sc *collectionImpl) DisableCache() {
	sc.disableCache = true
}

// CloseShardChain closes the given shard chain.
func (sc *collectionImpl) CloseShardChain(shardID uint32) error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	bc, ok := sc.pool[shardID]
	if !ok {
		return ctxerror.New("shard chain not found", "shardID", shardID)
	}
	utils.Logger().Info().
		Uint32("shardID", shardID).
		Msg("closing shard chain")
	delete(sc.pool, shardID)
	bc.Stop()
	bc.ChainDb().Close()
	utils.Logger().Info().
		Uint32("shardID", shardID).
		Msg("closed shard chain")
	return nil
}

// Close closes all shard chains.
func (sc *collectionImpl) Close() error {
	newPool := make(map[uint32]*core.BlockChain)
	sc.mtx.Lock()
	oldPool := sc.pool
	sc.pool = newPool
	sc.mtx.Unlock()
	for shardID, bc := range oldPool {
		utils.Logger().Info().
			Uint32("shardID", shardID).
			Msg("closing shard chain")
		bc.Stop()
		bc.ChainDb().Close()
		utils.Logger().Info().
			Uint32("shardID", shardID).
			Msg("closed shard chain")
	}
	return nil
}
