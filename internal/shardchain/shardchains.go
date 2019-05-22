package shardchain

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"

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

	// Close closes all shard chains.
	Close() error
}

type collection struct {
	dbFactory DBFactory
	dbInit    DBInitializer
	engine    engine.Engine
	mtx       sync.Mutex
	pool      map[uint32]*core.BlockChain
}

func NewCollection(
	dbFactory DBFactory, dbInit DBInitializer, engine engine.Engine,
) *collection {
	return &collection{
		dbFactory: dbFactory,
		dbInit:    dbInit,
		engine:    engine,
		pool:      make(map[uint32]*core.BlockChain),
	}
}

// ShardChain returns the blockchain for the given shard,
// opening one as necessary.
func (sc *collection) ShardChain(shardID uint32) (*core.BlockChain, error) {
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
		utils.GetLogger().Info("initializing a new chain database",
			"shardID", shardID)
		if err := sc.dbInit.InitChainDB(db, shardID); err != nil {
			return nil, ctxerror.New("cannot initialize a new chain database").
				WithCause(err)
		}
	}
	var cacheConfig *core.CacheConfig
	// TODO ek – archival
	if false {
		cacheConfig = &core.CacheConfig{Disabled: true}
	}
	chainConfig := *params.TestChainConfig
	chainConfig.ChainID = big.NewInt(int64(shardID))
	bc, err := core.NewBlockChain(
		db, cacheConfig, &chainConfig, sc.engine, vm.Config{}, nil,
	)
	if err != nil {
		return nil, ctxerror.New("cannot create blockchain").WithCause(err)
	}
	db = nil // don't close
	sc.pool[shardID] = bc
	return bc, nil
}

// CloseShardChain closes the given shard chain.
func (sc *collection) CloseShardChain(shardID uint32) error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	bc, ok := sc.pool[shardID]
	if !ok {
		return ctxerror.New("shard chain not found", "shardID", shardID)
	}
	utils.GetLogger().Info("closing shard chain", "shardID", shardID)
	delete(sc.pool, shardID)
	bc.Stop()
	bc.ChainDb().Close()
	utils.GetLogger().Info("closed shard chain", "shardID", shardID)
	return nil
}

// Close closes all shard chains.
func (sc *collection) Close() error {
	newPool := make(map[uint32]*core.BlockChain)
	sc.mtx.Lock()
	oldPool := sc.pool
	sc.pool = newPool
	sc.mtx.Unlock()
	for shardID, bc := range oldPool {
		utils.GetLogger().Info("closing shard chain", "shardID", shardID)
		bc.Stop()
		bc.ChainDb().Close()
		utils.GetLogger().Info("closed shard chain", "shardID", shardID)
	}
	return nil
}
