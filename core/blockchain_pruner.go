package core

import (
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/utils"
)

const (
	logProgressBatchCount = 10000
)

// PruneBlocks deletes all blocks from the blockchain that are further
// back than the provided amountToPreserve
func (bc *BlockChain) PruneBlocks(amountToPreserve uint64) {
	if bc.CurrentBlock() == nil { // exit early if this chain has no blocks
		return
	}
	blockNumberToDelete := getStartingBlockNumber(bc, amountToPreserve)

	utils.Logger().Info().Msgf("Pruning all blocks before block number %d\n", blockNumberToDelete)

	numDeletedBlocksThisBatch := 0
	for {
		blockNumberToDelete--
		pruned := bc.PruneBlock(blockNumberToDelete)
		logProgress := blockNumberToDelete%logProgressBatchCount == 0
		if pruned {
			numDeletedBlocksThisBatch++
		}
		// If we haven't deleted any blocks yet this batch then we're done
		if logProgress && numDeletedBlocksThisBatch == 0 {
			utils.Logger().Info().Msgf("No blocks were pruned between %d and %d. Done pruning\n", blockNumberToDelete+logProgressBatchCount, blockNumberToDelete)
			break
		}
		if logProgress {
			numDeletedBlocksThisBatch = 0
			utils.Logger().Info().Msgf("Pruned back to block number %d. Continuing\n", blockNumberToDelete)
			rawdb.WriteBlockPruningState(bc.ChainDb(), bc.ShardID(), blockNumberToDelete)
		}
	}

	utils.Logger().Info().Msgf("Finished pruning blocks\n")
}

// PruneBlock deletes the block with a given block number. Returns bool for whether a block was deleted.
func (bc *BlockChain) PruneBlock(blockNumber uint64) bool {
	utils.Logger().Debug().Msgf("Pruning block number %d\n", blockNumber)

	blockToDelete := bc.GetBlockByNumber(blockNumber)

	if blockToDelete == nil {
		utils.Logger().Debug().Msgf("Did not find block number %d to prune\n", blockNumber)
		return false
	} else if blockToDelete.NumberU64() == bc.Genesis().NumberU64() {
		utils.Logger().Info().Msgf("Requested to prune genesis block number %d. Cancelling deletion.\n", blockNumber)
		return false
	}

	for _, tx := range blockToDelete.Transactions() {
		rawdb.DeleteTxLookupEntry(bc.ChainDb(), tx.Hash())
	}
	for _, stx := range blockToDelete.StakingTransactions() {
		rawdb.DeleteTxLookupEntry(bc.ChainDb(), stx.Hash())
	}
	for _, cx := range blockToDelete.IncomingReceipts() {
		rawdb.DeleteCXReceiptsProofSpent(bc.ChainDb(), bc.ShardID(), cx.MerkleProof.BlockNum.Uint64())
		rawdb.DeleteCxReceipts(bc.ChainDb(), bc.ShardID(), blockToDelete.NumberU64(), blockToDelete.Hash())
		for _, cxp := range cx.Receipts {
			rawdb.DeleteCxLookupEntry(bc.ChainDb(), cxp.TxHash)
		}
	}

	rawdb.DeleteBlock(bc.ChainDb(), blockToDelete.Hash(), blockToDelete.NumberU64())

	utils.Logger().Debug().Msgf("Pruned block number %d\n", blockNumber)
	return true
}

func getStartingBlockNumber(bc *BlockChain, amountToPreserve uint64) uint64 {
	startingBlockNumber := bc.CurrentBlock().NumberU64() - amountToPreserve

	blockPruningStateNum := rawdb.ReadBlockPruningState(bc.ChainDb(), bc.ShardID())

	// If there's no block num in db or the block num in db is greater
	// than it should be, return the value based on current block num
	if blockPruningStateNum == nil || *blockPruningStateNum > startingBlockNumber {
		return startingBlockNumber
	} else {
		return *blockPruningStateNum
	}
}
