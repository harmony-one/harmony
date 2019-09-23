package hmy

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
)

// APIBackend An implementation of internal/hmyapi/Backend. Full client.
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
func (b *APIBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.DB, *block.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		return nil, nil, errors.New("not implemented")
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := b.hmy.blockchain.StateAt(header.Root())
	return stateDb, header, err
}

// HeaderByNumber ...
func (b *APIBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*block.Header, error) {
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
// TODO: this is not implemented or verified yet for harmony.
func (b *APIBackend) EventMux() *event.TypeMux { return b.hmy.eventMux }

// BloomStatus ...
// TODO: this is not implemented or verified yet for harmony.
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
func (b *APIBackend) HeaderByHash(ctx context.Context, blockHash common.Hash) (*block.Header, error) {
	// TODO(ricl): implement
	return nil, nil
}

// ServiceFilter ...
func (b *APIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	// TODO(ricl): implement
}

// SubscribeNewTxsEvent subcribes new tx event.
// TODO: this is not implemented or verified yet for harmony.
func (b *APIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.hmy.TxPool().SubscribeNewTxsEvent(ch)
}

// SubscribeChainEvent subcribes chain event.
// TODO: this is not implemented or verified yet for harmony.
func (b *APIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.hmy.BlockChain().SubscribeChainEvent(ch)
}

// SubscribeChainHeadEvent subcribes chain head event.
// TODO: this is not implemented or verified yet for harmony.
func (b *APIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.hmy.BlockChain().SubscribeChainHeadEvent(ch)
}

// SubscribeChainSideEvent subcribes chain side event.
// TODO: this is not implemented or verified yet for harmony.
func (b *APIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.hmy.BlockChain().SubscribeChainSideEvent(ch)
}

// SubscribeRemovedLogsEvent subcribes removed logs event.
// TODO: this is not implemented or verified yet for harmony.
func (b *APIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.hmy.BlockChain().SubscribeRemovedLogsEvent(ch)
}

// SubscribeLogsEvent subcribes log event.
// TODO: this is not implemented or verified yet for harmony.
func (b *APIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.hmy.BlockChain().SubscribeLogsEvent(ch)
}

// GetPoolTransactions returns pool transactions.
// TODO: this is not implemented or verified yet for harmony.
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

// GetBalance returns balance of an given address.
func (b *APIBackend) GetBalance(address common.Address) (*hexutil.Big, error) {
	balance, err := b.hmy.nodeAPI.GetBalanceOfAddress(address)
	return (*hexutil.Big)(balance), err
}

// GetTransactionsHistory returns list of transactions hashes of address.
func (b *APIBackend) GetTransactionsHistory(address string) ([]common.Hash, error) {
	hashes, err := b.hmy.nodeAPI.GetTransactionsHistory(address)
	return hashes, err
}

// NetVersion returns net version
func (b *APIBackend) NetVersion() uint64 {
	return b.hmy.NetVersion()
}

// GetEVM returns a new EVM entity
func (b *APIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.DB, header *block.Header) (*vm.EVM, func() error, error) {
	// TODO(ricl): The code is borrowed from [go-ethereum](https://github.com/ethereum/go-ethereum/blob/40cdcf8c47ff094775aca08fd5d94051f9cf1dbb/les/api_backend.go#L114)
	// [question](https://ethereum.stackexchange.com/q/72977/54923)
	// Might need to reconsider the SetBalance behavior
	state.SetBalance(msg.From(), math.MaxBig256)
	vmError := func() error { return nil }

	context := core.NewEVMContext(msg, header, b.hmy.BlockChain(), nil)
	return vm.NewEVM(context, state, b.hmy.blockchain.Config(), *b.hmy.blockchain.GetVMConfig()), vmError, nil
}

// RPCGasCap returns the gas cap of rpc
func (b *APIBackend) RPCGasCap() *big.Int {
	return b.hmy.RPCGasCap // TODO(ricl): should be hmy.config.RPCGasCap
}

// GetShardID returns the gas cap of rpc
func (b *APIBackend) GetShardID() uint32 {
	return b.hmy.shardID
}

// GetCommittee returns committee for a particular epoch.
func (b *APIBackend) GetCommittee(epoch *big.Int) (*shard.Committee, error) {
	state, err := b.hmy.BlockChain().ReadShardState(epoch)
	if err != nil {
		return nil, err
	}
	for _, committee := range state {
		if committee.ShardID == b.GetShardID() {
			return &committee, nil
		}
	}
	return nil, nil
}

// ResendCx retrieve blockHash from txID and add blockHash to CxPool for resending
func (b *APIBackend) ResendCx(ctx context.Context, txID common.Hash) (uint64, bool) {
	blockHash, blockNum, index := b.hmy.BlockChain().ReadTxLookupEntry(txID)
	blk := b.hmy.BlockChain().GetBlockByHash(blockHash)
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
	entry := core.CxEntry{blockHash, tx.ToShardID()}
	success := b.hmy.CxPool().Add(entry)
	return blockNum, success
}
