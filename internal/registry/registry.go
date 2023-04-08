package registry

import (
	"sync"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/webhooks"
)

// Registry consolidates services at one place.
type Registry struct {
	mu          sync.Mutex
	blockchain  core.BlockChain
	beaconchain core.BlockChain
	webHooks    *webhooks.Hooks
	txPool      *core.TxPool
	cxPool      *core.CxPool
	isBackup    bool
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

// SetBeaconchain sets the beaconchain to registry.
func (r *Registry) SetBeaconchain(bc core.BlockChain) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.beaconchain = bc
	return r
}

// GetBeaconchain gets the beaconchain from registry.
func (r *Registry) GetBeaconchain() core.BlockChain {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.beaconchain
}

// SetWebHooks sets the webhooks to registry.
func (r *Registry) SetWebHooks(hooks *webhooks.Hooks) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.webHooks = hooks
	return r
}

// GetWebHooks gets the webhooks from registry.
func (r *Registry) GetWebHooks() *webhooks.Hooks {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.webHooks
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

func (r *Registry) SetIsBackup(isBackup bool) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.isBackup = isBackup
	return r
}

func (r *Registry) IsBackup() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.isBackup
}

// SetCxPool sets the cxpool to registry.
func (r *Registry) SetCxPool(cxPool *core.CxPool) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cxPool = cxPool
	return r
}

// GetCxPool gets the cxpool from registry.
func (r *Registry) GetCxPool() *core.CxPool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.cxPool
}
