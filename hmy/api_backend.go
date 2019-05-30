package hmy

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
)

// APIBackend An implementation of Backend. Full client.
type APIBackend struct {
	hmy *Harmony
}

// ChainDb ...
func (b *APIBackend) ChainDb() ethdb.Database {
	return b.hmy.chainDb
}

// GetBlock ...
func (b *APIBackend) GetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.hmy.blockchain.GetBlockByHash(hash), nil
}

// GetPoolTransaction ...
func (b *APIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.hmy.txPool.Get(hash)
}

// BlockByNumber ...
func (b *APIBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		return nil, errors.New("not implemented")
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.hmy.blockchain.CurrentBlock(), nil
	}
	return b.hmy.blockchain.GetBlockByNumber(uint64(blockNr)), nil
}

// StateAndHeaderByNumber ...
func (b *APIBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.DB, *types.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		return nil, nil, errors.New("not implemented")
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := b.hmy.blockchain.StateAt(header.Root)
	return stateDb, header, err
}

// HeaderByNumber ...
func (b *APIBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		return nil, errors.New("not implemented")
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.hmy.blockchain.CurrentBlock().Header(), nil
	}
	return b.hmy.blockchain.GetHeaderByNumber(uint64(blockNr)), nil
}

// GetPoolNonce ...
func (b *APIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.hmy.txPool.State().GetNonce(addr), nil
}

// SendTx ...
func (b *APIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	// return b.hmy.txPool.Add(ctx, signedTx)
	b.hmy.nodeAPI.AddPendingTransaction(signedTx)
	return nil // TODO(ricl): AddPendingTransaction should return error
}

// ChainConfig ...
func (b *APIBackend) ChainConfig() *params.ChainConfig {
	return b.hmy.blockchain.Config()
}

// CurrentBlock ...
func (b *APIBackend) CurrentBlock() *types.Block {
	return types.NewBlockWithHeader(b.hmy.blockchain.CurrentHeader())
}

// AccountManager ...
func (b *APIBackend) AccountManager() *accounts.Manager {
	return b.hmy.accountManager
}

// GetReceipts ...
func (b *APIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.hmy.blockchain.GetReceiptsByHash(hash), nil
}

// EventMux ...
func (b *APIBackend) EventMux() *event.TypeMux { return b.hmy.eventMux }

// BloomStatus ...
func (b *APIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.hmy.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

// ProtocolVersion ...
func (b *APIBackend) ProtocolVersion() int {
	return proto.ProtocolVersion
}

// Filter related APIs

// GetLogs ...
func (b *APIBackend) GetLogs(ctx context.Context, blockHash common.Hash) ([][]*types.Log, error) {
	// TODO(ricl): implement
	return nil, nil
}

// HeaderByHash ...
func (b *APIBackend) HeaderByHash(ctx context.Context, blockHash common.Hash) (*types.Header, error) {
	// TODO(ricl): implement
	return nil, nil
}

// ServiceFilter ...
func (b *APIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	// TODO(ricl): implement
}

// SubscribeNewTxsEvent ...
func (b *APIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.hmy.TxPool().SubscribeNewTxsEvent(ch)
}

// SubscribeChainEvent ...
func (b *APIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.hmy.BlockChain().SubscribeChainEvent(ch)
}

// SubscribeChainHeadEvent ...
func (b *APIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.hmy.BlockChain().SubscribeChainHeadEvent(ch)
}

// SubscribeChainSideEvent ...
func (b *APIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.hmy.BlockChain().SubscribeChainSideEvent(ch)
}

// SubscribeRemovedLogsEvent ...
func (b *APIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.hmy.BlockChain().SubscribeRemovedLogsEvent(ch)
}

// SubscribeLogsEvent ...
func (b *APIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.hmy.BlockChain().SubscribeLogsEvent(ch)
}

// GetPoolTransactions ...
func (b *APIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.hmy.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

// GetBalance ...
func (b *APIBackend) GetBalance(address common.Address) (*hexutil.Big, error) {
	balance, err := b.hmy.nodeAPI.GetBalanceOfAddress(address)
	return (*hexutil.Big)(balance), err
}
