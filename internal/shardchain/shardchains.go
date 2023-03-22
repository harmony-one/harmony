package shardchain

import (
	"math/big"
	"sync"
	"time"

	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/chain"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/shardchain/tikv_manage"

	"github.com/harmony-one/harmony/shard"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/pkg/errors"
)

// Collection is a collection of per-shard blockchains.
type Collection interface {
	// ShardChain returns the blockchain for the given shard,
	// opening one as necessary.
	ShardChain(shardID uint32, options ...core.Options) (core.BlockChain, error)

	// CloseShardChain closes the given shard chain.
	CloseShardChain(shardID uint32) error

	// Close closes all shard chains.
	Close() error
}

// CollectionImpl is the main implementation of the shard chain collection.
// See the Collection interface for details.
type CollectionImpl struct {
	dbFactory     DBFactory
	dbInit        DBInitializer
	mtx           sync.Mutex
	pool          map[uint32]core.BlockChain
	disableCache  map[uint32]bool
	networkType   nodeconfig.NetworkType
	harmonyconfig *harmonyconfig.HarmonyConfig
}

// NewCollection creates and returns a new shard chain collection.
//
// dbFactory is the shard chain database factory to use.
//
// dbInit is the shard chain initializer to use when the database returned by
// the factory is brand new (empty).
func NewCollection(
	harmonyconfig *harmonyconfig.HarmonyConfig,
	dbFactory DBFactory, dbInit DBInitializer,
	network nodeconfig.NetworkType,
) *CollectionImpl {
	return &CollectionImpl{
		harmonyconfig: harmonyconfig,
		dbFactory:     dbFactory,
		dbInit:        dbInit,
		pool:          make(map[uint32]core.BlockChain),
		disableCache:  make(map[uint32]bool),
		networkType:   network,
	}
}

// ShardChain returns the blockchain for the given shard,
// opening one as necessary.
func (sc *CollectionImpl) ShardChain(shardID uint32, options ...core.Options) (core.BlockChain, error) {
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
		return nil, errors.Wrap(err, "cannot open chain database")
	}
	if rawdb.ReadCanonicalHash(db, 0) == (common.Hash{}) {
		utils.Logger().Info().
			Uint32("shardID", shardID).
			Msg("initializing a new chain database")
		if err := sc.dbInit.InitChainDB(db, sc.networkType, shardID); err != nil {
			return nil, errors.Wrapf(err, "cannot initialize a new chain database")
		}
	}
	var cacheConfig *core.CacheConfig
	if sc.disableCache[shardID] {
		cacheConfig = &core.CacheConfig{Disabled: true}
		utils.Logger().Info().
			Uint32("shardID", shardID).
			Msg("disable cache, running in archival mode")
	} else {
		cacheConfig = &core.CacheConfig{
			TrieNodeLimit: 256 * 1024 * 1024,
			TrieTimeLimit: 2 * time.Minute,
			TriesInMemory: 128,
		}
		if sc.harmonyconfig != nil {
			cacheConfig.TriesInMemory = uint64(sc.harmonyconfig.General.TriesInMemory)
		}
	}

	chainConfig := sc.networkType.ChainConfig()

	if shardID == shard.BeaconChainShardID {
		// For beacon chain inside a shard chain, need to reset the eth chainID to shard 0's eth chainID in the config
		chainConfig.EthCompatibleChainID = big.NewInt(chainConfig.EthCompatibleShard0ChainID.Int64())
	}
	opts := core.Options{}
	if len(options) == 1 {
		opts = options[0]
	}
	var bc core.BlockChain
	if opts.EpochChain {
		bc, err = core.NewEpochChain(db, &chainConfig, vm.Config{})
	} else {
		stateCache, err := initStateCache(db, sc, shardID)
		if err != nil {
			return nil, err
		}
		if shardID == shard.BeaconChainShardID {
			bc, err = core.NewBlockChainWithOptions(
				db, stateCache, bc, cacheConfig, &chainConfig, chain.Engine(), vm.Config{}, opts,
			)
		} else {
			beacon, ok := sc.pool[shard.BeaconChainShardID]
			if !ok {
				return nil, errors.New("beacon chain is not initialized")
			}

			bc, err = core.NewBlockChainWithOptions(
				db, stateCache, beacon, cacheConfig, &chainConfig, chain.Engine(), vm.Config{}, opts,
			)
		}
	}

	if err != nil {
		return nil, errors.Wrapf(err, "cannot create blockchain")
	}
	db = nil // don't close
	sc.pool[shardID] = bc

	if sc.harmonyconfig != nil && sc.harmonyconfig.General.RunElasticMode {
		// init the tikv mode
		bc.InitTiKV(sc.harmonyconfig.TiKV)
	}

	return bc, nil
}

func initStateCache(db ethdb.Database, sc *CollectionImpl, shardID uint32) (state.Database, error) {
	if sc.harmonyconfig != nil && sc.harmonyconfig.General.RunElasticMode {
		// used for tikv mode, init state db using tikv storage
		stateDB, err := tikv_manage.GetDefaultTiKVFactory().NewStateDB(shardID)
		if err != nil {
			return nil, err
		}
		return state.NewDatabaseWithCache(stateDB, 64), nil
	} else {
		return state.NewDatabase(db), nil
	}
}

// DisableCache disables caching mode for newly opened chains.
// It does not affect already open chains.  For best effect,
// use this immediately after creating collection.
func (sc *CollectionImpl) DisableCache(shardID uint32) {
	if sc.disableCache == nil {
		sc.disableCache = make(map[uint32]bool)
	}
	sc.disableCache[shardID] = true
}

// CloseShardChain closes the given shard chain.
func (sc *CollectionImpl) CloseShardChain(shardID uint32) error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	bc, ok := sc.pool[shardID]
	if !ok {
		return errors.Errorf("shard chain not found %d", shardID)
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
func (sc *CollectionImpl) Close() error {
	newPool := make(map[uint32]core.BlockChain)
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
