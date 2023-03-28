package shardchain

import "github.com/ethereum/go-ethereum/ethdb"

// DBInitializer initializes a newly created chain database.
type DBInitializer interface {
	InitChainDB(db ethdb.Database, shardID uint32) error
}
