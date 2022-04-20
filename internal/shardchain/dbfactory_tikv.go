package shardchain

import (
	"fmt"
	"github.com/ethereum/go-ethereum/core/rawdb"
	tikvCommon "github.com/harmony-one/harmony/internal/tikv/common"
	"github.com/harmony-one/harmony/internal/tikv/prefix"
	"github.com/harmony-one/harmony/internal/tikv/remote"
	"github.com/harmony-one/harmony/internal/tikv/statedb_cache"
	"io"
	"sync"

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

func (f *TiKvFactory) getRemoteDB(shardID uint32) (*prefix.PrefixDatabase, error) {
	key := fmt.Sprintf("remote_%d_%s", shardID, f.Role)
	if db, ok := f.cacheDBMap.Load(key); ok {
		return db.(*prefix.PrefixDatabase), nil
	} else {
		prefixStr := []byte(fmt.Sprintf("%s_%d/", LDBTiKVPrefix, shardID))
		remoteDatabase, err := remote.NewRemoteDatabase(f.PDAddr, f.Role == "Reader")
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

func (f *TiKvFactory) getStateDB(shardID uint32) (*statedb_cache.StateDBCacheDatabase, error) {
	key := fmt.Sprintf("state_db_%d_%s", shardID, f.Role)
	if db, ok := f.cacheDBMap.Load(key); ok {
		return db.(*statedb_cache.StateDBCacheDatabase), nil
	} else {
		db, err := f.getRemoteDB(shardID)
		if err != nil {
			return nil, err
		}

		tmpDB, err := statedb_cache.NewStateDBCacheDatabase(db, f.CacheConfig, f.Role == "Reader")
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

// NewStateDB
func (f *TiKvFactory) NewStateDB(shardID uint32) (ethdb.Database, error) {
	var database ethdb.KeyValueStore

	cacheDatabase, err := f.getStateDB(shardID)
	if err != nil {
		return nil, err
	}

	database = tikvCommon.ToEthKeyValueStore(cacheDatabase)

	return rawdb.NewDatabase(database), nil
}

func (f *TiKvFactory) NewCacheStateDB(shardID uint32) (*statedb_cache.StateDBCacheDatabase, error) {
	return f.getStateDB(shardID)
}

func (f *TiKvFactory) CloseAllDB() {
	f.cacheDBMap.Range(func(_, value interface{}) bool {
		if closer, ok := value.(io.Closer); ok {
			_ = closer.Close()
		}
		return true
	})
}
