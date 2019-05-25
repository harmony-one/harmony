package hmy

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"
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
	blockchain     *core.BlockChain
	txPool         *core.TxPool
	accountManager *accounts.Manager
	eventMux       *event.TypeMux

	hmy *Harmony

	bloomIndexer *core.ChainIndexer // Bloom indexer operating during block imports
}

// NewBackend ...
func NewBackend(blockchain *core.BlockChain, txPool *core.TxPool, accountManager *accounts.Manager, eventMux *event.TypeMux) *APIBackend {
	return &APIBackend{
		blockchain:     blockchain,
		txPool:         txPool,
		accountManager: accountManager,
		eventMux:       eventMux,
	}
}

// ChainDb ...
func (b *APIBackend) ChainDb() ethdb.Database {
	return b.hmy.chainDb
}

// GetBlock ...
func (b *APIBackend) GetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.blockchain.GetBlockByHash(hash), nil
}

// GetPoolTransaction ...
func (b *APIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.txPool.Get(hash)
}

// BlockByNumber ...
func (b *APIBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		return nil, errors.New("not implemented")
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.blockchain.CurrentBlock(), nil
	}
	return b.blockchain.GetBlockByNumber(uint64(blockNr)), nil
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
	stateDb, err := b.blockchain.StateAt(header.Root)
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
		return b.blockchain.CurrentBlock().Header(), nil
	}
	return b.blockchain.GetHeaderByNumber(uint64(blockNr)), nil
}

// GetPoolNonce ...
func (b *APIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.txPool.State().GetNonce(addr), nil
}

// SendTx ...
func (b *APIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.txPool.Add(ctx, signedTx)
}

// ChainConfig ...
func (b *APIBackend) ChainConfig() *params.ChainConfig {
	return b.hmy.blockchain.Config()
}

// CurrentBlock ...
func (b *APIBackend) CurrentBlock() *types.Block {
	return types.NewBlockWithHeader(b.blockchain.CurrentHeader())
}

// AccountManager ...
func (b *APIBackend) AccountManager() *accounts.Manager {
	return b.accountManager
}

// GetReceipts ...
func (b *APIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.blockchain.GetReceiptsByHash(hash), nil
}

// EventMux ...
func (b *APIBackend) EventMux() *event.TypeMux { return b.eventMux }

// BloomStatus ...
func (b *APIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.bloomIndexer.Sections()
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
func (b *APIBackend) SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription {
	// TODO(ricl): implement
	return nil
}

// SubscribeChainEvent ...
func (b *APIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	// TODO(ricl): implement
	return nil
}

// SubscribeRemovedLogsEvent ...
func (b *APIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	// TODO(ricl): implement
	return nil
}

// SubscribeLogsEvent ...
func (b *APIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	// TODO(ricl): implement
	return nil
}
