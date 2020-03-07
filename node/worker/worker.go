package worker

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"
	"time"

	common2 "github.com/harmony-one/harmony/internal/common"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
)

// environment is the worker's current environment and holds all of the current state information.
type environment struct {
	signer     types.Signer
	state      *state.DB     // apply state changes here
	gasPool    *core.GasPool // available gas used to pack transactions
	header     *block.Header
	txs        []*types.Transaction
	stakingTxs []*staking.StakingTransaction
	receipts   []*types.Receipt
	outcxs     []*types.CXReceipt       // cross shard transaction receipts (source shard)
	incxs      []*types.CXReceiptsProof // cross shard receipts and its proof (desitinatin shard)
	slashes    slash.Records
}

// Worker is the main object which takes care of submitting new work to consensus engine
// and gathering the sealing result.
type Worker struct {
	config   *params.ChainConfig
	factory  blockfactory.Factory
	chain    *core.BlockChain
	current  *environment // An environment for current running cycle.
	engine   consensus_engine.Engine
	gasFloor uint64
	gasCeil  uint64
}

// CommitTransactions commits transactions for new block.
func (w *Worker) CommitTransactions(
	pendingNormal map[common.Address]types.Transactions,
	pendingStaking staking.StakingTransactions, coinbase common.Address,
) error {

	if w.current.gasPool == nil {
		w.current.gasPool = new(core.GasPool).AddGas(w.current.header.GasLimit())
	}

	txs := types.NewTransactionsByPriceAndNonce(w.current.signer, pendingNormal)
	coalescedLogs := []*types.Log{}
	// NORMAL
	for {
		// If we don't have enough gas for any further transactions then we're done
		if w.current.gasPool.Gas() < params.TxGas {
			utils.Logger().Info().Uint64("have", w.current.gasPool.Gas()).Uint64("want", params.TxGas).Msg("Not enough gas for further transactions")
			break
		}
		// Retrieve the next transaction and abort if all done
		tx := txs.Peek()
		if tx == nil {
			break
		}
		// Error may be ignored here. The error has already been checked
		// during transaction acceptance is the transaction pool.
		// We use the eip155 signer regardless of the current hf.
		from, _ := types.Sender(w.current.signer, tx)
		// Check whether the tx is replay protected. If we're not in the EIP155 hf
		// phase, start ignoring the sender until we do.
		if tx.Protected() && !w.config.IsEIP155(w.current.header.Number()) {
			utils.Logger().Info().Str("hash", tx.Hash().Hex()).Str("eip155Epoch", w.config.EIP155Epoch.String()).Msg("Ignoring reply protected transaction")
			txs.Pop()
			continue
		}
		// Start executing the transaction
		w.current.state.Prepare(tx.Hash(), common.Hash{}, len(w.current.txs))

		if tx.ShardID() != w.chain.ShardID() {
			txs.Shift()
			continue
		}

		logs, err := w.commitTransaction(tx, coinbase)

		sender, _ := common2.AddressToBech32(from)
		switch err {
		case core.ErrGasLimitReached:
			// Pop the current out-of-gas transaction without shifting in the next from the account
			utils.Logger().Info().Str("sender", sender).Msg("Gas limit exceeded for current block")
			txs.Pop()

		case core.ErrNonceTooLow:
			// New head notification data race between the transaction pool and miner, shift
			utils.Logger().Info().Str("sender", sender).Uint64("nonce", tx.Nonce()).Msg("Skipping transaction with low nonce")
			txs.Shift()

		case core.ErrNonceTooHigh:
			// Reorg notification data race between the transaction pool and miner, skip account =
			utils.Logger().Info().Str("sender", sender).Uint64("nonce", tx.Nonce()).Msg("Skipping account with high nonce")
			txs.Pop()

		case nil:
			// Everything ok, collect the logs and shift in the next transaction from the same account
			coalescedLogs = append(coalescedLogs, logs...)
			txs.Shift()

		default:
			// Strange error, discard the transaction and get the next in line (note, the
			// nonce-too-high clause will prevent us from executing in vain).
			utils.Logger().Info().Str("hash", tx.Hash().Hex()).AnErr("err", err).Msg("Transaction failed, account skipped")
			txs.Shift()
		}
	}

	// STAKING - only beaconchain process staking transaction
	if w.chain.ShardID() == shard.BeaconChainShardID {
		for _, tx := range pendingStaking {
			// TODO: merge staking transaction processing with normal transaction processing.
			// <<THESE CODE ARE DUPLICATED AS ABOVE
			// If we don't have enough gas for any further transactions then we're done
			if w.current.gasPool.Gas() < params.TxGas {
				utils.Logger().Info().Uint64("have", w.current.gasPool.Gas()).Uint64("want", params.TxGas).Msg("Not enough gas for further transactions")
				break
			}
			// Check whether the tx is replay protected. If we're not in the EIP155 hf
			// phase, start ignoring the sender until we do.
			if tx.Protected() && !w.config.IsEIP155(w.current.header.Number()) {
				utils.Logger().Info().Str("hash", tx.Hash().Hex()).Str("eip155Epoch", w.config.EIP155Epoch.String()).Msg("Ignoring reply protected transaction")
				txs.Pop()
				continue
			}

			// Start executing the transaction
			w.current.state.Prepare(tx.Hash(), common.Hash{}, len(w.current.txs))
			// THESE CODE ARE DUPLICATED AS ABOVE>>

			logs, err := w.commitStakingTransaction(tx, coinbase)
			if err != nil {
				txID := tx.Hash().Hex()
				utils.Logger().Error().Err(err).
					Str("stakingTxID", txID).
					Interface("stakingTx", tx).
					Msg("Failed committing staking transaction")
			} else {
				coalescedLogs = append(coalescedLogs, logs...)
				utils.Logger().Info().Str("stakingTxId", tx.Hash().Hex()).
					Uint64("txGasLimit", tx.Gas()).
					Msg("Successfully committed staking transaction")
			}
		}
	}

	utils.Logger().Info().
		Int("newTxns", len(w.current.txs)).
		Int("newStakingTxns", len(w.current.stakingTxs)).
		Uint64("blockGasLimit", w.current.header.GasLimit()).
		Uint64("blockGasUsed", w.current.header.GasUsed()).
		Msg("Block gas limit and usage info")
	return nil
}

func (w *Worker) commitStakingTransaction(
	tx *staking.StakingTransaction, coinbase common.Address,
) ([]*types.Log, error) {
	snap := w.current.state.Snapshot()
	gasUsed := w.current.header.GasUsed()
	receipt, _, err := core.ApplyStakingTransaction(
		w.config, w.chain, &coinbase, w.current.gasPool,
		w.current.state, w.current.header, tx, &gasUsed, vm.Config{},
	)
	w.current.header.SetGasUsed(gasUsed)
	if err != nil {
		w.current.state.RevertToSnapshot(snap)
		return nil, err
	}
	if receipt == nil {
		return nil, fmt.Errorf("nil staking receipt")
	}

	w.current.stakingTxs = append(w.current.stakingTxs, tx)
	w.current.receipts = append(w.current.receipts, receipt)
	return receipt.Logs, nil
}

func (w *Worker) commitTransaction(tx *types.Transaction, coinbase common.Address) ([]*types.Log, error) {
	snap := w.current.state.Snapshot()

	gasUsed := w.current.header.GasUsed()
	receipt, cx, _, err := core.ApplyTransaction(w.config, w.chain, &coinbase, w.current.gasPool, w.current.state, w.current.header, tx, &gasUsed, vm.Config{})
	w.current.header.SetGasUsed(gasUsed)
	if err != nil {
		w.current.state.RevertToSnapshot(snap)
		utils.Logger().Error().Err(err).Str("stakingTxId", tx.Hash().Hex()).Msg("Offchain ValidatorMap Read/Write Error")
		return nil, err
	}
	if receipt == nil {
		utils.Logger().Warn().Interface("tx", tx).Interface("cx", cx).Msg("Receipt is Nil!")
		return nil, fmt.Errorf("Receipt is Nil")
	}
	w.current.txs = append(w.current.txs, tx)
	w.current.receipts = append(w.current.receipts, receipt)
	if cx != nil {
		w.current.outcxs = append(w.current.outcxs, cx)
	}

	return receipt.Logs, nil
}

// CommitReceipts commits a list of already verified incoming cross shard receipts
func (w *Worker) CommitReceipts(receiptsList []*types.CXReceiptsProof) error {
	if w.current.gasPool == nil {
		w.current.gasPool = new(core.GasPool).AddGas(w.current.header.GasLimit())
	}

	if len(receiptsList) == 0 {
		w.current.header.SetIncomingReceiptHash(types.EmptyRootHash)
	} else {
		w.current.header.SetIncomingReceiptHash(
			types.DeriveSha(types.CXReceiptsProofs(receiptsList)),
		)
	}

	for _, cx := range receiptsList {
		err := core.ApplyIncomingReceipt(w.config, w.current.state, w.current.header, cx)
		if err != nil {
			return ctxerror.New("Failed applying cross-shard receipts").WithCause(err)
		}
	}

	for _, cx := range receiptsList {
		w.current.incxs = append(w.current.incxs, cx)
	}
	return nil
}

// UpdateCurrent updates the current environment with the current state and header.
func (w *Worker) UpdateCurrent() error {
	parent := w.chain.CurrentBlock()
	num := parent.Number()
	timestamp := time.Now().Unix()

	epoch := w.GetNewEpoch()
	header := w.factory.NewHeader(epoch).With().
		ParentHash(parent.Hash()).
		Number(num.Add(num, common.Big1)).
		GasLimit(core.CalcGasLimit(parent, w.gasFloor, w.gasCeil)).
		Time(big.NewInt(timestamp)).
		ShardID(w.chain.ShardID()).
		Header()
	return w.makeCurrent(parent, header)
}

// GetCurrentHeader returns the current header to propose
func (w *Worker) GetCurrentHeader() *block.Header {
	return w.current.header
}

// makeCurrent creates a new environment for the current cycle.
func (w *Worker) makeCurrent(parent *types.Block, header *block.Header) error {
	state, err := w.chain.StateAt(parent.Root())
	if err != nil {
		return err
	}
	env := &environment{
		signer: types.NewEIP155Signer(w.config.ChainID),
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

// GetNewEpoch gets the current epoch.
func (w *Worker) GetNewEpoch() *big.Int {
	parent := w.chain.CurrentBlock()
	epoch := new(big.Int).Set(parent.Header().Epoch())

	shardState, err := parent.Header().GetShardState()
	if err == nil &&
		shardState.Epoch != nil &&
		w.config.IsStaking(shardState.Epoch) {
		// For shard state of staking epochs, the shard state will
		// have an epoch and it will decide the next epoch for following blocks
		epoch = new(big.Int).Set(shardState.Epoch)
	} else {
		if len(parent.Header().ShardState()) > 0 && parent.NumberU64() != 0 {
			// if parent has proposed a new shard state it increases by 1, except for genesis block.
			epoch = epoch.Add(epoch, common.Big1)
		}
	}
	return epoch
}

// GetCurrentReceipts get the receipts generated starting from the last state.
func (w *Worker) GetCurrentReceipts() []*types.Receipt {
	return w.current.receipts
}

// OutgoingReceipts get the receipts generated starting from the last state.
func (w *Worker) OutgoingReceipts() []*types.CXReceipt {
	return w.current.outcxs
}

// IncomingReceipts get incoming receipts in destination shard that is received from source shard
func (w *Worker) IncomingReceipts() []*types.CXReceiptsProof {
	return w.current.incxs
}

// CollectAndVerifySlashes ..
func (w *Worker) CollectAndVerifySlashes() error {
	allSlashing, err := w.chain.ReadPendingSlashingCandidates()
	if err != nil {
		return err
	}
	if d := allSlashing; len(d) > 0 {
		// TODO add specific error which is
		// "could not verify slash", which should not return as err
		// and therefore stop the block proposal
		if allSlashing, err = w.VerifyAll(d); err != nil {
			// TODO(audit): very slashes individually; do not return err if verify fails
			utils.Logger().Err(err).
				RawJSON("slashes", []byte(d.String())).
				Msg("could not verify slashes proposed")
			return err
		}
	}
	w.current.slashes = allSlashing
	return nil
}

// VerifyAll ..
func (w *Worker) VerifyAll(allSlashing []slash.Record) ([]slash.Record, error) {
	d := allSlashing
	slashingToPropose := []slash.Record{}
	// Enforce order, reproducibility
	sort.SliceStable(d,
		func(i, j int) bool {
			return bytes.Compare(
				d[i].Reporter.Bytes(), d[j].Reporter.Bytes(),
			) == -1
		},
	)

	for i := range d {
		if err := slash.Verify(w.chain, &d[i]); err != nil {
			return nil, err
		}
		slashingToPropose = append(slashingToPropose, d[i])
	}
	count := len(slashingToPropose)
	utils.Logger().Info().
		Msgf("set into propose headers %d slashing record", count)
	return slashingToPropose, nil
}

// FinalizeNewBlock generate a new block for the next consensus round.
func (w *Worker) FinalizeNewBlock(
	sig []byte, signers []byte, viewID uint64, coinbase common.Address,
	crossLinks types.CrossLinks, shardState *shard.State,
) (*types.Block, error) {
	// Put sig, signers, viewID, coinbase into header
	if len(sig) > 0 && len(signers) > 0 {
		// TODO: directly set signature into lastCommitSignature
		sig2 := w.current.header.LastCommitSignature()
		copy(sig2[:], sig[:])
		w.current.header.SetLastCommitSignature(sig2)
		w.current.header.SetLastCommitBitmap(signers)
	}
	w.current.header.SetCoinbase(coinbase)
	w.current.header.SetViewID(new(big.Int).SetUint64(viewID))

	// Put crosslinks into header
	if len(crossLinks) > 0 {
		crossLinks.Sort()
		crossLinkData, err := rlp.EncodeToBytes(crossLinks)
		if err == nil {
			utils.Logger().Debug().
				Uint64("blockNum", w.current.header.Number().Uint64()).
				Int("numCrossLinks", len(crossLinks)).
				Msg("Successfully proposed cross links into new block")
			w.current.header.SetCrossLinks(crossLinkData)
		} else {
			utils.Logger().Debug().Err(err).Msg("Failed to encode proposed cross links")
			return nil, err
		}
	} else {
		utils.Logger().Debug().Msg("Zero crosslinks to finalize")
	}

	// Put slashes into header
	if w.config.IsStaking(w.current.header.Epoch()) {
		doubleSigners := w.current.slashes
		if len(doubleSigners) > 0 {
			if data, err := rlp.EncodeToBytes(doubleSigners); err == nil {
				w.current.header.SetSlashes(data)
				utils.Logger().Info().
					RawJSON("slashes", []byte(doubleSigners.String())).
					Msg("encoded slashes into headers of proposed new block")
			} else {
				utils.Logger().Debug().Err(err).Msg("Failed to encode proposed slashes")
				return nil, err
			}
		}
	}

	// Put shard state into header
	if shardState != nil && len(shardState.Shards) != 0 {
		//we store shardstatehash in header only before prestaking epoch (header v0,v1,v2)
		if !w.config.IsPreStaking(w.current.header.Epoch()) {
			w.current.header.SetShardStateHash(shardState.Hash())
		}
		isStaking := false
		if shardState.Epoch != nil && w.config.IsStaking(shardState.Epoch) {
			isStaking = true
		}
		// NOTE: Besides genesis, this is the only place where the shard state is encoded.
		shardStateData, err := shard.EncodeWrapper(*shardState, isStaking)
		if err == nil {
			w.current.header.SetShardState(shardStateData)
		} else {
			utils.Logger().Debug().Err(err).Msg("Failed to encode proposed shard state")
			return nil, err
		}
	}

	state := w.current.state.Copy()
	copyHeader := types.CopyHeader(w.current.header)
	// TODO: feed coinbase into here so the proposer gets extra rewards.
	block, _, err := w.engine.Finalize(
		w.chain, copyHeader, state, w.current.txs, w.current.receipts,
		w.current.outcxs, w.current.incxs, w.current.stakingTxs,
		w.current.slashes,
	)
	if err != nil {
		return nil, ctxerror.New("cannot finalize block").WithCause(err)
	}

	return block, nil
}

// New create a new worker object.
func New(config *params.ChainConfig, chain *core.BlockChain, engine consensus_engine.Engine) *Worker {
	worker := &Worker{
		config:  config,
		factory: blockfactory.NewFactory(config),
		chain:   chain,
		engine:  engine,
	}
	worker.gasFloor = 80000000
	worker.gasCeil = 120000000

	parent := worker.chain.CurrentBlock()
	num := parent.Number()
	timestamp := time.Now().Unix()

	epoch := worker.GetNewEpoch()
	header := worker.factory.NewHeader(epoch).With().
		ParentHash(parent.Hash()).
		Number(num.Add(num, common.Big1)).
		GasLimit(core.CalcGasLimit(parent, worker.gasFloor, worker.gasCeil)).
		Time(big.NewInt(timestamp)).
		ShardID(worker.chain.ShardID()).
		Header()
	worker.makeCurrent(parent, header)

	return worker
}
