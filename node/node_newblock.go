package node

import (
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
)

// Constants of proposing a new block
const (
	PeriodicBlock         = 200 * time.Millisecond
	IncomingReceiptsLimit = 6000 // 2000 * (numShards - 1)
)

// WaitForConsensusReadyV2 listen for the readiness signal from consensus and generate new block for consensus.
// only leader will receive the ready signal
// TODO: clean pending transactions for validators; or validators not prepare pending transactions
func (node *Node) WaitForConsensusReadyV2(readySignal chan struct{}, stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		// Setup stoppedChan
		defer close(stoppedChan)

		utils.Logger().Debug().
			Msg("Waiting for Consensus ready")
		// TODO: make local net start faster
		time.Sleep(30 * time.Second) // Wait for other nodes to be ready (test-only)

		// Set up the very first deadline.
		deadline := time.Now().Add(node.BlockPeriod)
		for {
			// keep waiting for Consensus ready
			select {
			case <-stopChan:
				utils.Logger().Debug().
					Msg("Consensus new block proposal: STOPPED!")
				return
			case <-readySignal:
				for node.Consensus != nil && node.Consensus.IsLeader() {
					time.Sleep(PeriodicBlock)
					if time.Now().Before(deadline) {
						continue
					}

					utils.Logger().Debug().
						Uint64("blockNum", node.Blockchain().CurrentBlock().NumberU64()+1).
						Msg("PROPOSING NEW BLOCK ------------------------------------------------")

					newBlock, err := node.proposeNewBlock()

					if err == nil {
						utils.Logger().Debug().
							Uint64("blockNum", newBlock.NumberU64()).
							Int("numTxs", newBlock.Transactions().Len()).
							Int("crossShardReceipts", newBlock.IncomingReceipts().Len()).
							Msg("=========Successfully Proposed New Block==========")

						// Set deadline will be BlockPeriod from now at this place. Announce stage happens right after this.
						deadline = time.Now().Add(node.BlockPeriod)
						// Send the new block to Consensus so it can be confirmed.
						node.BlockChannel <- newBlock
						break
					} else {
						utils.Logger().Err(err).Msg("!!!!!!!!!cannot commit new block!!!!!!!!!")
					}
				}
			}
		}
	}()
}

func (node *Node) proposeNewBlock() (*types.Block, error) {
	node.Worker.UpdateCurrent()

	// Update worker's current header and state data in preparation to propose/process new transactions
	var (
		coinbase    = node.Consensus.SelfAddress
		beneficiary = coinbase
		err         error
	)

	node.Worker.GetCurrentHeader().SetCoinbase(coinbase)

	// After staking, all coinbase will be the address of bls pub key
	if header := node.Worker.GetCurrentHeader(); node.Blockchain().Config().IsStaking(header.Epoch()) {
		addr := common.Address{}
		blsPubKeyBytes := node.Consensus.PubKey.GetAddress()
		addr.SetBytes(blsPubKeyBytes[:])
		coinbase = addr // coinbase will be the bls address
		header.SetCoinbase(coinbase)
	}

	beneficiary, err = node.Blockchain().GetECDSAFromCoinbase(node.Worker.GetCurrentHeader())
	if err != nil {
		return nil, err
	}

	// Prepare transactions including staking transactions
	pending, err := node.TxPool.Pending()
	if err != nil {
		utils.Logger().Err(err).Msg("Failed to fetch pending transactions")
		return nil, err
	}

	// TODO: integrate staking transaction into tx pool
	pendingStakingTransactions := staking.StakingTransactions{}
	// Only process staking transactions after pre-staking epoch happened.
	if node.Blockchain().Config().IsPreStaking(node.Worker.GetCurrentHeader().Epoch()) {
		node.pendingStakingTxMutex.Lock()
		for _, tx := range node.pendingStakingTransactions {
			pendingStakingTransactions = append(pendingStakingTransactions, tx)
		}
		node.pendingStakingTransactions = make(map[common.Hash]*staking.StakingTransaction)
		node.pendingStakingTxMutex.Unlock()
	}

	if err := node.Worker.CommitTransactions(
		pending, pendingStakingTransactions, beneficiary,
		func(payload []staking.RPCTransactionError) {
			node.errorSink.Lock()
			for i := range payload {
				node.errorSink.failedStakingTxns.Value = payload[i]
				node.errorSink.failedStakingTxns = node.errorSink.failedStakingTxns.Next()
			}
			node.errorSink.Unlock()
		},
	); err != nil {
		utils.Logger().Error().Err(err).Msg("cannot commit transactions")
		return nil, err
	}

	// Prepare cross shard transaction receipts
	receiptsList := node.proposeReceiptsProof()
	if len(receiptsList) != 0 {
		if err := node.Worker.CommitReceipts(receiptsList); err != nil {
			utils.Logger().Error().Err(err).Msg("[proposeNewBlock] cannot commit receipts")
		}
	}

	// Prepare cross links
	var (
		slashingToPropose   []slash.Record
		crossLinksToPropose types.CrossLinks
	)

	if node.NodeConfig.ShardID == shard.BeaconChainShardID &&
		node.Blockchain().Config().IsCrossLink(node.Worker.GetCurrentHeader().Epoch()) {
		allPending, err := node.Blockchain().ReadPendingCrossLinks()

		if err == nil {
			for _, pending := range allPending {
				if err = node.VerifyCrossLink(pending); err != nil {
					continue
				}
				exist, err := node.Blockchain().ReadCrossLink(pending.ShardID(), pending.BlockNum())
				if err == nil || exist != nil {
					continue
				}
				crossLinksToPropose = append(crossLinksToPropose, pending)
			}
			utils.Logger().Debug().Msgf("[proposeNewBlock] Proposed %d crosslinks from %d pending crosslinks", len(crossLinksToPropose), len(allPending))
		} else {
			utils.Logger().Error().Err(err).Msgf("[proposeNewBlock] Unable to Read PendingCrossLinks, number of crosslinks: %d", len(allPending))
		}
	}

	// Prepare shard state
	shardState := new(shard.State)
	if shardState, err = node.Blockchain().SuperCommitteeForNextEpoch(
		node.Beaconchain(), node.Worker.GetCurrentHeader(), false,
	); err != nil {
		return nil, err
	}

	// Prepare last commit signatures
	sig, mask, err := node.Consensus.LastCommitSig()
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[proposeNewBlock] Cannot get commit signatures from last block")
		return nil, err
	}
	return node.Worker.FinalizeNewBlock(
		sig, mask, node.Consensus.GetViewID(),
		coinbase, crossLinksToPropose, shardState,
		slashingToPropose,
	)
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

	m := make(map[common.Hash]bool)

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
			m[hash] = true
		}

		for _, item := range cxp.Receipts {
			if item.ToShardID != node.Blockchain().ShardID() {
				continue Loop
			}
		}

		if err := node.Blockchain().Validator().ValidateCXReceiptsProof(cxp); err != nil {
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
