package core

import (
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/utils"
)

// PruneBlocks deletes all blocks from the blockchain that are further
// back than the provided amountToPreserve
func (bc *BlockChain) PruneBlocks(amountToPeserve uint64) {
	currentBlock := bc.CurrentBlock()
	if currentBlock == nil {
		return
	}

	blockNumberToDelete := currentBlock.NumberU64() - amountToPeserve

	utils.Logger().Info().Msgf("Pruning all blocks before block number %d\n", blockNumberToDelete)

	for {
		blockNumberToDelete--
		if blockNumberToDelete%10000 == 0 {
			utils.Logger().Info().Msgf("Pruned back to block number %d. Continuing\n", blockNumberToDelete)
		}
		if !bc.PruneBlock(blockNumberToDelete) {
			break
		}
	}

	utils.Logger().Info().Msgf("Finished pruning blocks\n")
}

// PruneBlock deletes the block with a given block number.
// Returns true if successful and false if it didn't find a block
func (bc *BlockChain) PruneBlock(blockNumber uint64) bool {
	utils.Logger().Debug().Msgf("Pruning block number %d\n", blockNumber)

	blockToDelete := bc.GetBlockByNumber(blockNumber)

	if blockToDelete == nil {
		utils.Logger().Info().Msgf("Did not find block number %d to prune\n", blockNumber)
		return false
	} else if blockToDelete.NumberU64() == bc.Genesis().NumberU64() {
		utils.Logger().Info().Msgf("Requested to prune genesis block number %d. Cancelling deletion.\n", blockNumber)
		return false
	}

	rawdb.DeleteBlock(bc.ChainDb(), blockToDelete.Hash(), blockToDelete.NumberU64())

	utils.Logger().Debug().Msgf("Pruned block number %d\n", blockNumber)
	return true
}
