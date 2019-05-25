package hmy

import (
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core"
)

// Harmony implements the Harmony full node service.
type Harmony struct {
	txPool     *core.TxPool
	blockchain *core.BlockChain

	// DB interfaces
	chainDb ethdb.Database // Block chain database

	APIBackend *APIBackend
}
