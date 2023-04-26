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
	"math/big"
	"time"

	lru "github.com/hashicorp/golang-lru"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

const (
	resultCacheLimit = 64 // The number of cached results from processing blocks
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config      *params.ChainConfig     // Chain configuration options
	bc          BlockChain              // Canonical blockchain
	beacon      BlockChain              // Beacon chain
	engine      consensus_engine.Engine // Consensus engine used for block rewards
	resultCache *lru.Cache              // Cache for result after a certain block is processed
}

// this structure is cached, and each individual element is returned
type ProcessorResult struct {
	Receipts   types.Receipts
	CxReceipts types.CXReceipts
	StakeMsgs  []staking.StakeMsg
	Logs       []*types.Log
	UsedGas    uint64
	Reward     reward.Reader
	State      *state.DB
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(
	config *params.ChainConfig, bc BlockChain, beacon BlockChain, engine consensus_engine.Engine,
) *StateProcessor {
	if bc == nil {
		panic("bc is nil")
	}
	if beacon == nil {
		panic("beacon is nil")
	}
	resultCache, _ := lru.New(resultCacheLimit)
	return &StateProcessor{
		config:      config,
		bc:          bc,
		beacon:      beacon,
		engine:      engine,
		resultCache: resultCache,
	}
}

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(
	block *types.Block, statedb *state.DB, cfg vm.Config, readCache bool,
) (
	types.Receipts, types.CXReceipts, []staking.StakeMsg,
	[]*types.Log, uint64, reward.Reader, *state.DB, error,
) {
	cacheKey := block.Hash()
	if readCache {
		if cached, ok := p.resultCache.Get(cacheKey); ok {
			// Return the cached result to avoid process the same block again.
			// Only the successful results are cached in case for retry.
			result := cached.(*ProcessorResult)
			utils.Logger().Info().Str("block num", block.Number().String()).Msg("result cache hit.")
			return result.Receipts, result.CxReceipts, result.StakeMsgs, result.Logs, result.UsedGas, result.Reward, result.State, nil
		}
	}

	var (
		receipts       types.Receipts
		outcxs         types.CXReceipts
		incxs          = block.IncomingReceipts()
		usedGas        = new(uint64)
		header         = block.Header()
		allLogs        []*types.Log
		gp                                = new(GasPool).AddGas(block.GasLimit())
		blockStakeMsgs []staking.StakeMsg = make([]staking.StakeMsg, 0)
	)

	beneficiary, err := p.bc.GetECDSAFromCoinbase(header)
	if err != nil {
		return nil, nil, nil, nil, 0, nil, statedb, err
	}

	startTime := time.Now()
	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, cxReceipt, stakeMsgs, _, err := ApplyTransaction(
			p.config, p.bc, &beneficiary, gp, statedb, header, tx, usedGas, cfg,
		)
		if err != nil {
			return nil, nil, nil, nil, 0, nil, statedb, err
		}
		receipts = append(receipts, receipt)
		if cxReceipt != nil {
			outcxs = append(outcxs, cxReceipt)
		}
		if len(stakeMsgs) > 0 {
			blockStakeMsgs = append(blockStakeMsgs, stakeMsgs...)
		}
		allLogs = append(allLogs, receipt.Logs...)
	}
	utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("Process Normal Txns")

	startTime = time.Now()
	// Iterate over and process the staking transactions
	L := len(block.Transactions())
	for i, tx := range block.StakingTransactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i+L)
		receipt, _, err := ApplyStakingTransaction(
			p.config, p.bc, &beneficiary, gp, statedb, header, tx, usedGas, cfg,
		)
		if err != nil {
			return nil, nil, nil, nil, 0, nil, statedb, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}
	utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("Process Staking Txns")

	// incomingReceipts should always be processed
	// after transactions (to be consistent with the block proposal)
	for _, cx := range block.IncomingReceipts() {
		if err := ApplyIncomingReceipt(
			p.config, statedb, header, cx,
		); err != nil {
			return nil, nil,
				nil, nil, 0, nil, statedb, errors.New("[Process] Cannot apply incoming receipts")
		}
	}

	slashes := slash.Records{}
	if s := header.Slashes(); len(s) > 0 {
		if err := rlp.DecodeBytes(s, &slashes); err != nil {
			return nil, nil, nil, nil, 0, nil, statedb, errors.New(
				"[Process] Cannot finalize block",
			)
		}
	}

	if err := MayTestnetShardReduction(p.bc, statedb, header); err != nil {
		return nil, nil, nil, nil, 0, nil, statedb, err
	}

	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	sigsReady := make(chan bool)
	go func() {
		// Block processing don't need to block on reward computation as in block proposal
		sigsReady <- true
	}()
	_, payout, err := p.engine.Finalize(
		p.bc,
		p.beacon,
		header, statedb, block.Transactions(),
		receipts, outcxs, incxs, block.StakingTransactions(), slashes, sigsReady, func() uint64 { return header.ViewID().Uint64() },
	)
	if err != nil {
		return nil, nil, nil, nil, 0, nil, statedb, errors.New("[Process] Cannot finalize block")
	}

	result := &ProcessorResult{
		Receipts:   receipts,
		CxReceipts: outcxs,
		StakeMsgs:  blockStakeMsgs,
		Logs:       allLogs,
		UsedGas:    *usedGas,
		Reward:     payout,
		State:      statedb,
	}
	p.resultCache.Add(cacheKey, result)
	return receipts, outcxs, blockStakeMsgs, allLogs, *usedGas, payout, statedb, nil
}

// CacheProcessorResult caches the process result on the cache key.
func (p *StateProcessor) CacheProcessorResult(cacheKey interface{}, result *ProcessorResult) {
	p.resultCache.Add(cacheKey, result)
}

// return true if it is valid
func getTransactionType(
	config *params.ChainConfig, header *block.Header, tx *types.Transaction,
) types.TransactionType {
	if header.ShardID() == tx.ShardID() &&
		(!config.AcceptsCrossTx(header.Epoch()) ||
			tx.ShardID() == tx.ToShardID()) {
		return types.SameShardTx
	}
	numShards := shard.Schedule.InstanceForEpoch(header.Epoch()).NumShards()
	// Assuming here all the shards are consecutive from 0 to n-1, n is total number of shards
	if tx.ShardID() != tx.ToShardID() &&
		header.ShardID() == tx.ShardID() &&
		tx.ToShardID() < numShards {
		return types.SubtractionOnly
	}
	return types.InvalidTx
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.DB, header *block.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, *types.CXReceipt, []staking.StakeMsg, uint64, error) {
	txType := getTransactionType(config, header, tx)
	if txType == types.InvalidTx {
		return nil, nil, nil, 0, errors.New("Invalid Transaction Type")
	}

	if txType != types.SameShardTx && !config.AcceptsCrossTx(header.Epoch()) {
		return nil, nil, nil, 0, errors.Errorf(
			"cannot handle cross-shard transaction until after epoch %v (now %v)",
			config.CrossTxEpoch, header.Epoch(),
		)
	}

	var signer types.Signer
	if tx.IsEthCompatible() {
		if !config.IsEthCompatible(header.Epoch()) {
			return nil, nil, nil, 0, errors.New("ethereum compatible transactions not supported at current epoch")
		}
		signer = types.NewEIP155Signer(config.EthCompatibleChainID)
	} else {
		signer = types.MakeSigner(config, header.Epoch())
	}
	msg, err := tx.AsMessage(signer)

	// skip signer err for additiononly tx
	if err != nil {
		return nil, nil, nil, 0, err
	}

	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, author)
	context.TxType = txType
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)
	// Apply the transaction to the current state (included in the env)
	result, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	// Update the state with pending changes
	var root []byte
	if config.IsS3(header.Epoch()) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsS3(header.Epoch())).Bytes()
	}
	*usedGas += result.UsedGas

	failedExe := result.VMErr != nil
	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failedExe, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}

	// Set the receipt logs and create a bloom for filtering
	if config.IsReceiptLog(header.Epoch()) {
		receipt.Logs = statedb.GetLogs(tx.Hash(), header.Number().Uint64(), header.Hash())
	}
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	var cxReceipt *types.CXReceipt
	// Do not create cxReceipt if EVM call failed
	if txType == types.SubtractionOnly && !failedExe {
		if vmenv.CXReceipt != nil {
			return nil, nil, nil, 0, errors.New("cannot have cross shard receipt via precompile and directly")
		}
		cxReceipt = &types.CXReceipt{
			TxHash:    tx.Hash(),
			From:      msg.From(),
			To:        msg.To(),
			ShardID:   tx.ShardID(),
			ToShardID: tx.ToShardID(),
			Amount:    msg.Value(),
		}
	} else {
		if !failedExe {
			if vmenv.CXReceipt != nil {
				cxReceipt = vmenv.CXReceipt
				// this tx.Hash needs to be the "original" tx.Hash
				// since, in effect, we have added
				// support for cross shard txs
				// to eth txs
				cxReceipt.TxHash = tx.HashByType()
			}
		} else {
			cxReceipt = nil
		}
	}

	return receipt, cxReceipt, vmenv.StakeMsgs, result.UsedGas, err
}

// ApplyStakingTransaction attempts to apply a staking transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the staking transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
// staking transaction will use the code field in the account to store the staking information
func ApplyStakingTransaction(
	config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.DB,
	header *block.Header, tx *staking.StakingTransaction, usedGas *uint64, cfg vm.Config) (receipt *types.Receipt, gas uint64, err error) {

	msg, err := StakingToMessage(tx, header.Number())
	if err != nil {
		return nil, 0, err
	}

	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, author)

	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)

	// Apply the transaction to the current state (included in the env)
	gas, err = ApplyStakingMessage(vmenv, msg, gp, bc)
	if err != nil {
		return nil, 0, err
	}

	// Update the state with pending changes
	var root []byte
	if config.IsS3(header.Epoch()) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsS3(header.Epoch())).Bytes()
	}
	*usedGas += gas
	receipt = types.NewReceipt(root, false, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas

	if config.IsReceiptLog(header.Epoch()) {
		receipt.Logs = statedb.GetLogs(tx.Hash(), header.Number().Uint64(), header.Hash())
		utils.Logger().Info().Interface("CollectReward", receipt.Logs)
	}

	return receipt, gas, nil
}

// ApplyIncomingReceipt will add amount into ToAddress in the receipt
func ApplyIncomingReceipt(
	config *params.ChainConfig, db *state.DB,
	header *block.Header, cxp *types.CXReceiptsProof,
) error {
	if cxp == nil {
		return nil
	}

	for _, cx := range cxp.Receipts {
		if cx == nil || cx.To == nil { // should not happend
			return errors.Errorf(
				"ApplyIncomingReceipts: Invalid incomingReceipt! %v", cx,
			)
		}
		utils.Logger().Info().Interface("receipt", cx).
			Msgf("ApplyIncomingReceipts: ADDING BALANCE %d", cx.Amount)

		if !db.Exist(*cx.To) {
			db.CreateAccount(*cx.To)
		}
		db.AddBalance(*cx.To, cx.Amount)
		db.IntermediateRoot(config.IsS3(header.Epoch()))
	}
	return nil
}

// StakingToMessage returns the staking transaction as a core.Message.
// requires a signer to derive the sender.
// put it here to avoid cyclic import
func StakingToMessage(
	tx *staking.StakingTransaction, blockNum *big.Int,
) (types.Message, error) {
	payload, err := tx.RLPEncodeStakeMsg()
	if err != nil {
		return types.Message{}, err
	}
	from, err := tx.SenderAddress()
	if err != nil {
		return types.Message{}, err
	}

	msg := types.NewStakingMessage(from, tx.Nonce(), tx.GasLimit(), tx.GasPrice(), payload, blockNum)
	stkType := tx.StakingType()
	if _, ok := types.StakingTypeMap[stkType]; !ok {
		return types.Message{}, staking.ErrInvalidStakingKind
	}
	msg.SetType(types.StakingTypeMap[stkType])
	return msg, nil
}

// MayTestnetShardReduction handles the change in the number of Shards. It will mark the affected validator as inactive.
// This function does not handle all cases, only for ShardNum from 4 to 2.
func MayTestnetShardReduction(bc ChainContext, statedb *state.DB, header *block.Header) error {
	isBeaconChain := header.ShardID() == shard.BeaconChainShardID
	isLastBlock := shard.Schedule.IsLastBlock(header.Number().Uint64())
	isTestnet := nodeconfig.GetDefaultConfig().GetNetworkType() == nodeconfig.Testnet
	if !(isTestnet && isBeaconChain && isLastBlock) {
		return nil
	}
	curInstance := shard.Schedule.InstanceForEpoch(header.Epoch())
	nextEpoch := big.NewInt(header.Epoch().Int64() + 1)
	nextInstance := shard.Schedule.InstanceForEpoch(nextEpoch)
	curNumShards := curInstance.NumShards()
	nextNumShards := nextInstance.NumShards()

	if curNumShards == nextNumShards {
		return nil
	}

	if curNumShards != 4 && nextNumShards != 2 {
		return errors.New("can only handle the reduction from 4 to 2")
	}
	addresses, err := bc.ReadValidatorList()
	if err != nil {
		return err
	}
	for _, address := range addresses {
		validator, err := statedb.ValidatorWrapper(address, true, false)
		if err != nil {
			return err
		}
		if validator.Status == effective.Inactive || validator.Status == effective.Banned {
			continue
		}
		for _, pubKey := range validator.SlotPubKeys {
			curShard := new(big.Int).Mod(pubKey.Big(), big.NewInt(int64(curNumShards))).Uint64()
			nextShard := new(big.Int).Mod(pubKey.Big(), big.NewInt(int64(nextNumShards))).Uint64()
			if curShard >= uint64(nextNumShards) || curShard != nextShard {
				validator.Status = effective.Inactive
				break
			}
		}
	}
	statedb.IntermediateRoot(bc.Config().IsS3(header.Epoch()))
	return nil
}
