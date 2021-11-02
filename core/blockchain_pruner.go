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
	highestBlockNumInBatch := blockNumberToDelete - 1
	for {
		blockNumberToDelete--
		pruned := bc.PruneBlock(blockNumberToDelete)
		finishedBatch := blockNumberToDelete%logPruningProgressBatchCount == 0
		if pruned {
			numDeletedBlocksThisBatch++
			if numDeletedBlocksThisBatch == 1 { // track the first deleted block for compaction later
				highestBlockNumInBatch = blockNumberToDelete
			}
		}
		if finishedBatch {
			utils.Logger().Info().Msgf("Pruned back to block number %d. %d blocks were pruned. Continuing\n", blockNumberToDelete, numDeletedBlocksThisBatch)
			rawdb.WriteBlockPruningState(bc.ChainDb(), bc.ShardID(), blockNumberToDelete)
			if numDeletedBlocksThisBatch > 0 { // if we deleted anything this batch, do compaction
				rawdb.CompactPrunedBlocks(bc.ChainDb(), bc.ShardID(), blockNumberToDelete, highestBlockNumInBatch)
			}
			if blockNumberToDelete == 0 {
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

	// If this was pruned outside of initial pruning, trigger compaction if necessary
	if blockNumber > bc.GetInitialPruningStartBlockNum() && bc.ShouldDoCompaction(blockNumber) {
		startBlockNum := blockNumber - logPruningProgressBatchCount
		if bc.GetInitialPruningStartBlockNum() > startBlockNum { // Prevent overlapping compaction with initial pruning
			startBlockNum = bc.GetInitialPruningStartBlockNum()
		}
		rawdb.CompactPrunedBlocks(bc.ChainDb(), bc.ShardID(), startBlockNum, blockNumber)
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

func DetermineInitialPruningStartingBlockNumber(db rawdb.DatabaseReader, currentBlockNumber uint64, amountToPreserve uint64) uint64 {
	if currentBlockNumber <= amountToPreserve {
		return 0
	}
	startingBlockNumber := currentBlockNumber - amountToPreserve

	blockPruningStateNum := rawdb.ReadBlockPruningState(db, 0)

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
