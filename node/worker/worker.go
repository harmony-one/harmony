package worker

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/common"
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

func (w *Worker) isMyTransaction(tx *types.Transaction) bool {
	msg, _ := tx.AsMessage(types.HomesteadSigner{})

	// {Address: "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8", Private: "3c8642f7188e05acc4467d9e2aa7fd539e82aa90a5497257cf0ecbb98ed3b88f", Public: "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8"},
	// {Address: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E", Private: "371cb68abe6a6101ac88603fc847e0c013a834253acee5315884d2c4e387ebca", Public: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E"},
	// curl 'http://127.0.0.1:30000/balance?key=0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8'

	return msg.From().Hex() == "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8"
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

		if w.isMyTransaction(tx) {
			fmt.Println("received SubmitTransaction")
		}
		if tx.ShardID() != w.shardID {
			invalid = append(invalid, tx)
			if w.isMyTransaction(tx) {
				fmt.Println("received SubmitTransaction but invalid because of different shardID")
			}
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
				if w.isMyTransaction(tx) {
					fmt.Println("received  and selected SubmitTransaction")
				}
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

	msg, _ := tx.AsMessage(types.HomesteadSigner{})

	// {Address: "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8", Private: "3c8642f7188e05acc4467d9e2aa7fd539e82aa90a5497257cf0ecbb98ed3b88f", Public: "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8"},
	// {Address: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E", Private: "371cb68abe6a6101ac88603fc847e0c013a834253acee5315884d2c4e387ebca", Public: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E"},
	// curl 'http://127.0.0.1:30000/balance?key=0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8'

	if msg.From().Hex() == "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8" {
		fmt.Println("SubmitTransaction received")
	}
	receipt, _, err := core.ApplyTransaction(w.config, w.chain, &coinbase, w.current.gasPool, w.current.state, w.current.header, tx, &w.current.header.GasUsed, vm.Config{})
	if err != nil {
		if msg.From().Hex() == "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8" {
			fmt.Println("error SubmitTransaction 4", "error", err)
		}
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
	if len(parent.Header().ShardState) > 0 {
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
func (w *Worker) Commit() (*types.Block, error) {
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
	if len(parent.Header().ShardState) > 0 {
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
