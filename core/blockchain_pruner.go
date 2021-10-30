package core

import (
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

const (
	logPruningProgressBatchCount = 10000
	PreserveBlockAmount          = 262144 // Preserve 8 epochs of blocks (2^15) * 8
)

// InitialBlockPruning deletes all blocks from the blockchain backwards
// from the current block preserving 8 epochs worth of blocks
func (bc *BlockChain) InitialBlockPruning() {
	blockNumberToDelete := bc.GetInitialPruningStartBlockNum()
	if blockNumberToDelete == 0 {
		utils.Logger().Info().Msgf("Initial pruning canceled - start number is 0\n")
		return
	}
	utils.Logger().Info().Msgf("Pruning all blocks before block number %d\n", blockNumberToDelete)
	numDeletedBlocksThisBatch := 0
	for {
		blockNumberToDelete--
		pruned := bc.PruneBlock(blockNumberToDelete)
		logProgress := blockNumberToDelete%logPruningProgressBatchCount == 0
		if pruned {
			numDeletedBlocksThisBatch++
		}
		// If we haven't deleted any blocks yet this batch then we're done
		if logProgress && numDeletedBlocksThisBatch == 0 {
			utils.Logger().Info().Msgf("No blocks were pruned between %d and %d. Done pruning\n", blockNumberToDelete+logPruningProgressBatchCount, blockNumberToDelete)
			break
		}
		if logProgress {
			numDeletedBlocksThisBatch = 0
			utils.Logger().Info().Msgf("Pruned back to block number %d. Continuing\n", blockNumberToDelete)
			rawdb.WriteBlockPruningState(bc.ChainDb(), bc.ShardID(), blockNumberToDelete)
			rawdb.CompactPrunedBlocks(bc.ChainDb(), bc.ShardID(), blockNumberToDelete, blockNumberToDelete+logPruningProgressBatchCount)
			if blockNumberToDelete == 0 {
				break
			}
		}
	}

	utils.Logger().Info().Msgf("Finished pruning blocks\n")
}

// PruneBlock deletes the block with a given block number. Returns bool for whether a block was deleted.
func (bc *BlockChain) PruneBlock(blockNumber uint64) bool {
	if blockNumber == 0 { // do not prune genesis block
		return false
	}

	utils.Logger().Debug().Msgf("Pruning block number %d\n", blockNumber)

	blockHash := rawdb.ReadCanonicalHash(bc.ChainDb(), blockNumber)
	blockToDelete := rawdb.ReadBlock(bc.ChainDb(), blockHash, blockNumber)
	if blockToDelete == nil {
		utils.Logger().Debug().Msgf("Did not find block number %d to prune\n", blockNumber)
		return false
	}

	deleteAllBlockData(bc.ChainDb(), bc.ShardID(), blockToDelete)

	utils.Logger().Debug().Msgf("Pruned block number %d\n", blockNumber)

	// If this was pruned outside of initial pruning, trigger compaction if necessary
	if blockNumber > bc.GetInitialPruningStartBlockNum() && bc.ShouldDoCompaction(blockNumber) {
		rawdb.CompactPrunedBlocks(bc.ChainDb(), bc.ShardID(), blockNumber-logPruningProgressBatchCount, blockNumber)
	}
	return true
}

func deleteAllBlockData(db ethdb.Database, shardID uint32, blockToDelete *types.Block) {
	for _, tx := range blockToDelete.Transactions() {
		rawdb.DeleteTxLookupEntry(db, tx.Hash())
	}
	for _, stx := range blockToDelete.StakingTransactions() {
		rawdb.DeleteTxLookupEntry(db, stx.Hash())
	}
	for _, cx := range blockToDelete.IncomingReceipts() {
		rawdb.DeleteCXReceiptsProofSpent(db, shardID, cx.MerkleProof.BlockNum.Uint64())
		rawdb.DeleteCxReceipts(db, shardID, blockToDelete.NumberU64(), blockToDelete.Hash())
		for _, cxp := range cx.Receipts {
			rawdb.DeleteCxLookupEntry(db, cxp.TxHash)
		}
	}

	rawdb.DeleteBlock(db, blockToDelete.Hash(), blockToDelete.NumberU64())
}

func DetermineInitialPruningStartingBlockNumber(db rawdb.DatabaseReader, shardID uint32, currentBlockNumber uint64, amountToPreserve uint64) uint64 {
	if currentBlockNumber <= amountToPreserve {
		return 0
	}
	startingBlockNumber := currentBlockNumber - amountToPreserve

	blockPruningStateNum := rawdb.ReadBlockPruningState(db, shardID)

	// If there's no block num in db or the block num in db is greater
	// than it should be, return the value based on current block num
	if blockPruningStateNum == nil || *blockPruningStateNum > startingBlockNumber {
		return startingBlockNumber
	} else {
		return *blockPruningStateNum
	}
}

func (bc *BlockChain) SetInitialPruningStartBlockNum(blockNum uint64) {
	bc.initialPruningStartBlockNum = blockNum
}

func (bc *BlockChain) GetInitialPruningStartBlockNum() uint64 {
	return bc.initialPruningStartBlockNum
}

func (bc *BlockChain) ShouldDoCompaction(prunedBlockNumber uint64) bool {
	return prunedBlockNumber%logPruningProgressBatchCount == 0
}
