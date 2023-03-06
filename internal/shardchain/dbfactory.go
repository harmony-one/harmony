package shardchain

import (
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/harmony-one/harmony/internal/shardchain/leveldb_shard"
	"github.com/harmony-one/harmony/internal/shardchain/local_cache"

	"github.com/harmony-one/harmony/core/rawdb"

	"github.com/ethereum/go-ethereum/ethdb"
)

const (
	LDBDirPrefix      = "harmony_db"
	LDBShardDirPrefix = "harmony_sharddb"
)

// DBFactory is a blockchain database factory.
type DBFactory interface {
	// NewChainDB returns a new database for the blockchain for
	// given shard.
	NewChainDB(shardID uint32) (ethdb.Database, error)
}

// LDBFactory is a LDB-backed blockchain database factory.
type LDBFactory struct {
	RootDir string // directory in which to put shard databases in.
}

// NewChainDB returns a new LDB for the blockchain for given shard.
func (f *LDBFactory) NewChainDB(shardID uint32) (ethdb.Database, error) {
	dir := path.Join(f.RootDir, fmt.Sprintf("%s_%d", LDBDirPrefix, shardID))
	return rawdb.NewLevelDBDatabase(dir, 256, 1024, "", false)
}

// MemDBFactory is a memory-backed blockchain database factory.
type MemDBFactory struct{}

// NewChainDB returns a new memDB for the blockchain for given shard.
func (f *MemDBFactory) NewChainDB(shardID uint32) (ethdb.Database, error) {
	return rawdb.NewMemoryDatabase(), nil
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
