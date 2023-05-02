package shardchain

import (
	"fmt"
	"io"
	"sync"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/tikv"
	tikvCommon "github.com/harmony-one/harmony/internal/tikv/common"
	"github.com/harmony-one/harmony/internal/tikv/prefix"
	"github.com/harmony-one/harmony/internal/tikv/remote"
	"github.com/harmony-one/harmony/internal/tikv/statedb_cache"

	"github.com/ethereum/go-ethereum/ethdb"
)

const (
	LDBTiKVPrefix = "harmony_tikv"
)

type TiKvCacheConfig struct {
	StateDBCacheSizeInMB        uint32
	StateDBCachePersistencePath string
	StateDBRedisServerAddr      string
	StateDBRedisLRUTimeInDay    uint32
}

// TiKvFactory is a memory-backed blockchain database factory.
type TiKvFactory struct {
	cacheDBMap sync.Map

	PDAddr      []string
	Role        string
	CacheConfig statedb_cache.StateDBCacheConfig
}

// getStateDB create statedb storage use tikv
func (f *TiKvFactory) getRemoteDB(shardID uint32) (*prefix.PrefixDatabase, error) {
	key := fmt.Sprintf("remote_%d_%s", shardID, f.Role)
	if db, ok := f.cacheDBMap.Load(key); ok {
		return db.(*prefix.PrefixDatabase), nil
	} else {
		prefixStr := []byte(fmt.Sprintf("%s_%d/", LDBTiKVPrefix, shardID))
		remoteDatabase, err := remote.NewRemoteDatabase(f.PDAddr, f.Role == tikv.RoleReader)
		if err != nil {
			return nil, err
		}

		tmpDB := prefix.NewPrefixDatabase(prefixStr, remoteDatabase)
		if loadedDB, loaded := f.cacheDBMap.LoadOrStore(key, tmpDB); loaded {
			_ = tmpDB.Close()
			return loadedDB.(*prefix.PrefixDatabase), nil
		}

		return tmpDB, nil
	}
}

// getStateDB create statedb storage with local memory cache.
func (f *TiKvFactory) getStateDB(shardID uint32) (*statedb_cache.StateDBCacheDatabase, error) {
	key := fmt.Sprintf("state_db_%d_%s", shardID, f.Role)
	if db, ok := f.cacheDBMap.Load(key); ok {
		return db.(*statedb_cache.StateDBCacheDatabase), nil
	} else {
		db, err := f.getRemoteDB(shardID)
		if err != nil {
			return nil, err
		}

		tmpDB, err := statedb_cache.NewStateDBCacheDatabase(db, f.CacheConfig, f.Role == tikv.RoleReader)
		if err != nil {
			return nil, err
		}

		if loadedDB, loaded := f.cacheDBMap.LoadOrStore(key, tmpDB); loaded {
			_ = tmpDB.Close()
			return loadedDB.(*statedb_cache.StateDBCacheDatabase), nil
		}

		return tmpDB, nil
	}
}

// NewChainDB returns a new memDB for the blockchain for given shard.
func (f *TiKvFactory) NewChainDB(shardID uint32) (ethdb.Database, error) {
	var database ethdb.KeyValueStore

	db, err := f.getRemoteDB(shardID)
	if err != nil {
		return nil, err
	}

	database = tikvCommon.ToEthKeyValueStore(db)

	return rawdb.NewDatabase(database), nil
}

// NewStateDB create shard tikv database
func (f *TiKvFactory) NewStateDB(shardID uint32) (ethdb.Database, error) {
	var database ethdb.KeyValueStore

	cacheDatabase, err := f.getStateDB(shardID)
	if err != nil {
		return nil, err
	}

	database = tikvCommon.ToEthKeyValueStore(cacheDatabase)

	return rawdb.NewDatabase(database), nil
}

// NewCacheStateDB create shard statedb storage with memory cache
func (f *TiKvFactory) NewCacheStateDB(shardID uint32) (*statedb_cache.StateDBCacheDatabase, error) {
	return f.getStateDB(shardID)
}

// CloseAllDB close all tikv database
func (f *TiKvFactory) CloseAllDB() {
	f.cacheDBMap.Range(func(_, value interface{}) bool {
		if closer, ok := value.(io.Closer); ok {
			_ = closer.Close()
		}
		return true
	})
}
