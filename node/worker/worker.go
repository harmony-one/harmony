package worker

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"

	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/ctxerror"
)

// environment is the worker's current environment and holds all of the current state information.
type environment struct {
	state   *state.DB     // apply state changes here
	gasPool *core.GasPool // available gas used to pack transactions

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt
}

// Worker is the main object which takes care of submitting new work to consensus engine
// and gathering the sealing result.
type Worker struct {
	config  *params.ChainConfig
	chain   *core.BlockChain
	current *environment // An environment for current running cycle.

	coinbase common.Address
	engine   consensus_engine.Engine

	gasFloor uint64
	gasCeil  uint64

	shardID uint32
}

// SelectTransactionsForNewBlock selects transactions for new block.
func (w *Worker) SelectTransactionsForNewBlock(txs types.Transactions, maxNumTxs int) (types.Transactions, types.Transactions, types.Transactions) {
	if w.current.gasPool == nil {
		w.current.gasPool = new(core.GasPool).AddGas(w.current.header.GasLimit)
	}
	selected := types.Transactions{}
	unselected := types.Transactions{}
	invalid := types.Transactions{}
	for _, tx := range txs {
		if tx.ShardID() != w.shardID {
			invalid = append(invalid, tx)
		}
		snap := w.current.state.Snapshot()
		_, err := w.commitTransaction(tx, w.coinbase)
		if len(selected) > maxNumTxs {
			unselected = append(unselected, tx)
		} else {
			if err != nil {
				w.current.state.RevertToSnapshot(snap)
				invalid = append(invalid, tx)
				log.Debug("Invalid transaction", "Error", err)
			} else {
				selected = append(selected, tx)
			}
		}
	}
	err := w.UpdateCurrent()
	if err != nil {
		log.Debug("Failed updating worker's state", "Error", err)
	}
	return selected, unselected, invalid
}

func (w *Worker) commitTransaction(tx *types.Transaction, coinbase common.Address) ([]*types.Log, error) {
	snap := w.current.state.Snapshot()

	receipt, _, err := core.ApplyTransaction(w.config, w.chain, &coinbase, w.current.gasPool, w.current.state, w.current.header, tx, &w.current.header.GasUsed, vm.Config{})
	if err != nil {
		w.current.state.RevertToSnapshot(snap)
		return nil, err
	}
	w.current.txs = append(w.current.txs, tx)
	w.current.receipts = append(w.current.receipts, receipt)

	return receipt.Logs, nil
}

// CommitTransactions commits transactions.
func (w *Worker) CommitTransactions(txs types.Transactions) error {
	if w.current.gasPool == nil {
		w.current.gasPool = new(core.GasPool).AddGas(w.current.header.GasLimit)
	}
	for _, tx := range txs {
		snap := w.current.state.Snapshot()
		_, err := w.commitTransaction(tx, w.coinbase)
		if err != nil {
			w.current.state.RevertToSnapshot(snap)
			return err

		}
	}
	return nil
}

// UpdateCurrent updates the current environment with the current state and header.
func (w *Worker) UpdateCurrent() error {
	parent := w.chain.CurrentBlock()
	num := parent.Number()
	timestamp := time.Now().Unix()
	// New block's epoch is the same as parent's...
	epoch := new(big.Int).Set(parent.Header().Epoch)

	// TODO: Don't depend on sharding state for epoch change.
	if len(parent.Header().ShardState) > 0 && parent.NumberU64() != 0 {
		// ... except if parent has a resharding assignment it increases by 1.
		epoch = epoch.Add(epoch, common.Big1)
	}
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		GasLimit:   core.CalcGasLimit(parent, w.gasFloor, w.gasCeil),
		Time:       big.NewInt(timestamp),
		Epoch:      epoch,
		ShardID:    w.chain.ShardID(),
	}
	return w.makeCurrent(parent, header)
}

// makeCurrent creates a new environment for the current cycle.
func (w *Worker) makeCurrent(parent *types.Block, header *types.Header) error {
	state, err := w.chain.StateAt(parent.Root())
	if err != nil {
		return err
	}
	env := &environment{
		state:  state,
		header: header,
	}

	w.current = env
	return nil
}

// GetCurrentState gets the current state.
func (w *Worker) GetCurrentState() *state.DB {
	return w.current.state
}

// GetCurrentReceipts get the receipts generated starting from the last state.
func (w *Worker) GetCurrentReceipts() []*types.Receipt {
	return w.current.receipts
}

// Commit generate a new block for the new txs.
func (w *Worker) Commit(sig []byte, signers []byte, viewID uint32, addr common.Address) (*types.Block, error) {
	if len(sig) > 0 && len(signers) > 0 {
		copy(w.current.header.LastCommitSignature[:], sig[:])
		w.current.header.LastCommitBitmap = append(signers[:0:0], signers...)
	}
	w.current.header.Coinbase = addr
	w.current.header.ViewID = viewID

	s := w.current.state.Copy()
	block, err := w.engine.Finalize(w.chain, w.current.header, s, w.current.txs, w.current.receipts)
	if err != nil {
		return nil, ctxerror.New("cannot finalize block").WithCause(err)
	}
	return block, nil
}

// New create a new worker object.
func New(config *params.ChainConfig, chain *core.BlockChain, engine consensus_engine.Engine, coinbase common.Address, shardID uint32) *Worker {
	worker := &Worker{
		config: config,
		chain:  chain,
		engine: engine,
	}
	worker.gasFloor = 500000000000000000
	worker.gasCeil = 1000000000000000000
	worker.coinbase = coinbase
	worker.shardID = shardID

	parent := worker.chain.CurrentBlock()
	num := parent.Number()
	timestamp := time.Now().Unix()
	// New block's epoch is the same as parent's...
	epoch := new(big.Int).Set(parent.Header().Epoch)

	// TODO: Don't depend on sharding state for epoch change.
	if len(parent.Header().ShardState) > 0 && parent.NumberU64() != 0 {
		// ... except if parent has a resharding assignment it increases by 1.
		epoch = epoch.Add(epoch, common.Big1)
	}
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		GasLimit:   core.CalcGasLimit(parent, worker.gasFloor, worker.gasCeil),
		Time:       big.NewInt(timestamp),
		Epoch:      epoch,
		ShardID:    worker.chain.ShardID(),
	}
	worker.makeCurrent(parent, header)

	return worker
}
