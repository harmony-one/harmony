package consensus

import (
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/harmony/worker"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// Constants of proposing a new block
const (
	IncomingReceiptsLimit = 6000 // 2000 * (numShards - 1)
	SleepPeriod           = 20 * time.Millisecond
)

// ProposeNewBlock proposes a new block...
func (consensus *Consensus) ProposeNewBlock(commitSigs chan []byte) (*types.Block, error) {
	var (
		currentHeader = consensus.Blockchain().CurrentHeader()
		nowEpoch      = currentHeader.Epoch()
		blockNow      = currentHeader.Number()
		worker        = consensus.registry.GetWorker()
	)
	utils.AnalysisStart("ProposeNewBlock", nowEpoch, blockNow)
	defer utils.AnalysisEnd("ProposeNewBlock", nowEpoch, blockNow)

	// Update worker's current header and
	// state data in preparation to propose/process new transactions
	env, err := worker.UpdateCurrent()
	if err != nil {
		return nil, errors.Wrap(err, "failed to update worker")
	}
	header := env.CurrentHeader()
	shardState, err := consensus.Blockchain().ReadShardState(header.Epoch())
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read shard")
	}

	var (
		leaderKey   = consensus.GetLeaderPubKey()
		coinbase    = consensus.registry.GetAddressToBLSKey().GetAddressForBLSKey(consensus.GetPublicKeys(), shardState, leaderKey.Object, header.Epoch())
		beneficiary = coinbase
	)

	// After staking, all coinbase will be the address of bls pub key
	if consensus.Blockchain().Config().IsStaking(header.Epoch()) {
		blsPubKeyBytes := leaderKey.Object.GetAddress()
		coinbase.SetBytes(blsPubKeyBytes[:])
	}

	if coinbase == (common.Address{}) {
		return nil, errors.New("[ProposeNewBlock] Failed setting coinbase")
	}

	// Must set coinbase here because the operations below depend on it
	header.SetCoinbase(coinbase)

	// Get beneficiary based on coinbase
	// Before staking, coinbase itself is the beneficial
	// After staking, beneficial is the corresponding ECDSA address of the bls key
	beneficiary, err = consensus.Blockchain().GetECDSAFromCoinbase(header)
	if err != nil {
		return nil, err
	}

	// Add VRF
	if consensus.Blockchain().Config().IsVRF(header.Epoch()) {
		//generate a new VRF for the current block
		if err := consensus.GenerateVrfAndProof(header); err != nil {
			return nil, err
		}
	}

	// Execute all the time except for last block of epoch for shard 0
	if !shard.Schedule.IsLastBlock(header.Number().Uint64()) || consensus.ShardID != 0 {
		// Prepare normal and staking transactions retrieved from transaction pool
		utils.AnalysisStart("proposeNewBlockChooseFromTxnPool")

		pendingPoolTxs, err := consensus.registry.GetTxPool().Pending()
		if err != nil {
			utils.Logger().Err(err).Msg("Failed to fetch pending transactions")
			return nil, err
		}
		pendingPlainTxs := map[common.Address]types.Transactions{}
		pendingStakingTxs := staking.StakingTransactions{}
		for addr, poolTxs := range pendingPoolTxs {
			plainTxsPerAcc := types.Transactions{}
			for _, tx := range poolTxs {
				if plainTx, ok := tx.(*types.Transaction); ok {
					plainTxsPerAcc = append(plainTxsPerAcc, plainTx)
				} else if stakingTx, ok := tx.(*staking.StakingTransaction); ok {
					// Only process staking transactions after pre-staking epoch happened.
					if consensus.Blockchain().Config().IsPreStaking(worker.GetCurrentHeader().Epoch()) {
						pendingStakingTxs = append(pendingStakingTxs, stakingTx)
					}
				} else {
					utils.Logger().Err(types.ErrUnknownPoolTxType).
						Msg("Failed to parse pending transactions")
					return nil, types.ErrUnknownPoolTxType
				}
			}
			if plainTxsPerAcc.Len() > 0 {
				pendingPlainTxs[addr] = plainTxsPerAcc
			}
		}

		// Try commit normal and staking transactions based on the current state
		// The successfully committed transactions will be put in the proposed block
		if err := worker.CommitTransactions(
			pendingPlainTxs, pendingStakingTxs, beneficiary,
		); err != nil {
			utils.Logger().Error().Err(err).Msg("cannot commit transactions")
			return nil, err
		}
		utils.AnalysisEnd("proposeNewBlockChooseFromTxnPool")
	}

	// Prepare incoming cross shard transaction receipts
	// These are accepted even during the epoch before hip-30
	// because the destination shard only receives them after
	// balance is deducted on source shard. to prevent this from
	// being a significant problem, the source shards will stop
	// accepting txs destined to the shards which are shutting down
	// one epoch prior the shut down
	receiptsList := consensus.proposeReceiptsProof()

	if len(receiptsList) != 0 {
		if err := worker.CommitReceipts(receiptsList); err != nil {
			return nil, err
		}
	}

	isBeaconchainInCrossLinkEra := consensus.ShardID == shard.BeaconChainShardID &&
		consensus.Blockchain().Config().IsCrossLink(worker.GetCurrentHeader().Epoch())

	isBeaconchainInStakingEra := consensus.ShardID == shard.BeaconChainShardID &&
		consensus.Blockchain().Config().IsStaking(worker.GetCurrentHeader().Epoch())

	utils.AnalysisStart("proposeNewBlockVerifyCrossLinks")
	// Prepare cross links and slashing messages
	var crossLinksToPropose types.CrossLinks
	if isBeaconchainInCrossLinkEra {
		allPending, err := consensus.Blockchain().ReadPendingCrossLinks()
		invalidToDelete := []types.CrossLink{}
		if err == nil {
			for _, pending := range allPending {
				// ReadCrossLink beacon chain usage.
				exist, err := consensus.Blockchain().ReadCrossLink(pending.ShardID(), pending.BlockNum())
				if err == nil || exist != nil {
					invalidToDelete = append(invalidToDelete, pending)
					utils.Logger().Debug().
						AnErr("[ProposeNewBlock] pending crosslink is already committed onchain", err)
					continue
				}
				last, err := consensus.Blockchain().ReadShardLastCrossLink(pending.ShardID())
				if err != nil {
					utils.Logger().Debug().
						AnErr("[ProposeNewBlock] failed to read last crosslink", err)
					// no return
				}
				// if pending crosslink is older than the last crosslink, delete it and continue
				if err == nil && exist == nil && last != nil && last.BlockNum() >= pending.BlockNum() {
					invalidToDelete = append(invalidToDelete, pending)
				}

				// Crosslink is already verified before it's accepted to pending,
				// no need to verify again in proposal.
				if !consensus.Blockchain().Config().IsCrossLink(pending.Epoch()) {
					utils.Logger().Debug().
						AnErr("[ProposeNewBlock] pending crosslink that's before crosslink epoch", err)
					continue
				}

				crossLinksToPropose = append(crossLinksToPropose, pending)
				if len(crossLinksToPropose) > 15 {
					break
				}
			}
			utils.Logger().Info().
				Msgf("[ProposeNewBlock] Proposed %d crosslinks from %d pending crosslinks",
					len(crossLinksToPropose), len(allPending),
				)
		} else {
			utils.Logger().Warn().Err(err).Msgf(
				"[ProposeNewBlock] Unable to Read PendingCrossLinks, number of crosslinks: %d",
				len(allPending),
			)
		}
		if n, err := consensus.Blockchain().DeleteFromPendingCrossLinks(invalidToDelete); err != nil {
			utils.Logger().Error().
				Err(err).
				Msg("[ProposeNewBlock] invalid pending cross links failed")
		} else if len(invalidToDelete) > 0 {
			utils.Logger().Info().
				Int("not-deleted", n).
				Int("deleted", len(invalidToDelete)).
				Msg("[ProposeNewBlock] deleted invalid pending cross links")
		}
	}
	utils.AnalysisEnd("proposeNewBlockVerifyCrossLinks")

	if isBeaconchainInStakingEra {
		// this will set a meaningful w.current.slashes
		if err := worker.CollectVerifiedSlashes(); err != nil {
			return nil, err
		}
	}

	worker.ApplyShardReduction()

	// Prepare shard state
	if shardState, err = consensus.Blockchain().SuperCommitteeForNextEpoch(
		consensus.Beaconchain(), worker.GetCurrentHeader(), false,
	); err != nil {
		return nil, err
	}

	viewIDFunc := func() uint64 {
		return consensus.GetCurBlockViewID()
	}
	finalizedBlock, err := worker.FinalizeNewBlock(
		commitSigs, viewIDFunc,
		coinbase, crossLinksToPropose, shardState,
	)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[ProposeNewBlock] Failed finalizing the new block")
		return nil, err
	}

	utils.Logger().Info().Msg("[ProposeNewBlock] verifying the new block header")
	err = core.NewBlockValidator(consensus.Blockchain()).ValidateHeader(finalizedBlock, true)

	if err != nil {
		utils.Logger().Error().Err(err).Msg("[ProposeNewBlock] Failed verifying the new block header")
		return nil, err
	}

	// Save process result in the cache for later use for faster block commitment to db.
	result := worker.GetCurrentResult()
	consensus.Blockchain().Processor().CacheProcessorResult(finalizedBlock.Hash(), result)
	return finalizedBlock, nil
}

func (consensus *Consensus) proposeReceiptsProof() []*types.CXReceiptsProof {
	if !consensus.Blockchain().Config().HasCrossTxFields(consensus.registry.GetWorker().GetCurrentHeader().Epoch()) {
		return []*types.CXReceiptsProof{}
	}

	numProposed := 0
	validReceiptsList := []*types.CXReceiptsProof{}
	pendingReceiptsList := []*types.CXReceiptsProof{}

	// not necessary to sort the list, but we just prefer to process the list ordered by shard and blocknum
	pendingCXReceipts := []*types.CXReceiptsProof{}
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	for _, v := range consensus.pendingCXReceipts {
		pendingCXReceipts = append(pendingCXReceipts, v)
	}

	sort.SliceStable(pendingCXReceipts, func(i, j int) bool {
		shardCMP := pendingCXReceipts[i].MerkleProof.ShardID < pendingCXReceipts[j].MerkleProof.ShardID
		shardEQ := pendingCXReceipts[i].MerkleProof.ShardID == pendingCXReceipts[j].MerkleProof.ShardID
		blockCMP := pendingCXReceipts[i].MerkleProof.BlockNum.Cmp(
			pendingCXReceipts[j].MerkleProof.BlockNum,
		) == -1
		return shardCMP || (shardEQ && blockCMP)
	})

	m := map[common.Hash]struct{}{}

Loop:
	for _, cxp := range consensus.pendingCXReceipts {
		if numProposed > IncomingReceiptsLimit {
			pendingReceiptsList = append(pendingReceiptsList, cxp)
			continue
		}
		// check double spent
		if consensus.Blockchain().IsSpent(cxp) {
			utils.Logger().Debug().Interface("cxp", cxp).Msg("[proposeReceiptsProof] CXReceipt is spent")
			continue
		}
		hash := cxp.MerkleProof.BlockHash
		// ignore duplicated receipts
		if _, ok := m[hash]; ok {
			continue
		} else {
			m[hash] = struct{}{}
		}

		for _, item := range cxp.Receipts {
			if item.ToShardID != consensus.Blockchain().ShardID() {
				continue Loop
			}
		}

		if err := core.NewBlockValidator(consensus.Blockchain()).ValidateCXReceiptsProof(cxp); err != nil {
			if strings.Contains(err.Error(), rawdb.MsgNoShardStateFromDB) {
				pendingReceiptsList = append(pendingReceiptsList, cxp)
			} else {
				utils.Logger().Error().Err(err).Msg("[proposeReceiptsProof] Invalid CXReceiptsProof")
			}
			continue
		}

		utils.Logger().Debug().Interface("cxp", cxp).Msg("[proposeReceiptsProof] CXReceipts Added")
		validReceiptsList = append(validReceiptsList, cxp)
		numProposed = numProposed + len(cxp.Receipts)
	}

	consensus.pendingCXReceipts = make(map[utils.CXKey]*types.CXReceiptsProof)
	for _, v := range pendingReceiptsList {
		blockNum := v.Header.Number().Uint64()
		shardID := v.Header.ShardID()
		key := utils.GetPendingCXKey(shardID, blockNum)
		consensus.pendingCXReceipts[key] = v
	}

	utils.Logger().Debug().Msgf("[proposeReceiptsProof] number of validReceipts %d", len(validReceiptsList))
	return validReceiptsList
}

func (consensus *Consensus) AddPendingReceipts(receipts *types.CXReceiptsProof) {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	if receipts.ContainsEmptyField() {
		utils.Logger().Info().
			Int("totalPendingReceipts", len(consensus.pendingCXReceipts)).
			Msg("CXReceiptsProof contains empty field")
		return
	}

	blockNum := receipts.Header.Number().Uint64()
	shardID := receipts.Header.ShardID()

	// Sanity checks

	if err := core.NewBlockValidator(consensus.Blockchain()).ValidateCXReceiptsProof(receipts); err != nil {
		if !strings.Contains(err.Error(), rawdb.MsgNoShardStateFromDB) {
			utils.Logger().Error().Err(err).Msg("[AddPendingReceipts] Invalid CXReceiptsProof")
			return
		}
	}

	// cross-shard receipt should not be coming from our shard
	if s := consensus.ShardID; s == shardID {
		utils.Logger().Info().
			Uint32("my-shard", s).
			Uint32("receipt-shard", shardID).
			Msg("ShardID of incoming receipt was same as mine")
		return
	}

	if e := receipts.Header.Epoch(); blockNum == 0 ||
		!consensus.Blockchain().Config().AcceptsCrossTx(e) {
		utils.Logger().Info().
			Uint64("incoming-epoch", e.Uint64()).
			Msg("Incoming receipt had meaningless epoch")
		return
	}

	key := utils.GetPendingCXKey(shardID, blockNum)

	// DDoS protection
	const maxCrossTxnSize = 4096
	if s := len(consensus.pendingCXReceipts); s >= maxCrossTxnSize {
		utils.Logger().Info().
			Int("pending-cx-receipts-size", s).
			Int("pending-cx-receipts-limit", maxCrossTxnSize).
			Msg("Current pending cx-receipts reached size limit")
		return
	}

	if _, ok := consensus.pendingCXReceipts[key]; ok {
		utils.Logger().Info().
			Int("totalPendingReceipts", len(consensus.pendingCXReceipts)).
			Msg("Already Got Same Receipt message")
		return
	}
	consensus.pendingCXReceipts[key] = receipts
	utils.Logger().Info().
		Int("totalPendingReceipts", len(consensus.pendingCXReceipts)).
		Msg("Got ONE more receipt message")
}

// PendingCXReceipts returns node.pendingCXReceiptsProof
func (consensus *Consensus) PendingCXReceipts() []*types.CXReceiptsProof {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	cxReceipts := make([]*types.CXReceiptsProof, len(consensus.pendingCXReceipts))
	i := 0
	for _, cxReceipt := range consensus.pendingCXReceipts {
		cxReceipts[i] = cxReceipt
		i++
	}
	return cxReceipts
}

// WaitForConsensusReadyV2 listen for the readiness signal from consensus and generate new block for consensus.
// only leader will receive the ready signal
func (consensus *Consensus) WaitForConsensusReadyV2(stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		// Setup stoppedChan
		defer close(stoppedChan)

		utils.Logger().Debug().
			Msg("Waiting for Consensus ready")
		select {
		case <-time.After(30 * time.Second):
		case <-stopChan:
			return
		}

		for {
			// keep waiting for Consensus ready
			select {
			case <-stopChan:
				utils.Logger().Warn().
					Msg("Consensus new block proposal: STOPPED!")
				return
			case proposal := <-consensus.GetReadySignal():
				for retryCount := 0; retryCount < 3 && consensus.IsLeader(); retryCount++ {
					time.Sleep(SleepPeriod)
					utils.Logger().Info().
						Uint64("blockNum", consensus.Blockchain().CurrentBlock().NumberU64()+1).
						Bool("asyncProposal", proposal.Type == AsyncProposal).
						Str("called", proposal.Caller).
						Msg("PROPOSING NEW BLOCK ------------------------------------------------")

					// Prepare last commit signatures
					newCommitSigsChan := make(chan []byte)

					go func() {
						waitTime := 0 * time.Second
						if proposal.Type == AsyncProposal {
							waitTime = worker.CommitSigReceiverTimeout
						}
						select {
						case <-time.After(waitTime):
							if waitTime == 0 {
								utils.Logger().Info().Msg("[ProposeNewBlock] Sync block proposal, reading commit sigs directly from DB")
							} else {
								utils.Logger().Info().Msg("[ProposeNewBlock] Timeout waiting for commit sigs, reading directly from DB")
							}
							sigs, err := consensus.BlockCommitSigs(consensus.Blockchain().CurrentBlock().NumberU64())

							if err != nil {
								utils.Logger().Error().Err(err).Msg("[ProposeNewBlock] Cannot get commit signatures from last block")
							} else {
								newCommitSigsChan <- sigs
							}
						case commitSigs := <-consensus.GetCommitSigChannel():
							utils.Logger().Info().Msg("[ProposeNewBlock] received commit sigs asynchronously")
							if len(commitSigs) > bls.BLSSignatureSizeInBytes {
								newCommitSigsChan <- commitSigs
							}
						}
					}()
					newBlock, err := consensus.ProposeNewBlock(newCommitSigsChan)
					if err == nil {
						utils.Logger().Info().
							Uint64("blockNum", newBlock.NumberU64()).
							Uint64("epoch", newBlock.Epoch().Uint64()).
							Uint64("viewID", newBlock.Header().ViewID().Uint64()).
							Int("numTxs", newBlock.Transactions().Len()).
							Int("numStakingTxs", newBlock.StakingTransactions().Len()).
							Int("crossShardReceipts", newBlock.IncomingReceipts().Len()).
							Msgf("=========Successfully Proposed New Block, shard: %d epoch: %d number: %d ==========", newBlock.ShardID(), newBlock.Epoch().Uint64(), newBlock.NumberU64())

						// Send the new block to Consensus so it can be confirmed.
						consensus.BlockChannel(newBlock)
						break
					} else {
						utils.Logger().Err(err).Int("retryCount", retryCount).
							Msg("!!!!!!!!!Failed Proposing New Block!!!!!!!!!")
						continue
					}
				}
			}
		}
	}()
}
