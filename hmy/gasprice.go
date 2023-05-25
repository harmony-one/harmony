// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package hmy

import (
	"context"
	"math/big"
	"sort"
	"sync"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/internal/configs/harmony"
	"github.com/harmony-one/harmony/internal/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core/types"
)

// OracleBackend includes all necessary background APIs for oracle.
type OracleBackend interface {
	HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*block.Header, error)
	BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error)
	ChainConfig() *params.ChainConfig
}

// Oracle recommends gas prices based on the content of recent blocks.
type Oracle struct {
	backend   *Harmony
	lastHead  common.Hash
	lastPrice *big.Int
	maxPrice  *big.Int
	cacheLock sync.RWMutex
	fetchLock sync.Mutex

	checkBlocks       int
	percentile        int
	checkTxs          int
	lowUsageThreshold float64
	blockGasLimit     int
	defaultPrice      *big.Int
}

var DefaultGPOConfig = harmony.GasPriceOracleConfig{
	Blocks:            20,
	Transactions:      3,
	Percentile:        60,
	DefaultPrice:      100 * denominations.Nano,  // 100 gwei
	MaxPrice:          1000 * denominations.Nano, // 1000 gwei
	LowUsageThreshold: 50,
	BlockGasLimit:     0, // TODO should we set default to 30M?
}

// NewOracle returns a new gasprice oracle which can recommend suitable
// gasprice for newly created transaction.
func NewOracle(backend *Harmony, params *harmony.GasPriceOracleConfig) *Oracle {
	blocks := params.Blocks
	if blocks < 1 {
		blocks = DefaultGPOConfig.Blocks
		utils.Logger().Warn().
			Int("provided", params.Blocks).
			Int("updated", blocks).
			Msg("Sanitizing invalid gasprice oracle sample blocks")
	}
	txs := params.Transactions
	if txs < 1 {
		txs = DefaultGPOConfig.Transactions
		utils.Logger().Warn().
			Int("provided", params.Transactions).
			Int("updated", txs).
			Msg("Sanitizing invalid gasprice oracle sample transactions")
	}
	percentile := params.Percentile
	if percentile < 0 || percentile > 100 {
		percentile = DefaultGPOConfig.Percentile
		utils.Logger().Warn().
			Int("provided", params.Percentile).
			Int("updated", percentile).
			Msg("Sanitizing invalid gasprice oracle percentile")
	}
	// no sanity check done, simply convert it
	defaultPrice := big.NewInt(params.DefaultPrice)
	maxPrice := big.NewInt(params.MaxPrice)
	lowUsageThreshold := float64(params.LowUsageThreshold) / 100.0
	if lowUsageThreshold < 0 || lowUsageThreshold > 1 {
		lowUsageThreshold = float64(DefaultGPOConfig.LowUsageThreshold) / 100.0
		utils.Logger().Warn().
			Float64("provided", float64(params.LowUsageThreshold)/100.0).
			Float64("updated", lowUsageThreshold).
			Msg("Sanitizing invalid gasprice oracle lowUsageThreshold")
	}
	blockGasLimit := params.BlockGasLimit
	return &Oracle{
		backend:           backend,
		lastPrice:         defaultPrice,
		maxPrice:          maxPrice,
		checkBlocks:       blocks,
		percentile:        percentile,
		checkTxs:          txs,
		lowUsageThreshold: lowUsageThreshold,
		blockGasLimit:     blockGasLimit,
		// do not reference lastPrice
		defaultPrice: new(big.Int).Set(defaultPrice),
	}
}

// SuggestPrice returns a gasprice so that newly created transaction can
// have a very high chance to be included in the following blocks.
func (gpo *Oracle) SuggestPrice(ctx context.Context) (*big.Int, error) {
	head, _ := gpo.backend.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	headHash := head.Hash()

	// If the latest gasprice is still available, return it.
	gpo.cacheLock.RLock()
	lastHead, lastPrice := gpo.lastHead, gpo.lastPrice
	gpo.cacheLock.RUnlock()
	if headHash == lastHead {
		return lastPrice, nil
	}
	gpo.fetchLock.Lock()
	defer gpo.fetchLock.Unlock()

	// Try checking the cache again, maybe the last fetch fetched what we need
	gpo.cacheLock.RLock()
	lastHead, lastPrice = gpo.lastHead, gpo.lastPrice
	gpo.cacheLock.RUnlock()
	if headHash == lastHead {
		return lastPrice, nil
	}
	var (
		sent, exp int
		number    = head.Number().Uint64()
		result    = make(chan getBlockPricesResult, gpo.checkBlocks)
		quit      = make(chan struct{})
		txPrices  []*big.Int
		usageSum  float64
	)
	for sent < gpo.checkBlocks && number > 0 {
		go gpo.getBlockPrices(ctx, types.MakeSigner(gpo.backend.ChainConfig(), big.NewInt(int64(number))), number, gpo.checkTxs, result, quit)
		sent++
		exp++
		number--
	}
	for exp > 0 {
		res := <-result
		if res.err != nil {
			close(quit)
			return lastPrice, res.err
		}
		exp--
		// Nothing returned. There are two special cases here:
		// - The block is empty
		// - All the transactions included are sent by the miner itself.
		// In these cases, use the latest calculated price for samping.
		if len(res.prices) == 0 {
			res.prices = []*big.Int{lastPrice}
		}
		// Besides, in order to collect enough data for sampling, if nothing
		// meaningful returned, try to query more blocks. But the maximum
		// is 2*checkBlocks.
		if len(res.prices) == 1 && len(txPrices)+1+exp < gpo.checkBlocks*2 && number > 0 {
			go gpo.getBlockPrices(ctx, types.MakeSigner(gpo.backend.ChainConfig(), big.NewInt(int64(number))), number, gpo.checkTxs, result, quit)
			sent++
			exp++
			number--
		}
		txPrices = append(txPrices, res.prices...)
		usageSum += res.usage
	}
	price := lastPrice
	if len(txPrices) > 0 {
		sort.Sort(bigIntArray(txPrices))
		price = txPrices[(len(txPrices)-1)*gpo.percentile/100]
	}
	// `sent` is the number of queries that are sent, while `exp` and `number` count down at query resolved, and sent respectively
	// each query is per block, therefore `sent` is the number of blocks for which the usage was (successfully) determined
	// approximation that only holds when the gas limits, of all blocks that are sampled, are equal
	usage := usageSum / float64(sent)
	if usage < gpo.lowUsageThreshold {
		price = new(big.Int).Set(gpo.defaultPrice)
	}
	if price.Cmp(gpo.maxPrice) > 0 {
		price = new(big.Int).Set(gpo.maxPrice)
	}
	gpo.cacheLock.Lock()
	gpo.lastHead = headHash
	gpo.lastPrice = price
	gpo.cacheLock.Unlock()
	return price, nil
}

type getBlockPricesResult struct {
	prices []*big.Int
	usage  float64
	err    error
}

type transactionsByGasPrice []*types.Transaction

func (t transactionsByGasPrice) Len() int      { return len(t) }
func (t transactionsByGasPrice) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t transactionsByGasPrice) Less(i, j int) bool {
	return t[i].GasPrice().Cmp(t[j].GasPrice()) < 0
}

// getBlockPrices calculates the lowest transaction gas price in a given block
// and sends it to the result channel. If the block is empty or all transactions
// are sent by the miner itself(it doesn't make any sense to include this kind of
// transaction prices for sampling), nil gasprice is returned.
func (gpo *Oracle) getBlockPrices(ctx context.Context, signer types.Signer, blockNum uint64, limit int, result chan getBlockPricesResult, quit chan struct{}) {
	block, err := gpo.backend.BlockByNumber(ctx, rpc.BlockNumber(blockNum))
	if block == nil {
		select {
		// when no block is found, `err` is returned which short-circuits the oracle entirely
		// therefore the `usage` becomes irrelevant
		case result <- getBlockPricesResult{nil, 2.0, err}:
		case <-quit:
		}
		return
	}
	blockTxs := block.Transactions()
	txs := make([]*types.Transaction, len(blockTxs))
	copy(txs, blockTxs)
	sort.Sort(transactionsByGasPrice(txs))

	var prices []*big.Int
	for _, tx := range txs {
		sender, err := types.Sender(signer, tx)
		if err == nil && sender != block.Coinbase() {
			prices = append(prices, tx.GasPrice())
			if len(prices) >= limit {
				break
			}
		}
	}
	// HACK
	var gasLimit float64
	if gpo.blockGasLimit == 0 {
		gasLimit = float64(block.GasLimit())
	} else {
		gasLimit = float64(gpo.blockGasLimit)
	}
	// if `gasLimit` is 0, no crash. +Inf is returned and percentile is applied
	// this usage includes any transactions from the miner, which are excluded by the `prices` slice
	usage := float64(block.GasUsed()) / gasLimit
	select {
	case result <- getBlockPricesResult{prices, usage, nil}:
	case <-quit:
	}
}

type bigIntArray []*big.Int

func (s bigIntArray) Len() int           { return len(s) }
func (s bigIntArray) Less(i, j int) bool { return s[i].Cmp(s[j]) < 0 }
func (s bigIntArray) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
