package node

import (
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
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

	// Prepare transactions
	selectedTxs := node.getTransactionsForNewBlock(coinbase)

	if err := node.Worker.CommitTransactions(selectedTxs, coinbase); err != nil {
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
	if node.NodeConfig.ShardID == 0 {
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

func (node *Node) proposeShardStateWithoutBeaconSync(block *types.Block) shard.State {
	if block == nil || !core.IsEpochLastBlock(block) {
		return nil
	}

	nextEpoch := new(big.Int).Add(block.Header().Epoch(), common.Big1)
	return core.GetShardState(nextEpoch)
}

func (node *Node) proposeShardState(block *types.Block) error {
	switch node.Consensus.ShardID {
	case 0:
		return node.proposeBeaconShardState(block)
	default:
		node.proposeLocalShardState(block)
		return nil
	}
}

func (node *Node) proposeBeaconShardState(block *types.Block) error {
	// TODO ek - replace this with variable epoch logic.
	if !core.IsEpochLastBlock(block) {
		// We haven't reached the end of this epoch; don't propose yet.
		return nil
	}
	nextEpoch := new(big.Int).Add(block.Header().Epoch(), common.Big1)
	shardState, err := core.CalculateNewShardState(
		node.Blockchain(), nextEpoch, &node.CurrentStakes)
	if err != nil {
		return err
	}
	return block.AddShardState(shardState)
}

func (node *Node) proposeLocalShardState(block *types.Block) {
	logger := block.Logger(utils.Logger())
	// TODO ek – read this from beaconchain once BC sync is fixed
	if node.nextShardState.master == nil {
		logger.Debug().Msg("yet to receive master proposal from beaconchain")
		return
	}

	nlogger := logger.With().
		Uint64("nextEpoch", node.nextShardState.master.Epoch).
		Time("proposeTime", node.nextShardState.proposeTime).
		Logger()
	logger = &nlogger
	if time.Now().Before(node.nextShardState.proposeTime) {
		logger.Debug().Msg("still waiting for shard state to propagate")
		return
	}
	masterShardState := node.nextShardState.master.ShardState
	var localShardState shard.State
	committee := masterShardState.FindCommitteeByID(block.ShardID())
	if committee != nil {
		logger.Info().Msg("found local shard info; proposing it")
		localShardState = append(localShardState, *committee)
	} else {
		logger.Info().Msg("beacon committee disowned us; proposing nothing")
		// Leave local proposal empty to signal the end of shard (disbanding).
	}
	err := block.AddShardState(localShardState)
	if err != nil {
		logger.Error().Err(err).Msg("Failed proposin local shard state")
	}
}

func (node *Node) proposeReceiptsProof() []*types.CXReceiptsProof {
	numProposed := 0
	validReceiptsList := []*types.CXReceiptsProof{}
	pendingReceiptsList := []*types.CXReceiptsProof{}

	node.pendingCXMutex.Lock()

	sort.Slice(node.pendingCXReceipts, func(i, j int) bool {
		return node.pendingCXReceipts[i].MerkleProof.ShardID < node.pendingCXReceipts[j].MerkleProof.ShardID || (node.pendingCXReceipts[i].MerkleProof.ShardID == node.pendingCXReceipts[j].MerkleProof.ShardID && node.pendingCXReceipts[i].MerkleProof.BlockNum.Cmp(node.pendingCXReceipts[j].MerkleProof.BlockNum) < 0)
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

		if err := core.IsValidCXReceiptsProof(cxp); err != nil {
			utils.Logger().Error().Err(err).Msg("[proposeReceiptsProof] Invalid CXReceiptsProof")
			continue
		}

		utils.Logger().Debug().Interface("cxp", cxp).Msg("[proposeReceiptsProof] CXReceipts Added")
		validReceiptsList = append(validReceiptsList, cxp)
		numProposed = numProposed + len(cxp.Receipts)
	}

	node.pendingCXReceipts = pendingReceiptsList
	node.pendingCXMutex.Unlock()

	utils.Logger().Debug().Msgf("[proposeReceiptsProof] number of validReceipts %d", len(validReceiptsList))
	return validReceiptsList
}
