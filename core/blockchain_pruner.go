package core

import (
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/utils"
)

// PruneBlocks deletes all blocks from the blockchain that are further
// back than the provided amountToPreserve
func (bc *BlockChain) PruneBlocks(amountToPrune uint64) {
	currentBlock := bc.CurrentBlock()
	if currentBlock == nil {
		return
	}

	db := bc.ChainDb()

	blockNumberToDelete := currentBlock.NumberU64() - amountToPrune - 1
	blockToDelete := bc.GetBlockByNumber(blockNumberToDelete)

	if blockToDelete == nil {
		return
	}

	utils.Logger().Debug().Msgf("Deleting all blocks before block number %d\n", blockNumberToDelete)

	for {
		rawdb.DeleteBlock(db, blockToDelete.Hash(), blockToDelete.NumberU64())

		blockToDelete = bc.GetBlockByNumber(blockToDelete.NumberU64() - 1)

		if blockToDelete == nil || blockToDelete.NumberU64() == bc.Genesis().NumberU64() {
			break
		}
	}

	utils.Logger().Debug().Msgf("Finished deleting all blocks before block number %d\n", blockNumberToDelete)
}
