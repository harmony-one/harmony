package chainread

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/state/snapshot"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

// UnexpectedCallError is panicked by every MinimalChainReader method outside
// the audited set. The pipeline converts it to exit code 2: an upstream
// internal/chain change that starts calling more ChainReader methods must
// fail closed, never silently return junk.
type UnexpectedCallError struct {
	Method string
}

func (e *UnexpectedCallError) Error() string {
	return fmt.Sprintf("minimal ChainReader: unexpected method call %s (fail-closed; the certificate verification path changed upstream)", e.Method)
}

// MinimalChainReader implements engine.ChainReader with exactly the state
// the certificate check needs: the chain config, the pinned target header
// (as CurrentHeader), the shard ID, and the walk-authenticated shard state
// for the target epoch. BlockChainImpl is never constructed (its open path
// calls loadLastState, which can reset or repair the chain on disk).
type MinimalChainReader struct {
	config     *params.ChainConfig
	shardID    uint32
	current    *block.Header
	epoch      *big.Int
	shardState *shard.State

	mu     sync.Mutex
	called map[string]int
}

// NewMinimalChainReader builds the reader from the outcome of the chain
// checks: header is the hash-verified target header and ss the
// byte-equality-authenticated shard state for the target epoch.
func NewMinimalChainReader(config *params.ChainConfig, shardID uint32, header *block.Header, epoch *big.Int, ss *shard.State) *MinimalChainReader {
	return &MinimalChainReader{
		config:     config,
		shardID:    shardID,
		current:    header,
		epoch:      epoch,
		shardState: ss,
		called:     make(map[string]int),
	}
}

var _ engine.ChainReader = (*MinimalChainReader)(nil)

func (r *MinimalChainReader) record(method string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.called[method]++
}

// CalledMethods returns the set of methods exercised so far (pin test).
func (r *MinimalChainReader) CalledMethods() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]int, len(r.called))
	for k, v := range r.called {
		out[k] = v
	}
	return out
}

// Config is in the audited set.
func (r *MinimalChainReader) Config() *params.ChainConfig {
	r.record("Config")
	return r.config
}

// CurrentHeader is in the audited set (the engine's Number()<=1 guard).
func (r *MinimalChainReader) CurrentHeader() *block.Header {
	r.record("CurrentHeader")
	return r.current
}

// ShardID is in the audited set.
func (r *MinimalChainReader) ShardID() uint32 {
	r.record("ShardID")
	return r.shardID
}

// ReadShardState is in the audited set; only the target epoch is served.
func (r *MinimalChainReader) ReadShardState(epoch *big.Int) (*shard.State, error) {
	r.record("ReadShardState")
	if epoch == nil || r.epoch.Cmp(epoch) != 0 {
		return nil, fmt.Errorf("minimal ChainReader: shard state requested for epoch %s, only epoch %s is available", epoch, r.epoch)
	}
	return r.shardState, nil
}

// --- everything below is outside the audited set and fails closed ---

func (r *MinimalChainReader) unexpected(method string) {
	panic(&UnexpectedCallError{Method: method})
}

func (r *MinimalChainReader) TrieDB() *trie.Database {
	r.unexpected("TrieDB")
	return nil
}

func (r *MinimalChainReader) TrieNode(hash common.Hash) ([]byte, error) {
	r.unexpected("TrieNode")
	return nil, nil
}

func (r *MinimalChainReader) ContractCode(hash common.Hash) ([]byte, error) {
	r.unexpected("ContractCode")
	return nil, nil
}

func (r *MinimalChainReader) ValidatorCode(hash common.Hash) ([]byte, error) {
	r.unexpected("ValidatorCode")
	return nil, nil
}

func (r *MinimalChainReader) GetReceiptsByHash(hash common.Hash) types.Receipts {
	r.unexpected("GetReceiptsByHash")
	return nil
}

func (r *MinimalChainReader) GetHeader(hash common.Hash, number uint64) *block.Header {
	r.unexpected("GetHeader")
	return nil
}

func (r *MinimalChainReader) GetHeaderByNumber(number uint64) *block.Header {
	r.unexpected("GetHeaderByNumber")
	return nil
}

func (r *MinimalChainReader) GetHeaderByHash(hash common.Hash) *block.Header {
	r.unexpected("GetHeaderByHash")
	return nil
}

func (r *MinimalChainReader) GetBlock(hash common.Hash, number uint64) *types.Block {
	r.unexpected("GetBlock")
	return nil
}

func (r *MinimalChainReader) Snapshots() *snapshot.Tree {
	r.unexpected("Snapshots")
	return nil
}

func (r *MinimalChainReader) ReadValidatorList() ([]common.Address, error) {
	r.unexpected("ReadValidatorList")
	return nil, nil
}

func (r *MinimalChainReader) CurrentBlock() *types.Block {
	r.unexpected("CurrentBlock")
	return nil
}

func (r *MinimalChainReader) StateAt(root common.Hash) (*state.DB, error) {
	r.unexpected("StateAt")
	return nil, nil
}

func (r *MinimalChainReader) ReadValidatorInformation(addr common.Address) (*staking.ValidatorWrapper, error) {
	r.unexpected("ReadValidatorInformation")
	return nil, nil
}

func (r *MinimalChainReader) ReadValidatorInformationAtState(addr common.Address, state *state.DB) (*staking.ValidatorWrapper, error) {
	r.unexpected("ReadValidatorInformationAtState")
	return nil, nil
}

func (r *MinimalChainReader) ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorSnapshot, error) {
	r.unexpected("ReadValidatorSnapshot")
	return nil, nil
}

func (r *MinimalChainReader) ValidatorCandidates() []common.Address {
	r.unexpected("ValidatorCandidates")
	return nil
}

func (r *MinimalChainReader) ReadValidatorSnapshotAtEpoch(epoch *big.Int, addr common.Address) (*staking.ValidatorSnapshot, error) {
	r.unexpected("ReadValidatorSnapshotAtEpoch")
	return nil, nil
}

func (r *MinimalChainReader) ReadBlockRewardAccumulator(number uint64) (*big.Int, error) {
	r.unexpected("ReadBlockRewardAccumulator")
	return nil, nil
}

func (r *MinimalChainReader) ReadValidatorStats(addr common.Address) (*staking.ValidatorStats, error) {
	r.unexpected("ReadValidatorStats")
	return nil, nil
}

func (r *MinimalChainReader) SuperCommitteeForNextEpoch(beacon engine.ChainReader, header *block.Header, isVerify bool) (*shard.State, error) {
	r.unexpected("SuperCommitteeForNextEpoch")
	return nil, nil
}

func (r *MinimalChainReader) ReadCommitSig(blockNum uint64) ([]byte, error) {
	r.unexpected("ReadCommitSig")
	return nil, nil
}
