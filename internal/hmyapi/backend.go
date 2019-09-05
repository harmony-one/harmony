package hmyapi

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
// implementations:
//   * hmy/api_backend.go
type Backend interface {
	// NOTE(ricl): this is not in ETH Backend inteface. They put it directly in eth object.
	NetVersion() uint64
	// General Ethereum API
	// Downloader() *downloader.Downloader
	ProtocolVersion() int
	// SuggestPrice(ctx context.Context) (*big.Int, error)
	ChainDb() ethdb.Database
	EventMux() *event.TypeMux
	AccountManager() *accounts.Manager
	// ExtRPCEnabled() bool
	RPCGasCap() *big.Int // global gas cap for hmy_call over rpc: DoS protection

	// BlockChain API
	// SetHead(number uint64)
	HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*block.Header, error)
	BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error)
	StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.DB, *block.Header, error)
	GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error)
	GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error)
	// GetTd(blockHash common.Hash) *big.Int
	GetEVM(ctx context.Context, msg core.Message, state *state.DB, header *block.Header) (*vm.EVM, func() error, error)
	SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
	SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription

	// TxPool API
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	// GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error)
	GetPoolTransactions() (types.Transactions, error)
	GetPoolTransaction(txHash common.Hash) *types.Transaction
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	// Stats() (pending int, queued int)
	// TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions)
	SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription

	ChainConfig() *params.ChainConfig
	CurrentBlock() *types.Block
	// Get balance
	GetBalance(address common.Address) (*hexutil.Big, error)
	GetShardID() uint32
}

// GetAPIs returns all the APIs.
func GetAPIs(b Backend) []rpc.API {
	nonceLock := new(AddrLocker)
	return []rpc.API{
		{
			Namespace: "hmy",
			Version:   "1.0",
			Service:   NewPublicHarmonyAPI(b),
			Public:    true,
		},
		{
			Namespace: "hmy",
			Version:   "1.0",
			Service:   NewPublicBlockChainAPI(b),
			Public:    true,
		}, {
			Namespace: "hmy",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(b, nonceLock),
			Public:    true,
		}, {
			Namespace: "hmy",
			Version:   "1.0",
			Service:   NewPublicAccountAPI(b.AccountManager()),
			Public:    true,
		}, {
			Namespace: "hmy",
			Version:   "1.0",
			Service:   NewDebugAPI(b),
			Public:    true, // FIXME: change to false once IPC implemented
		},
	}
}
