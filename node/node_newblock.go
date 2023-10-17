package node

import (
	"sort"
	"strings"
	"time"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/pkg/errors"

	staking "github.com/harmony-one/harmony/staking/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

// Constants of proposing a new block
const (
	SleepPeriod           = 20 * time.Millisecond
	IncomingReceiptsLimit = 6000 // 2000 * (numShards - 1)
)

// WaitForConsensusReadyV2 listen for the readiness signal from consensus and generate new block for consensus.
// only leader will receive the ready signal
func (node *Node) WaitForConsensusReadyV2(cs *consensus.Consensus, stopChan chan struct{}, stoppedChan chan struct{}) {
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
			case proposalType := <-cs.GetReadySignal():
				for retryCount := 0; retryCount < 3 && cs.IsLeader(); retryCount++ {
					time.Sleep(SleepPeriod)
					utils.Logger().Info().
						Uint64("blockNum", cs.Blockchain().CurrentBlock().NumberU64()+1).
						Bool("asyncProposal", proposalType == consensus.AsyncProposal).
						Msg("PROPOSING NEW BLOCK ------------------------------------------------")

					// Prepare last commit signatures
					newCommitSigsChan := make(chan []byte)

					go func() {
						waitTime := 0 * time.Second
						if proposalType == consensus.AsyncProposal {
							waitTime = consensus.CommitSigReceiverTimeout
						}
						select {
						case <-time.After(waitTime):
							if waitTime == 0 {
								utils.Logger().Info().Msg("[ProposeNewBlock] Sync block proposal, reading commit sigs directly from DB")
							} else {
								utils.Logger().Info().Msg("[ProposeNewBlock] Timeout waiting for commit sigs, reading directly from DB")
							}
							sigs, err := cs.BlockCommitSigs(cs.Blockchain().CurrentBlock().NumberU64())

							if err != nil {
								utils.Logger().Error().Err(err).Msg("[ProposeNewBlock] Cannot get commit signatures from last block")
							} else {
								newCommitSigsChan <- sigs
							}
						case commitSigs := <-cs.GetCommitSigChannel():
							utils.Logger().Info().Msg("[ProposeNewBlock] received commit sigs asynchronously")
							if len(commitSigs) > bls.BLSSignatureSizeInBytes {
								newCommitSigsChan <- commitSigs
							}
						}
					}()
					newBlock, err := node.ProposeNewBlock(newCommitSigsChan)
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
						cs.BlockChannel(newBlock)
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

// ProposeNewBlock proposes a new block...
func (node *Node) ProposeNewBlock(commitSigs chan []byte) (*types.Block, error) {
	currentHeader := node.Blockchain().CurrentHeader()
	nowEpoch, blockNow := currentHeader.Epoch(), currentHeader.Number()
	utils.AnalysisStart("ProposeNewBlock", nowEpoch, blockNow)
	defer utils.AnalysisEnd("ProposeNewBlock", nowEpoch, blockNow)

	// Update worker's current header and
	// state data in preparation to propose/process new transactions
	env, err := node.Worker.UpdateCurrent()
	if err != nil {
		return nil, errors.Wrap(err, "failed to update worker")
	}

	var (
		header      = env.CurrentHeader()
		leaderKey   = node.Consensus.GetLeaderPubKey()
		coinbase    = node.GetAddressForBLSKey(leaderKey.Object, header.Epoch())
		beneficiary = coinbase
	)

	// After staking, all coinbase will be the address of bls pub key
	if node.Blockchain().Config().IsStaking(header.Epoch()) {
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
	beneficiary, err = node.Blockchain().GetECDSAFromCoinbase(header)
	if err != nil {
		return nil, err
	}

	// Add VRF
	if node.Blockchain().Config().IsVRF(header.Epoch()) {
		//generate a new VRF for the current block
		if err := node.Consensus.GenerateVrfAndProof(header); err != nil {
			return nil, err
		}
	}

	// Execute all the time except for last block of epoch for shard 0
	if !shard.Schedule.IsLastBlock(header.Number().Uint64()) || node.Consensus.ShardID != 0 {
		// Prepare normal and staking transactions retrieved from transaction pool
		utils.AnalysisStart("proposeNewBlockChooseFromTxnPool")

		pendingPoolTxs, err := node.TxPool.Pending()
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
					if node.Blockchain().Config().IsPreStaking(node.Worker.GetCurrentHeader().Epoch()) {
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
		if err := node.Worker.CommitTransactions(
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
	receiptsList := node.proposeReceiptsProof()
	if len(receiptsList) != 0 {
		if err := node.Worker.CommitReceipts(receiptsList); err != nil {
			return nil, err
		}
	}

	isBeaconchainInCrossLinkEra := node.NodeConfig.ShardID == shard.BeaconChainShardID &&
		node.Blockchain().Config().IsCrossLink(node.Worker.GetCurrentHeader().Epoch())

	isBeaconchainInStakingEra := node.NodeConfig.ShardID == shard.BeaconChainShardID &&
		node.Blockchain().Config().IsStaking(node.Worker.GetCurrentHeader().Epoch())

	utils.AnalysisStart("proposeNewBlockVerifyCrossLinks")
	// Prepare cross links and slashing messages
	var crossLinksToPropose types.CrossLinks
	if isBeaconchainInCrossLinkEra {
		allPending, err := node.Blockchain().ReadPendingCrossLinks()
		invalidToDelete := []types.CrossLink{}
		if err == nil {
			for _, pending := range allPending {
				// ReadCrossLink beacon chain usage.
				exist, err := node.Blockchain().ReadCrossLink(pending.ShardID(), pending.BlockNum())
				if err == nil || exist != nil {
					invalidToDelete = append(invalidToDelete, pending)
					utils.Logger().Debug().
						AnErr("[ProposeNewBlock] pending crosslink is already committed onchain", err)
					continue
				}

				// Crosslink is already verified before it's accepted to pending,
				// no need to verify again in proposal.
				if !node.Blockchain().Config().IsCrossLink(pending.Epoch()) {
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
		node.Blockchain().DeleteFromPendingCrossLinks(invalidToDelete)
	}
	utils.AnalysisEnd("proposeNewBlockVerifyCrossLinks")

	if isBeaconchainInStakingEra {
		// this will set a meaningful w.current.slashes
		if err := node.Worker.CollectVerifiedSlashes(); err != nil {
			return nil, err
		}
	}

	node.Worker.ApplyShardReduction()
	// Prepare shard state
	var shardState *shard.State
	if shardState, err = node.Blockchain().SuperCommitteeForNextEpoch(
		node.Beaconchain(), node.Worker.GetCurrentHeader(), false,
	); err != nil {
		return nil, err
	}

	viewIDFunc := func() uint64 {
		return node.Consensus.GetCurBlockViewID()
	}
	finalizedBlock, err := node.Worker.FinalizeNewBlock(
		commitSigs, viewIDFunc,
		coinbase, crossLinksToPropose, shardState,
	)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[ProposeNewBlock] Failed finalizing the new block")
		return nil, err
	}

	utils.Logger().Info().Msg("[ProposeNewBlock] verifying the new block header")
	// err = node.Blockchain().Validator().ValidateHeader(finalizedBlock, true)
	err = core.NewBlockValidator(node.Blockchain()).ValidateHeader(finalizedBlock, true)

	if err != nil {
		utils.Logger().Error().Err(err).Msg("[ProposeNewBlock] Failed verifying the new block header")
		return nil, err
	}

	// Save process result in the cache for later use for faster block commitment to db.
	result := node.Worker.GetCurrentResult()
	node.Blockchain().Processor().CacheProcessorResult(finalizedBlock.Hash(), result)
	return finalizedBlock, nil
}

func (node *Node) proposeReceiptsProof() []*types.CXReceiptsProof {
	if !node.Blockchain().Config().HasCrossTxFields(node.Worker.GetCurrentHeader().Epoch()) {
		return []*types.CXReceiptsProof{}
	}

	numProposed := 0
	validReceiptsList := []*types.CXReceiptsProof{}
	pendingReceiptsList := []*types.CXReceiptsProof{}

	node.pendingCXMutex.Lock()
	defer node.pendingCXMutex.Unlock()

	// not necessary to sort the list, but we just prefer to process the list ordered by shard and blocknum
	pendingCXReceipts := []*types.CXReceiptsProof{}
	for _, v := range node.pendingCXReceipts {
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
	for _, cxp := range node.pendingCXReceipts {
		if numProposed > IncomingReceiptsLimit {
			pendingReceiptsList = append(pendingReceiptsList, cxp)
			continue
		}
		// check double spent
		if node.Blockchain().IsSpent(cxp) {
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
			if item.ToShardID != node.Blockchain().ShardID() {
				continue Loop
			}
		}

		if err := core.NewBlockValidator(node.Blockchain()).ValidateCXReceiptsProof(cxp); err != nil {
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

	node.pendingCXReceipts = make(map[string]*types.CXReceiptsProof)
	for _, v := range pendingReceiptsList {
		blockNum := v.Header.Number().Uint64()
		shardID := v.Header.ShardID()
		key := utils.GetPendingCXKey(shardID, blockNum)
		node.pendingCXReceipts[key] = v
	}

	utils.Logger().Debug().Msgf("[proposeReceiptsProof] number of validReceipts %d", len(validReceiptsList))
	return validReceiptsList
}
