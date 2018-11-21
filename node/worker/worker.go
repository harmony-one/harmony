package worker

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"math/big"
	"time"
)

// environment is the worker's current environment and holds all of the current state information.
type environment struct {
	state   *state.StateDB // apply state changes here
	gasPool *core.GasPool  // available gas used to pack transactions

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt
}

// worker is the main object which takes care of submitting new work to consensus engine
// and gathering the sealing result.
type Worker struct {
	config  *params.ChainConfig
	chain   *core.BlockChain
	current *environment // An environment for current running cycle.

	gasFloor uint64
	gasCeil  uint64
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

func (w *Worker) CommitTransactions(txs []*types.Transaction, coinbase common.Address) {
	for _, tx := range txs {
		w.commitTransaction(tx, coinbase)
	}
}

// makeCurrent creates a new environment for the current cycle.
func (w *Worker) makeCurrent(parent *types.Block, header *types.Header) error {
	state, err := w.chain.StateAt(parent.Root())
	if err != nil {
		return err
	}
	env := &environment{
		state:     state,
		header:    header,
	}

	w.current = env
	return nil
}

func New(config *params.ChainConfig, chain *core.BlockChain) *Worker {
	worker := &Worker{
		config:             config,
		chain:              chain,
	}
	worker.gasFloor = 0
	worker.gasCeil = 10000000
	
	parent := worker.chain.CurrentBlock()
	num := parent.Number()
	timestamp := time.Now().Unix()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		GasLimit:   core.CalcGasLimit(parent, worker.gasFloor, worker.gasCeil),
		Time:       big.NewInt(timestamp),
	}
	worker.makeCurrent(parent, header)

	return worker
}