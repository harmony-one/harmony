package txpool

import (
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

const (
	// TxPoolLimit is the limit of transaction pool.		// TxPoolLimit is the limit of transaction pool.
	TxPoolLimit = 20000
	// MaxTxAmountLimit is the limit of amount of transaction value.
	MaxTxAmountLimit = 1000
	// MaxNumRecentTxsPerAccountLimit is the limit of max recent txs per account.
	MaxNumRecentTxsPerAccountLimit = 1000
)

// DefaultSigner is homestead signer.
var DefaultSigner = types.HomesteadSigner{}

// HmyTxPool ...
type HmyTxPool struct {
	recentTxsStats      map[uint64]map[common.Address]int
	BlockPeriod         time.Duration
	blockNumHourAgo     uint64
	pendingTxMutex      sync.Mutex
	pendingTransactions types.Transactions // All the transactions received but not yet processed for Consensus
	chainReader         *core.BlockChain
}

// NewHmyTxPool returns an object of HmyTxPool.
func NewHmyTxPool(blockTime time.Duration, chainReader *core.BlockChain) *HmyTxPool {
	return &HmyTxPool{
		recentTxsStats:  make(map[uint64]map[common.Address]int),
		blockNumHourAgo: uint64(time.Hour / blockTime),
		chainReader:     chainReader,
	}
}

func valueInWei(ones int) *big.Int {
	return big.NewInt(int64(ones)).Mul(big.NewInt(int64(ones)), big.NewInt(1e18))
}

func (pool *HmyTxPool) cleanUpRecentTxs() {
	for blockNum := range pool.recentTxsStats {
		if blockNum < pool.chainReader.CurrentHeader().Number.Uint64()-pool.blockNumHourAgo {
			delete(pool.recentTxsStats, blockNum)
		}
	}
}

func (pool *HmyTxPool) removeOverLimitAmountTxs(newTxs types.Transactions) types.Transactions {
	res := types.Transactions{}
	for _, tx := range newTxs {
		if tx.Value().Cmp(valueInWei(MaxTxAmountLimit)) > 0 {
			utils.GetLogInstance().Info("Throttling tx with max amount limit",
				"tx Id", tx.Hash().Hex(),
				"MaxTxAmountLimit", valueInWei(MaxTxAmountLimit),
				"Tx amount", tx.Value())
		} else {
			res = append(res, tx)
		}
	}
	return res
}

func (pool *HmyTxPool) numTxsPastHour(sender common.Address) int {
	res := 0
	for _, stat := range pool.recentTxsStats {
		if val, ok := stat[sender]; ok {
			res += val
		}
	}
	return res
}

func (pool *HmyTxPool) removeByNumTxsPastHour(newTxs types.Transactions) types.Transactions {
	res := types.Transactions{}
	for _, tx := range newTxs {
		if sender, err := types.Sender(DefaultSigner, tx); err == nil {
			if pool.numTxsPastHour(sender) < MaxNumRecentTxsPerAccountLimit {
				res = append(res, tx)
			}
		}
	}
	return res
}

// GetTxPoolSize returns tx pool size.
func (pool *HmyTxPool) GetTxPoolSize() int {
	return len(pool.pendingTransactions)
}

// AddPendingTransactions add new transactions to the pending transaction list.
func (pool *HmyTxPool) AddPendingTransactions(newTxs types.Transactions) {
	newTxs = pool.removeOverLimitAmountTxs(newTxs)
	newTxs = pool.removeByNumTxsPastHour(newTxs)
	pool.pendingTxMutex.Lock()
	defer pool.pendingTxMutex.Unlock()

	pool.cleanUpRecentTxs()
	pool.pendingTransactions = append(pool.pendingTransactions, newTxs...)
	pool.reducePendingTransactions()
	utils.Logger().Info().Int("num", len(newTxs)).Int("totalPending", len(pool.pendingTransactions)).Msg("Got more transactions")
}

// AddPendingTransaction adds one new transaction to the pending transaction list.
func (pool *HmyTxPool) AddPendingTransaction(newTx *types.Transaction) {
	pool.AddPendingTransactions(types.Transactions{newTx})
	utils.Logger().Error().Int("totalPending", len(pool.pendingTransactions)).Msg("Got ONE more transaction")
}

func (pool *HmyTxPool) reducePendingTransactions() {
	// If length of pendingTransactions is greater than TxPoolLimit then by greedy take the TxPoolLimit recent transactions.
	if len(pool.pendingTransactions) > TxPoolLimit+TxPoolLimit {
		curLen := len(pool.pendingTransactions)
		pool.pendingTransactions = append(types.Transactions(nil), pool.pendingTransactions[curLen-TxPoolLimit:]...)
		utils.GetLogger().Info("mem stat reduce pending transaction")
	}
}
