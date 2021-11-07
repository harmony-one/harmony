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

// InitialBlockPruning deletes all blocks from the blockchain between the given
// startBlockNum and endBlockNum, ensuring we preserve 8 epochs worth of blocks
func (bc *BlockChain) InitialBlockPruning(startBlockNum, endBlockNum uint64) {
	if startBlockNum+1 >= endBlockNum {
		utils.Logger().Info().Msgf("Initial pruning canceled. StartBlockNum %d EndBlockNum %d\n", startBlockNum, endBlockNum)
		return
	}
	utils.Logger().Info().Msgf("Pruning all blocks after block number %d and before block number %d\n", startBlockNum, endBlockNum)
	numDeletedBlocksThisBatch := 0
	lowestBlockNumInBatch := startBlockNum
	blockNumberToDelete := startBlockNum
	for {
		blockNumberToDelete++
		pruned := bc.PruneBlock(blockNumberToDelete)
		finishedBatch := blockNumberToDelete%logPruningProgressBatchCount == 0 || blockNumberToDelete == endBlockNum
		if pruned {
			numDeletedBlocksThisBatch++
			if numDeletedBlocksThisBatch == 1 { // track the first deleted block for compaction later
				lowestBlockNumInBatch = blockNumberToDelete
			}
		}
		if finishedBatch {
			utils.Logger().Info().Msgf("Pruned up to block number %d. %d blocks were pruned. Continuing\n", blockNumberToDelete, numDeletedBlocksThisBatch)
			rawdb.WriteBlockPruningState(bc.ChainDb(), bc.ShardID(), blockNumberToDelete)
			if numDeletedBlocksThisBatch > 0 { // if we deleted anything this batch, do compaction
				bc.compactBatch(lowestBlockNumInBatch, blockNumberToDelete)
			}
			if blockNumberToDelete == endBlockNum {
				break
			}
			numDeletedBlocksThisBatch = 0
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

	bc.pruningMutex.Lock()
	defer bc.pruningMutex.Unlock()

	blockHash := rawdb.ReadCanonicalHash(bc.ChainDb(), blockNumber)
	blockToDelete := rawdb.ReadBlock(bc.ChainDb(), blockHash, blockNumber)
	if blockToDelete == nil {
		utils.Logger().Debug().Msgf("Did not find block number %d to prune\n", blockNumber)
		return false
	}

	deleteAllBlockData(bc.ChainDb(), bc.ShardID(), blockToDelete)

	utils.Logger().Debug().Msgf("Pruned block number %d\n", blockNumber)

	// If this was pruned outside of initial pruning, trigger writing of batch and compaction
	if blockNumber > bc.GetInitialPruningEndBlockNum() && ShouldDoCompaction(blockNumber) {
		startBlockNum := blockNumber - logPruningProgressBatchCount
		if bc.GetInitialPruningEndBlockNum() > startBlockNum { // Prevent overlapping compaction with initial pruning
			startBlockNum = bc.GetInitialPruningEndBlockNum()
		}
		bc.compactBatch(startBlockNum, blockNumber)
	}
	return true
}

func (bc *BlockChain) compactBatch(startBlockNum uint64, endBlockNum uint64) {
	bc.compactionMutex.Lock()
	rawdb.CompactPrunedBlocks(bc.ChainDb(), bc.ShardID(), startBlockNum, endBlockNum)
	bc.compactionMutex.Unlock()
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

func GetInitialPruningRange(db rawdb.DatabaseReader, currentBlockNumber uint64) (uint64, uint64) {
	startBlockNum := uint64(0)
	endBlockNum := uint64(0)
	if currentBlockNumber <= PreserveBlockAmount {
		return startBlockNum, endBlockNum
	}
	endBlockNum = currentBlockNumber - PreserveBlockAmount

	blockPruningStateNum := rawdb.ReadBlockPruningState(db, 0)
	if blockPruningStateNum != nil {
		startBlockNum = *blockPruningStateNum
	}

	// if the checkpoint value is somehow further than the end block, try to start pruning from the beginning again
	if startBlockNum > endBlockNum {
		startBlockNum = 0
	}

	return startBlockNum, endBlockNum
}

func (bc *BlockChain) SetInitialPruningEndBlockNum(blockNum uint64) {
	bc.initialPruningEndBlockNum = blockNum
}

func (bc *BlockChain) GetInitialPruningEndBlockNum() uint64 {
	return bc.initialPruningEndBlockNum
}

func ShouldDoCompaction(prunedBlockNumber uint64) bool {
	return prunedBlockNumber%logPruningProgressBatchCount == 0
}
