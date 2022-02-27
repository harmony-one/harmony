package shardchain

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/harmony-one/harmony/internal/shardchain/leveldb_shard"
	"github.com/harmony-one/harmony/internal/shardchain/local_cache"

	"github.com/ethereum/go-ethereum/core/rawdb"

	"github.com/ethereum/go-ethereum/ethdb"
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
	dir := path.Join(f.RootDir, fmt.Sprintf("harmony_db_%d", shardID))
	return rawdb.NewLevelDBDatabase(dir, 256, 1024, "")
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
}

// NewChainDB returns a new memDB for the blockchain for given shard.
func (f *LDBShardFactory) NewChainDB(shardID uint32) (ethdb.Database, error) {
	dir := filepath.Join(f.RootDir, fmt.Sprintf("harmony_sharddb_%d", shardID))
	shard, err := leveldb_shard.NewLeveldbShard(dir, f.DiskCount, f.ShardCount)
	if err != nil {
		return nil, err
	}

	return rawdb.NewDatabase(local_cache.NewLocalCacheDatabase(shard)), nil
}
