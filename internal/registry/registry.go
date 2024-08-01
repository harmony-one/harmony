package registry

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/node/harmony/worker"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/webhooks"
)

// Registry consolidates services at one place.
type Registry struct {
	mu              sync.Mutex
	blockchain      core.BlockChain
	beaconchain     core.BlockChain
	webHooks        *webhooks.Hooks
	txPool          *core.TxPool
	cxPool          *core.CxPool
	isBackup        bool
	engine          engine.Engine
	collection      *shardchain.CollectionImpl
	nodeConfig      *nodeconfig.ConfigType
	addressToBLSKey AddressToBLSKey
	worker          *worker.Worker
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

// SetEngine sets the engine to registry.
func (r *Registry) SetEngine(engine engine.Engine) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.engine = engine
	return r
}

// GetEngine gets the engine from registry.
func (r *Registry) GetEngine() engine.Engine {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.engine
}

// SetShardChainCollection sets the shard chain collection to registry.
func (r *Registry) SetShardChainCollection(collection *shardchain.CollectionImpl) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.collection = collection
	return r
}

// GetShardChainCollection gets the shard chain collection from registry.
func (r *Registry) GetShardChainCollection() *shardchain.CollectionImpl {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.collection
}

func (r *Registry) SetNodeConfig(n *nodeconfig.ConfigType) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodeConfig = n
	return r
}

func (r *Registry) GetNodeConfig() *nodeconfig.ConfigType {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.nodeConfig
}

func (r *Registry) GetAddressToBLSKey() AddressToBLSKey {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.addressToBLSKey
}

func (r *Registry) SetAddressToBLSKey(a AddressToBLSKey) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.addressToBLSKey = a
	return r
}

// SetWorker sets the worker to registry.
func (r *Registry) SetWorker(w *worker.Worker) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.worker = w
	return r
}

// GetWorker gets the worker from registry.
func (r *Registry) GetWorker() *worker.Worker {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.worker
}

type FindCommitteeByID interface {
	FindCommitteeByID(shardID uint32) (*shard.Committee, error)
}

type AddressToBLSKey interface {
	GetAddressForBLSKey(publicKeys multibls.PublicKeys, shardState FindCommitteeByID, blskey *bls_core.PublicKey, epoch *big.Int) common.Address
	GetAddresses(publicKeys multibls.PublicKeys, shardState FindCommitteeByID, epoch *big.Int) map[string]common.Address
}
