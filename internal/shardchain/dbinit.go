package shardchain

import (
	"github.com/ethereum/go-ethereum/ethdb"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

// DBInitializer initializes a newly created chain database.
type DBInitializer interface {
	InitChainDB(db ethdb.Database, networkType nodeconfig.NetworkType, shardID uint32) error
}
