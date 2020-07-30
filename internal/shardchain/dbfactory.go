package shardchain

import (
	"fmt"
	"path"

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
	return ethdb.NewLDBDatabase(dir, 128, 1024)
}

// MemDBFactory is a memory-backed blockchain database factory.
type MemDBFactory struct{}

// NewChainDB returns a new memDB for the blockchain for given shard.
func (f *MemDBFactory) NewChainDB(shardID uint32) (ethdb.Database, error) {
	return ethdb.NewMemDatabase(), nil
}
