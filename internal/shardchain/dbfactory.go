package shardchain

import (
	"fmt"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/harmony-one/harmony/internal/shardchain/leveldb_shard"
	"github.com/harmony-one/harmony/internal/shardchain/local_cache"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/harmony-one/harmony/core"

	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/harmony-one/harmony/internal/mmr"
	"github.com/harmony-one/harmony/internal/mmr/db"
)

const (
	LDBDirPrefix      = "harmony_db"
	LDBShardDirPrefix = "harmony_sharddb"
	MMRDirPrefix      = "mmr_db"
)

// DBFactory is a blockchain database factory.
type DBFactory interface {
	// NewChainDB returns a new database for the blockchain for
	// given shard.
	NewChainDB(shardID uint32) (ethdb.Database, error)
	// MmrDB returns OpenMmrDB for the blockchain for given shard and epoch.
	MmrDB(shardID uint32) (core.OpenMmrDB, error)
}

func MmrDB(RootDir string, shardID uint32) (core.OpenMmrDB, error) {
	mmrDir := path.Join(RootDir, fmt.Sprintf("%s_%d", MMRDirPrefix, shardID))
	if fi, err := os.Stat(mmrDir); err == nil {
		if !fi.IsDir() {
			return nil, fmt.Errorf("MmrDB: open %s: not a directory", mmrDir)
		}
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(mmrDir, 0755); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	return func(epoch *big.Int, create bool) *mmr.Mmr {
		var fileDB *db.Filebaseddb
		dbPath := path.Join(mmrDir, epoch.String())
		if create {
			fileDB = db.CreateFilebaseddb(dbPath, 32) // 32 is byte length of keccak256
		} else {
			fileDB = db.OpenFilebaseddb(dbPath)
		}
		return mmr.New(
			fileDB,
			epoch,
		)
	}, nil
}

// LDBFactory is a LDB-backed blockchain database factory.
type LDBFactory struct {
	RootDir string // directory in which to put shard databases in.
}

// NewChainDB returns a new LDB for the blockchain for given shard.
func (f *LDBFactory) NewChainDB(shardID uint32) (ethdb.Database, error) {
	dir := path.Join(f.RootDir, fmt.Sprintf("%s_%d", LDBDirPrefix, shardID))
	return rawdb.NewLevelDBDatabase(dir, 256, 1024, "")
}

// MmrDB returns OpenMmrDB for the blockchain for given shard and epoch.
func (f *LDBFactory) MmrDB(shardID uint32) (core.OpenMmrDB, error) {
	return MmrDB(f.RootDir, shardID)
}

// MemDBFactory is a memory-backed blockchain database factory.
type MemDBFactory struct{}

// NewChainDB returns a new memDB for the blockchain for given shard.
func (f *MemDBFactory) NewChainDB(shardID uint32) (ethdb.Database, error) {
	return rawdb.NewMemoryDatabase(), nil
}

// MmrDB returns mem OpenMmrDB for the blockchain for given shard and epoch.
func (f *MemDBFactory) MmrDB(shardID uint32) (core.OpenMmrDB, error) {
	return func(epoch *big.Int, create bool) *mmr.Mmr {
		if !create {
			panic("not support")
		}
		return mmr.New(
			db.NewMemorybaseddb(0, map[int64][]byte{}),
			epoch,
		)
	}, nil
}

// LDBShardFactory is a merged Multi-LDB-backed blockchain database factory.
type LDBShardFactory struct {
	RootDir    string // directory in which to put shard databases in.
	DiskCount  int
	ShardCount int
	CacheTime  int
	CacheSize  int
}

// NewChainDB returns a new memDB for the blockchain for given shard.
func (f *LDBShardFactory) NewChainDB(shardID uint32) (ethdb.Database, error) {
	dir := filepath.Join(f.RootDir, fmt.Sprintf("%s_%d", LDBShardDirPrefix, shardID))
	shard, err := leveldb_shard.NewLeveldbShard(dir, f.DiskCount, f.ShardCount)
	if err != nil {
		return nil, err
	}

	return rawdb.NewDatabase(local_cache.NewLocalCacheDatabase(shard, local_cache.CacheConfig{
		CacheTime: time.Duration(f.CacheTime) * time.Minute,
		CacheSize: f.CacheSize,
	})), nil
}

// MmrDB returns OpenMmrDB for the blockchain for given shard and epoch.
func (f *LDBShardFactory) MmrDB(shardID uint32) (core.OpenMmrDB, error) {
	return MmrDB(f.RootDir, shardID)
}
