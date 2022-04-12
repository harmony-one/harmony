package hmy

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/eth/rpc"
)

// SendTx ...
func (hmy *Harmony) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	tx, _, _, _ := rawdb.ReadTransaction(hmy.chainDb, signedTx.Hash())
	if tx == nil {
		return hmy.NodeAPI.AddPendingTransaction(signedTx)
	}
	return ErrFinalizedTransaction
}

// ResendCx retrieve blockHash from txID and add blockHash to CxPool for resending
// Note that cross shard txn is only for regular txns, not for staking txns, so the input txn hash
// is expected to be regular txn hash
func (hmy *Harmony) ResendCx(ctx context.Context, txID common.Hash) (uint64, bool) {
	blockHash, blockNum, index := hmy.BlockChain.ReadTxLookupEntry(txID)
	if blockHash == (common.Hash{}) {
		return 0, false
	}

	blk := hmy.BlockChain.GetBlockByHash(blockHash)
	if blk == nil {
		return 0, false
	}

	txs := blk.Transactions()
	// a valid index is from 0 to len-1
	if int(index) > len(txs)-1 {
		return 0, false
	}
	tx := txs[int(index)]

	// check whether it is a valid cross shard tx
	if tx.ShardID() == tx.ToShardID() || blk.Header().ShardID() != tx.ShardID() {
		return 0, false
	}
	entry := core.CxEntry{BlockHash: blockHash, ToShardID: tx.ToShardID()}
	success := hmy.CxPool.Add(entry)
	return blockNum, success
}

// GetReceipts ...
func (hmy *Harmony) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return hmy.BlockChain.GetReceiptsByHash(hash), nil
}

// GetTransactionsHistory returns list of transactions hashes of address.
func (hmy *Harmony) GetTransactionsHistory(address, txType, order string) ([]common.Hash, error) {
	return hmy.NodeAPI.GetTransactionsHistory(address, txType, order)
}

// GetAccountNonce returns the nonce value of the given address for the given block number
func (hmy *Harmony) GetAccountNonce(
	ctx context.Context, address common.Address, blockNum rpc.BlockNumber) (uint64, error) {
	state, _, err := hmy.StateAndHeaderByNumber(ctx, blockNum)
	if state == nil || err != nil {
		return 0, err
	}
	return state.GetNonce(address), state.Error()
}

// GetTransactionsCount returns the number of regular transactions of address.
func (hmy *Harmony) GetTransactionsCount(address, txType string) (uint64, error) {
	return hmy.NodeAPI.GetTransactionsCount(address, txType)
}

// GetCurrentTransactionErrorSink ..
func (hmy *Harmony) GetCurrentTransactionErrorSink() types.TransactionErrorReports {
	return hmy.NodeAPI.ReportPlainErrorSink()
}
