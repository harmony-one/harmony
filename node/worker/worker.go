package worker

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/harmony-one/harmony/crypto/hash"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// environment is the worker's current environment and holds all of the current state information.
type environment struct {
	signer     types.Signer
	ethSigner  types.Signer
	state      *state.DB     // apply state changes here
	gasPool    *core.GasPool // available gas used to pack transactions
	header     *block.Header
	txs        []*types.Transaction
	stakingTxs []*staking.StakingTransaction
	receipts   []*types.Receipt
	logs       []*types.Log
	reward     reward.Reader
	outcxs     []*types.CXReceipt       // cross shard transaction receipts (source shard)
	incxs      []*types.CXReceiptsProof // cross shard receipts and its proof (desitinatin shard)
	slashes    slash.Records
	stakeMsgs  []staking.StakeMsg
}

func (env *environment) CurrentHeader() *block.Header {
	return env.header
}

// Worker is the main object which takes care of submitting new work to consensus engine
// and gathering the sealing result.
type Worker struct {
	config   *params.ChainConfig
	factory  blockfactory.Factory
	chain    core.BlockChain
	beacon   core.BlockChain
	current  *environment // An environment for current running cycle.
	gasFloor uint64
	gasCeil  uint64
}

// New create a new worker object.
func New(
	chain core.BlockChain, beacon core.BlockChain,
) *Worker {
	worker := newWorker(chain.Config(), chain, beacon)

	parent := chain.CurrentBlock().Header()
	num := parent.Number()
	timestamp := time.Now().Unix()

	epoch := GetNewEpoch(chain)
	header := blockfactory.NewFactory(chain.Config()).NewHeader(epoch).With().
		ParentHash(parent.Hash()).
		Number(num.Add(num, common.Big1)).
		GasLimit(worker.GasFloor(epoch)). //core.CalcGasLimit(parent, worker.gasFloor, worker.gasCeil)).
		Time(big.NewInt(timestamp)).
		ShardID(chain.ShardID()).
		Header()
	worker.makeCurrent(parent, header)

	return worker
}

func newWorker(config *params.ChainConfig, chain, beacon core.BlockChain) *Worker {
	return &Worker{
		config:   config,
		factory:  blockfactory.NewFactory(config),
		chain:    chain,
		beacon:   beacon,
		gasFloor: 80000000,
		gasCeil:  120000000,
	}
}

// CommitSortedTransactions commits transactions for new block.
func (w *Worker) CommitSortedTransactions(
	txs *types.TransactionsByPriceAndNonce,
	coinbase common.Address,
) {
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
		signer := w.current.signer
		if tx.IsEthCompatible() {
			signer = w.current.ethSigner
		}
		from, _ := types.Sender(signer, tx)
		// Check whether the tx is replay protected. If we're not in the EIP155 hf
		// phase, start ignoring the sender until we do.
		if tx.Protected() && !w.chain.Config().IsEIP155(w.current.header.Epoch()) {
			utils.Logger().Info().Str("hash", tx.Hash().Hex()).Str("eip155Epoch", w.config.EIP155Epoch.String()).Msg("Ignoring reply protected transaction")
			txs.Pop()
			continue
		}

		if tx.ShardID() != w.chain.ShardID() {
			txs.Shift()
			continue
		}

		// Start executing the transaction
		w.current.state.Prepare(tx.Hash(), common.Hash{}, len(w.current.txs))
		err := w.commitTransaction(tx, coinbase)

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
			txs.Shift()

		default:
			// Strange error, discard the transaction and get the next in line (note, the
			// nonce-too-high clause will prevent us from executing in vain).
			utils.Logger().Info().Str("hash", tx.Hash().Hex()).AnErr("err", err).Msg("Transaction failed, account skipped")
			txs.Shift()
		}
	}
}

// CommitTransactions commits transactions for new block.
func (w *Worker) CommitTransactions(
	pendingNormal map[common.Address]types.Transactions,
	pendingStaking staking.StakingTransactions, coinbase common.Address,
) error {
	if w.current.gasPool == nil {
		w.current.gasPool = new(core.GasPool).AddGas(w.current.header.GasLimit())
	}

	// if this is epoch for balance migration, no txs (or stxs)
	// will be included in the block
	// it is technically feasible for some to end up in the pool
	// say, from the last epoch, but those will not be executed
	// and no balance will be lost
	// any cross-shard transfers destined to a shard being shut down
	// will execute (since they are already spent on the source shard)
	// but the balance will immediately be returned to shard 1
	cx, err := core.MayBalanceMigration(
		w.current.gasPool,
		w.GetCurrentHeader(),
		w.current.state,
		w.chain,
	)
	if err != nil {
		if errors.Is(err, core.ErrNoMigrationPossible) {
			// means we do not accept transactions from the network
			return nil
		}
		if !errors.Is(err, core.ErrNoMigrationRequired) {

			// this shard not migrating => ErrNoMigrationRequired
			// any other error means exit this block
			return err
		}
	} else {
		if cx != nil {
			w.current.outcxs = append(w.current.outcxs, cx)
			return nil
		}
	}

	// HARMONY TXNS
	normalTxns := types.NewTransactionsByPriceAndNonce(w.current.signer, w.current.ethSigner, pendingNormal)

	w.CommitSortedTransactions(normalTxns, coinbase)

	// STAKING - only beaconchain process staking transaction
	if w.chain.ShardID() == shard.BeaconChainShardID {
		for _, tx := range pendingStaking {
			// If we don't have enough gas for any further transactions then we're done
			if w.current.gasPool.Gas() < params.TxGas {
				utils.Logger().Info().Uint64("have", w.current.gasPool.Gas()).Uint64("want", params.TxGas).Msg("Not enough gas for further transactions")
				break
			}
			// Check whether the tx is replay protected. If we're not in the EIP155 hf
			// phase, start ignoring the sender until we do.
			if tx.Protected() && !w.config.IsEIP155(w.current.header.Epoch()) {
				utils.Logger().Info().Str("hash", tx.Hash().Hex()).Str("eip155Epoch", w.config.EIP155Epoch.String()).Msg("Ignoring reply protected transaction")
				continue
			}

			// Start executing the transaction
			w.current.state.Prepare(tx.Hash(), common.Hash{}, len(w.current.txs)+len(w.current.stakingTxs))
			// THESE CODE ARE DUPLICATED AS ABOVE>>
			if err := w.commitStakingTransaction(tx, coinbase); err != nil {
				txID := tx.Hash().Hex()
				utils.Logger().Error().Err(err).
					Str("stakingTxID", txID).
					Interface("stakingTx", tx).
					Msg("Failed committing staking transaction")
			} else {
				utils.Logger().Info().Str("stakingTxId", tx.Hash().Hex()).
					Uint64("txGasLimit", tx.GasLimit()).
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
) error {
	snap := w.current.state.Snapshot()
	gasUsed := w.current.header.GasUsed()
	receipt, _, err := core.ApplyStakingTransaction(
		w.chain, &coinbase, w.current.gasPool,
		w.current.state, w.current.header, tx, &gasUsed, vm.Config{},
	)
	w.current.header.SetGasUsed(gasUsed)
	if err != nil {
		w.current.state.RevertToSnapshot(snap)
		utils.Logger().Error().
			Err(err).Interface("stkTxn", tx).
			Msg("Staking transaction failed commitment")
		return err
	}
	if receipt == nil {
		return fmt.Errorf("nil staking receipt")
	}

	w.current.stakingTxs = append(w.current.stakingTxs, tx)
	w.current.receipts = append(w.current.receipts, receipt)
	w.current.logs = append(w.current.logs, receipt.Logs...)

	return nil
}

// ApplyShardReduction only used to reduce shards of Testnet
func (w *Worker) ApplyShardReduction() {
	core.MayShardReduction(w.chain, w.current.state, w.current.header)
}

var (
	errNilReceipt = errors.New("nil receipt")
)

func (w *Worker) commitTransaction(
	tx *types.Transaction, coinbase common.Address,
) error {
	snap := w.current.state.Snapshot()
	gasUsed := w.current.header.GasUsed()
	receipt, cx, stakeMsgs, _, err := core.ApplyTransaction(
		w.chain,
		&coinbase,
		w.current.gasPool,
		w.current.state,
		w.current.header,
		tx,
		&gasUsed,
		vm.Config{},
	)
	w.current.header.SetGasUsed(gasUsed)
	if err != nil {
		w.current.state.RevertToSnapshot(snap)
		utils.Logger().Error().
			Err(err).Interface("txn", tx).
			Msg("Transaction failed commitment")
		return errNilReceipt
	}
	if receipt == nil {
		utils.Logger().Warn().Interface("tx", tx).Interface("cx", cx).Msg("Receipt is Nil!")
		return errNilReceipt
	}

	w.current.txs = append(w.current.txs, tx)
	w.current.receipts = append(w.current.receipts, receipt)
	w.current.logs = append(w.current.logs, receipt.Logs...)
	w.current.stakeMsgs = append(w.current.stakeMsgs, stakeMsgs...)

	if cx != nil {
		w.current.outcxs = append(w.current.outcxs, cx)
	}
	return nil
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
		if err := core.ApplyIncomingReceipt(
			w.config, w.current.state, w.current.header, cx,
		); err != nil {
			return errors.Wrapf(err, "Failed applying cross-shard receipts")
		}
	}
	w.current.incxs = append(w.current.incxs, receiptsList...)
	return nil
}

// UpdateCurrent updates the current environment with the current state and header.
func (w *Worker) UpdateCurrent() (Environment, error) {
	parent := w.chain.CurrentHeader()
	num := parent.Number()
	timestamp := time.Now().Unix()

	epoch := GetNewEpoch(w.chain)
	header := w.factory.NewHeader(epoch).With().
		ParentHash(parent.Hash()).
		Number(num.Add(num, common.Big1)).
		GasLimit(core.CalcGasLimit(parent, w.GasFloor(epoch), w.GasCeil())).
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
func (w *Worker) makeCurrent(parent *block.Header, header *block.Header) (*environment, error) {
	env, err := makeEnvironment(w.chain, parent, header)
	if err != nil {
		return nil, err
	}

	w.current = env
	return w.current, nil
}

func makeEnvironment(chain core.BlockChain, parent *block.Header, header *block.Header) (*environment, error) {
	state, err := chain.StateAt(parent.Root())
	if err != nil {
		return nil, err
	}
	env := &environment{
		signer:    types.NewEIP155Signer(chain.Config().ChainID),
		ethSigner: types.NewEIP155Signer(chain.Config().EthCompatibleChainID),
		state:     state,
		header:    header,
	}
	return env, nil
}

// GetCurrentResult gets the current block processing result.
func (w *Worker) GetCurrentResult() *core.ProcessorResult {
	return &core.ProcessorResult{
		Receipts:   w.current.receipts,
		CxReceipts: w.current.outcxs,
		Logs:       w.current.logs,
		UsedGas:    w.current.header.GasUsed(),
		Reward:     w.current.reward,
		State:      w.current.state,
		StakeMsgs:  w.current.stakeMsgs,
	}
}

// GetCurrentState gets the current state.
func (w *Worker) GetCurrentState() *state.DB {
	return w.current.state
}

// GetNewEpoch gets the current epoch.
func GetNewEpoch(chain core.BlockChain) *big.Int {
	parent := chain.CurrentBlock()
	epoch := new(big.Int).Set(parent.Header().Epoch())

	shardState, err := parent.Header().GetShardState()
	if err == nil &&
		shardState.Epoch != nil &&
		chain.Config().IsStaking(shardState.Epoch) {
		// For shard state of staking epochs, the shard state will
		// have an epoch and it will decide the next epoch for following blocks
		epoch = new(big.Int).Set(shardState.Epoch)
	} else {
		if parent.IsLastBlockInEpoch() && parent.NumberU64() != 0 {
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

// CollectVerifiedSlashes sets w.current.slashes only to those that
// past verification
func (w *Worker) CollectVerifiedSlashes() error {
	pending, failures :=
		w.chain.ReadPendingSlashingCandidates(), slash.Records{}
	if d := pending; len(d) > 0 {
		pending, failures = w.verifySlashes(d)
	}

	if f := failures; len(f) > 0 {
		if err := w.chain.DeleteFromPendingSlashingCandidates(f); err != nil {
			return err
		}
	}
	w.current.slashes = pending
	return nil
}

// returns (successes, failures, error)
func (w *Worker) verifySlashes(
	d slash.Records,
) (slash.Records, slash.Records) {
	successes, failures := slash.Records{}, slash.Records{}
	// Enforce order, reproducibility
	sort.SliceStable(d,
		func(i, j int) bool {
			return bytes.Compare(
				d[i].Reporter.Bytes(), d[j].Reporter.Bytes(),
			) == -1
		},
	)

	// Always base the state on current tip of the chain
	workingState, err := w.chain.State()
	if err != nil {
		return successes, failures
	}

	seenEvidences := map[common.Hash]struct{}{}

	for i := range d {
		evidenceHash := hash.FromRLPNew256(d[i].Evidence)
		if existing, ok := seenEvidences[evidenceHash]; ok {
			utils.Logger().Warn().
				Interface("slashRecord1", existing).
				Interface("slashRecord2", d[i]).
				Msg("Duplicate slash records with different reporters")
			failures = append(failures, d[i])
		} else {
			seenEvidences[evidenceHash] = struct{}{}

			// In addition, need to count the same evidence with first and second vote swapped as seen
			swapVote := d[i].Evidence
			tmp := swapVote.ConflictingVotes.FirstVote
			swapVote.ConflictingVotes.FirstVote = swapVote.ConflictingVotes.SecondVote
			swapVote.ConflictingVotes.SecondVote = tmp
			swapHash := hash.FromRLPNew256(swapVote)
			seenEvidences[swapHash] = struct{}{}
		}

		if err := slash.Verify(
			w.chain, workingState, &d[i],
		); err != nil {
			utils.Logger().Warn().Err(err).
				Interface("slashRecord", d[i]).
				Msg("Slash failed verification")
			failures = append(failures, d[i])
			continue
		}
		successes = append(successes, d[i])
	}

	if f := len(failures); f > 0 {
		utils.Logger().Debug().
			Int("count", f).
			Msg("invalid slash records passed over in block proposal")
	}

	return successes, failures
}

// FinalizeNewBlock generate a new block for the next consensus round.
func (w *Worker) FinalizeNewBlock(
	commitSigs chan []byte, viewID func() uint64, coinbase common.Address,
	crossLinks types.CrossLinks, shardState *shard.State,
) (*types.Block, error) {
	w.current.header.SetCoinbase(coinbase)

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
	state := w.current.state
	copyHeader := types.CopyHeader(w.current.header)

	sigsReady := make(chan bool)
	go func() {
		select {
		case sigs := <-commitSigs:
			sig, signers, err := bls.SeparateSigAndMask(sigs)
			if err != nil {
				utils.Logger().Error().Err(err).Msg("Failed to parse commit sigs")
				sigsReady <- false
			}
			// Put sig, signers, viewID, coinbase into header
			if len(sig) > 0 && len(signers) > 0 {
				sig2 := copyHeader.LastCommitSignature()
				copy(sig2[:], sig[:])
				utils.Logger().Info().Hex("sigs", sig).Hex("bitmap", signers).Msg("Setting commit sigs")
				copyHeader.SetLastCommitSignature(sig2)
				copyHeader.SetLastCommitBitmap(signers)
			}
			sigsReady <- true
		case <-time.After(consensus.CommitSigReceiverTimeout):
			// Exit goroutine
			utils.Logger().Warn().Msg("Timeout waiting for commit sigs")
		}
	}()

	block, payout, err := w.chain.Engine().Finalize(
		w.chain,
		w.beacon,
		copyHeader, state, w.current.txs, w.current.receipts,
		w.current.outcxs, w.current.incxs, w.current.stakingTxs,
		w.current.slashes, sigsReady, viewID,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot finalize block")
	}
	w.current.reward = payout
	return block, nil
}

func (w *Worker) GasFloor(epoch *big.Int) uint64 {
	if w.config.IsBlockGas30M(epoch) {
		return 30_000_000
	}

	return w.gasFloor
}

func (w *Worker) GasCeil() uint64 {
	return w.gasCeil
}
