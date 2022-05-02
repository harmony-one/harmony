package hmy

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
)

// GetPoolStats returns the number of pending and queued transactions
func (hmy *Harmony) GetPoolStats() (pendingCount, queuedCount int) {
	return hmy.TxPool.Stats()
}

// GetPoolNonce ...
func (hmy *Harmony) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return hmy.TxPool.State().Get(addr), nil
}

// GetPoolTransaction ...
func (hmy *Harmony) GetPoolTransaction(hash common.Hash) types.PoolTransaction {
	return hmy.TxPool.Get(hash)
}

// GetPendingCXReceipts ..
func (hmy *Harmony) GetPendingCXReceipts() []*types.CXReceiptsProof {
	return hmy.NodeAPI.PendingCXReceipts()
}

// GetPoolTransactions returns pool transactions.
func (hmy *Harmony) GetPoolTransactions() (types.PoolTransactions, error) {
	pending := hmy.TxPool.Pending()
	queued := hmy.TxPool.Queued()

	var txs types.PoolTransactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	for _, batch := range queued {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (hmy *Harmony) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return hmy.gpo.SuggestPrice(ctx)
}
