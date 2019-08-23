// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig     // Chain configuration options
	bc     *BlockChain             // Canonical block chain
	engine consensus_engine.Engine // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus_engine.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.DB, cfg vm.Config) (types.Receipts, types.CXReceipts, []*types.Log, uint64, error) {
	var (
		receipts types.Receipts
		outcxs   types.CXReceipts

		incxs    = block.IncomingReceipts()
		usedGas  = new(uint64)
		header   = block.Header()
		coinbase = block.Header().Coinbase
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit())
	)

	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, cxReceipt, _, err := ApplyTransaction(p.config, p.bc, &coinbase, gp, statedb, header, tx, usedGas, cfg)
		if err != nil {
			return nil, nil, nil, 0, err
		}
		receipts = append(receipts, receipt)
		if cxReceipt != nil {
			outcxs = append(outcxs, cxReceipt)
		}
		allLogs = append(allLogs, receipt.Logs...)
	}

	// incomingReceipts should always be processed after transactions (to be consistent with the block proposal)
	for _, cx := range block.IncomingReceipts() {
		ApplyIncomingReceipt(p.config, statedb, header, cx)
	}

	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	_, err := p.engine.Finalize(p.bc, header, statedb, block.Transactions(), receipts, outcxs, incxs)
	if err != nil {
		return nil, nil, nil, 0, ctxerror.New("cannot finalize block").WithCause(err)
	}

	return receipts, outcxs, allLogs, *usedGas, nil
}

// return true if it is valid
func getTransactionType(header *types.Header, tx *types.Transaction) types.TransactionType {
	if tx.ShardID() == tx.ToShardID() && header.ShardID == tx.ShardID() {
		return types.SameShardTx
	}
	if tx.ShardID() != tx.ToShardID() && header.ShardID == tx.ShardID() {
		return types.SubtractionOnly
	}
	return types.InvalidTx
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.DB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, *types.CXReceipt, uint64, error) {
	txType := getTransactionType(header, tx)
	if txType == types.InvalidTx {
		return nil, nil, 0, fmt.Errorf("Invalid Transaction Type")
	}
	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	// skip signer err for additiononly tx
	if err != nil {
		return nil, nil, 0, err
	}

	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, author)
	context.TxType = txType
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)
	// Apply the transaction to the current state (included in the env)
	_, gas, failed, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, nil, 0, err
	}
	// Update the state with pending changes
	var root []byte
	if config.IsByzantium(header.Number) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
	}
	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	//receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	var cxReceipt *types.CXReceipt
	if txType == types.SubtractionOnly {
		cxReceipt = &types.CXReceipt{tx.Hash(), msg.From(), msg.To(), tx.ShardID(), tx.ToShardID(), msg.Value()}
	} else {
		cxReceipt = nil
	}

	return receipt, cxReceipt, gas, err
}

// ApplyIncomingReceipt will add amount into ToAddress in the receipt
func ApplyIncomingReceipt(config *params.ChainConfig, db *state.DB, header *types.Header, cxp *types.CXReceiptsProof) {
	if cxp == nil {
		return
	}

	// TODO: how to charge gas here?
	for _, cx := range cxp.Receipts {
		if cx == nil || cx.To == nil { // should not happend
			utils.Logger().Warn().Msg("ApplyIncomingReceipts: Invalid incoming receipt!!")
			continue
		}
		utils.Logger().Info().Msgf("ApplyIncomingReceipts: ADDING BALANCE %d", cx.Amount)

		if !db.Exist(*cx.To) {
			db.CreateAccount(*cx.To)
		}
		db.AddBalance(*cx.To, cx.Amount)
		db.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
	}
}
