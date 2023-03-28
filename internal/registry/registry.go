package registry

import (
	"sync"

	"github.com/harmony-one/harmony/core"
)

// Registry consolidates services at one place.
type Registry struct {
	mu         sync.Mutex
	blockchain core.BlockChain
	txPool     *core.TxPool
}

// New creates a new registry.
func New() *Registry {
	return &Registry{}
}

// SetBlockchain sets the blockchain to registry.
func (r *Registry) SetBlockchain(bc core.BlockChain) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.blockchain = bc
	return r
}

// GetBlockchain gets the blockchain from registry.
func (r *Registry) GetBlockchain() core.BlockChain {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.blockchain
}

// SetTxPool sets the txpool to registry.
func (r *Registry) SetTxPool(txPool *core.TxPool) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.txPool = txPool
	return r
}

// GetTxPool gets the txpool from registry.
func (r *Registry) GetTxPool() *core.TxPool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.txPool
}
