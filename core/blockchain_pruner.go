package core

import (
	"math/big"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

const (
	// pruneBeaconChainBeforeEpoch Keep 16 epoch
	pruneBeaconChainBeforeEpoch = 16
	// pruneBeaconChainBlockBefore Keep 16 epoch blocks
	pruneBeaconChainBlockBefore = 32768 * pruneBeaconChainBeforeEpoch
	// maxDeleteBlockOnce max delete block once
	maxDeleteBlockOnce = 10000
	// compactProportion compact proportion is thousandth
	compactProportion = 1000
)

type blockchainPruner struct {
	isPruneBeaconChainRunning int32 // isPruneBeaconChainRunning  must be called atomically

	db          ethdb.Database
	batchWriter ethdb.Batch

	compactRange             [][2][]byte
	startTime                time.Time
	deletedBlockCount        int
	deletedValidatorSnapshot int
	skipValidatorSnapshot    int
}

func newBlockchainPruner(db ethdb.Database) *blockchainPruner {
	return &blockchainPruner{
		db: db,
	}
}

// Put inserts the given value into the key-value data store.
func (bp *blockchainPruner) Put(key []byte, value []byte) error {
	return nil
}

func (bp *blockchainPruner) Delete(key []byte) error {
	err := bp.batchWriter.Delete(key)
	if err != nil {
		return err
	}

	if bp.batchWriter.ValueSize() >= ethdb.IdealBatchSize {
		err = bp.batchWriter.Write()
		if err != nil {
			return err
		}

		bp.batchWriter.Reset()
	}
	return nil
}

func (bp *blockchainPruner) resetBatchWriter() error {
	err := bp.batchWriter.Write()
	if err != nil {
		return err
	}

	bp.batchWriter = nil
	return nil
}

func (bp *blockchainPruner) runWhenNotPruning(cb func() error) error {
	// check is start
	if atomic.CompareAndSwapInt32(&bp.isPruneBeaconChainRunning, 0, 1) {
		defer atomic.StoreInt32(&bp.isPruneBeaconChainRunning, 0)

		return cb()
	}
	return nil
}

func (bp *blockchainPruner) Start(maxBlockNum uint64, maxEpoch *big.Int) error {
	return bp.runWhenNotPruning(func() error {
		// init
		bp.compactRange = make([][2][]byte, 0)
		bp.startTime = time.Now()
		bp.deletedBlockCount = 0
		bp.deletedValidatorSnapshot = 0
		bp.skipValidatorSnapshot = 0
		bp.batchWriter = bp.db.NewBatch()

		// prune data
		bp.addToCompactRange(bp.pruneBeaconChainBlock(maxBlockNum))
		bp.addToCompactRange(bp.pruneBeaconChainValidatorSnapshot(maxEpoch))

		//  batch write data and reset
		err := bp.resetBatchWriter()
		if err != nil {
			return err
		}

		prunerMaxBlock.Set(float64(maxBlockNum))
		deletedValidatorSnapshot.Add(float64(bp.deletedValidatorSnapshot))
		skipValidatorSnapshot.Add(float64(bp.skipValidatorSnapshot))
		deletedBlockCount.Add(float64(bp.deletedBlockCount))
		deletedBlockCountUsedTime.Add(float64(time.Now().Sub(bp.startTime).Milliseconds()))

		utils.Logger().Info().
			Uint64("maxBlockNum", maxBlockNum).
			Uint64("maxEpoch", maxEpoch.Uint64()).
			Int("deletedBlockCount", bp.deletedBlockCount).
			Int("deletedValidatorSnapshot", bp.deletedValidatorSnapshot).
			Int("skipValidatorSnapshot", bp.skipValidatorSnapshot).
			Dur("cost", time.Now().Sub(bp.startTime)).
			Msg("pruneBeaconChain delete block success")

		// probability of 1 in 1000 blocks to Compact, It consumes a lot of IO
		if rand.Intn(compactProportion) == 1 {
			startTime := time.Now()
			for _, compactStartEnd := range bp.compactRange {
				err := bp.db.Compact(compactStartEnd[0], compactStartEnd[1])
				if err != nil {
					return err
				}
			}
			compactBlockCountUsedTime.Add(float64(time.Now().Sub(startTime).Milliseconds()))
			utils.Logger().Info().
				Uint64("maxBlockNum", maxBlockNum).
				Uint64("maxEpoch", maxEpoch.Uint64()).
				Dur("cost", time.Now().Sub(startTime)).
				Msg("pruneBeaconChain compact db success")
		}

		return nil
	})
}

func (bp *blockchainPruner) pruneBeaconChainBlock(maxBlockNum uint64) (minKey []byte, maxKey []byte) {
	return rawdb.IteratorBlocks(bp.db, func(blockNum uint64, hash common.Hash) bool {
		if blockNum >= maxBlockNum {
			return false
		}

		// skip genesis block
		if blockNum == 0 {
			return true
		}

		blockInfo := rawdb.ReadBlock(bp.db, hash, blockNum)
		if blockInfo == nil {
			return true
		}

		err := bp.deleteBlockInfo(blockInfo)
		if err != nil {
			utils.Logger().Error().
				Uint64("blockNum", blockNum).
				AnErr("err", err).
				Msg("pruneBeaconChain delete block info error")
			return false
		}

		err = rawdb.DeleteBlock(bp, hash, blockNum)
		if err != nil {
			utils.Logger().Error().
				Uint64("blockNum", blockNum).
				AnErr("err", err).
				Msg("pruneBeaconChain delete block error")
			return false
		}

		// limit time spent
		bp.deletedBlockCount++
		return bp.deletedBlockCount < maxDeleteBlockOnce
	})
}

func (bp *blockchainPruner) deleteBlockInfo(info *types.Block) error {
	for _, cx := range info.IncomingReceipts() {
		for _, cxReceipt := range cx.Receipts {
			err := rawdb.DeleteCxLookupEntry(bp, cxReceipt.TxHash)
			if err != nil {
				return err
			}
		}
	}

	for _, tx := range info.Transactions() {
		err := rawdb.DeleteTxLookupEntry(bp, tx.Hash())
		if err != nil {
			return err
		}
	}

	for _, stx := range info.StakingTransactions() {
		err := rawdb.DeleteTxLookupEntry(bp, stx.Hash())
		if err != nil {
			return err
		}
	}

	return nil
}

func (bp *blockchainPruner) pruneBeaconChainValidatorSnapshot(maxEpoch *big.Int) (minKey []byte, maxKey []byte) {
	return rawdb.IteratorValidatorSnapshot(bp.db, func(addr common.Address, epoch *big.Int) bool {
		if epoch.Cmp(maxEpoch) < 0 {
			err := rawdb.DeleteValidatorSnapshot(bp, addr, epoch)
			if err != nil {
				utils.Logger().Error().
					Str("addr", addr.Hex()).
					Uint64("epoch", epoch.Uint64()).
					AnErr("err", err).
					Msg("pruneBeaconChain delete validator snapshot error")
				return false
			}

			// limit time spent
			bp.deletedValidatorSnapshot++
		} else {
			bp.skipValidatorSnapshot++
		}

		return bp.deletedValidatorSnapshot < maxDeleteBlockOnce
	})
}

func (bp *blockchainPruner) addToCompactRange(minKey []byte, maxKey []byte) {
	bp.compactRange = append(bp.compactRange, [2][]byte{minKey, maxKey})
}
