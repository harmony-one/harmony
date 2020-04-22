package shardchain

import "github.com/harmony-one/harmony/ethdb"

// DBInitializer initializes a newly created chain database.
type DBInitializer interface {
	InitChainDB(db ethdb.Database, shardID uint32) error
}
