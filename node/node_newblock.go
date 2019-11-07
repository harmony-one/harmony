package node

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
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
				for {
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
						utils.Logger().Err(err).
							Msg("!!!!!!!!!cannot commit new block!!!!!!!!!")
					}
				}
			}
		}
	}()
}

func (node *Node) proposeNewBlock() (*types.Block, error) {
	// Update worker's current header and state data in preparation to propose/process new transactions
	coinbase := node.Consensus.SelfAddress

	// Prepare transactions including staking transactions
	selectedTxs, selectedStakingTxs := node.getTransactionsForNewBlock(coinbase)

	if err := node.Worker.CommitTransactions(selectedTxs, selectedStakingTxs, coinbase); err != nil {
		ctxerror.Log15(utils.GetLogger().Error,
			ctxerror.New("cannot commit transactions").
				WithCause(err))
		return nil, err
	}

	// Prepare cross shard transaction receipts
	receiptsList := node.proposeReceiptsProof()
	if len(receiptsList) != 0 {
		if err := node.Worker.CommitReceipts(receiptsList); err != nil {
			ctxerror.Log15(utils.GetLogger().Error,
				ctxerror.New("cannot commit receipts").
					WithCause(err))
		}
	}

	// Prepare cross links
	var crossLinks types.CrossLinks
	if node.NodeConfig.ShardID == shard.BeaconChainShardID {
		crossLinksToPropose, localErr := node.ProposeCrossLinkDataForBeaconchain()
		if localErr == nil {
			crossLinks = crossLinksToPropose
		}
	}

	// Prepare shard state
	shardState := node.Worker.ProposeShardStateWithoutBeaconSync()

	// Prepare last commit signatures
	sig, mask, err := node.Consensus.LastCommitSig()
	if err != nil {
		ctxerror.Log15(utils.GetLogger().Error,
			ctxerror.New("Cannot get commit signatures from last block").
				WithCause(err))
		return nil, err
	}
	return node.Worker.FinalizeNewBlock(sig, mask, node.Consensus.GetViewID(), coinbase, crossLinks, shardState)
}

func (node *Node) proposeShardState(block *types.Block) error {
	switch node.Consensus.ShardID {
	case shard.BeaconChainShardID:
		return node.proposeBeaconShardState(block)
	default:
		fmt.Println("This should not be happening")
		// should not be possible - only beaconchain can create new supercommittee
		return nil
	}
}

func (node *Node) proposeBeaconShardState(block *types.Block) error {
	// TODO ek - replace this with variable epoch logic.
	if !core.IsEpochLastBlock(block) {
		// We haven't reached the end of this epoch; don't propose yet.
		return nil
	}
	// TODO need to pass it the ChainReader?
	shardState := committee.IncorporatingStaking.Read(
		new(big.Int).Add(block.Header().Epoch(), common.Big1),
	)
	// shardState, err := core.CalculateNewShardState(node.Blockchain(), nextEpoch, committee.MemberAssigner)
	// if err != nil {
	// 	return err
	// }
	return block.AddShardState(shardState)
}

func (node *Node) proposeReceiptsProof() []*types.CXReceiptsProof {
	if !node.Blockchain().Config().IsCrossTx(node.Worker.GetNewEpoch()) {
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

	sort.Slice(pendingCXReceipts, func(i, j int) bool {
		return pendingCXReceipts[i].MerkleProof.ShardID < pendingCXReceipts[j].MerkleProof.ShardID || (pendingCXReceipts[i].MerkleProof.ShardID == pendingCXReceipts[j].MerkleProof.ShardID && pendingCXReceipts[i].MerkleProof.BlockNum.Cmp(pendingCXReceipts[j].MerkleProof.BlockNum) < 0)
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
			utils.Logger().Error().Err(err).Msg("[proposeReceiptsProof] Invalid CXReceiptsProof")
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
