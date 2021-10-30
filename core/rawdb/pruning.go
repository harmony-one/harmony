package rawdb

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/utils"
)

type pruneKeys struct {
	BlockReceiptsKey  []byte
	HeaderKey         []byte
	HeaderNumberKey   []byte
	BlockBodyKey      []byte
	HeaderTDKey       []byte
	CxReceiptSpentKey []byte
	CxReceiptKey      []byte
}

func getPruneKeys(shardID uint32, blockNumber uint64) *pruneKeys {
	return &pruneKeys{
		BlockReceiptsKey:  blockReceiptsKey(blockNumber, common.Hash{}),
		HeaderKey:         headerKey(blockNumber, common.Hash{}),
		HeaderNumberKey:   headerNumberKey(common.Hash{}),
		BlockBodyKey:      blockBodyKey(blockNumber, common.Hash{}),
		HeaderTDKey:       headerTDKey(blockNumber, common.Hash{}),
		CxReceiptSpentKey: cxReceiptSpentKey(shardID, blockNumber),
		CxReceiptKey:      cxReceiptKey(shardID, blockNumber, common.Hash{}),
	}
}

func CompactPrunedBlocks(db ethdb.Compacter, shardID uint32, oldestBlockNumber uint64, newestBlockNumber uint64) {
	utils.Logger().Info().Msgf("Compacting db from block numbers %d to %d\n", oldestBlockNumber, newestBlockNumber)
	oldestPruneKeys := getPruneKeys(shardID, oldestBlockNumber)
	newestPruneKeys := getPruneKeys(shardID, uint64(newestBlockNumber))
	db.Compact(oldestPruneKeys.BlockReceiptsKey, newestPruneKeys.BlockReceiptsKey)
	db.Compact(oldestPruneKeys.HeaderKey, newestPruneKeys.HeaderKey)
	db.Compact(oldestPruneKeys.HeaderNumberKey, newestPruneKeys.HeaderNumberKey)
	db.Compact(oldestPruneKeys.BlockBodyKey, newestPruneKeys.BlockBodyKey)
	db.Compact(oldestPruneKeys.HeaderTDKey, newestPruneKeys.HeaderTDKey)
	db.Compact(oldestPruneKeys.CxReceiptSpentKey, newestPruneKeys.CxReceiptSpentKey)
	db.Compact(oldestPruneKeys.CxReceiptKey, newestPruneKeys.CxReceiptKey)
	utils.Logger().Info().Msgf("Finished compacting db from block numbers %d to %d\n", oldestBlockNumber, newestBlockNumber)
}
