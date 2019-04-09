package hmyapi

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
type Backend interface {
	// General Ethereum API
	Downloader() *downloader.Downloader
	ProtocolVersion() int
	SuggestPrice(ctx context.Context) (*big.Int, error)
	ChainDb() ethdb.Database
	EventMux() *event.TypeMux
	AccountManager() *accounts.Manager
	ExtRPCEnabled() bool

	// BlockChain API
	SetHead(number uint64)
	HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error)
	BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error)
	StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error)
	GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error)
	GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error)
	GetTd(blockHash common.Hash) *big.Int
	GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header) (*vm.EVM, func() error, error)
	SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
	SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription

	// TxPool API
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	GetPoolTransactions() (types.Transactions, error)
	GetPoolTransaction(txHash common.Hash) *types.Transaction
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	Stats() (pending int, queued int)
	TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions)
	SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription

	ChainConfig() *params.ChainConfig
	CurrentBlock() *types.Block
}

func GetAPIs(apiBackend Backend) []rpc.API {
	// nonceLock := new(AddrLocker)
	return []rpc.API{
		// {
		// 	Namespace: "eth",
		// 	Version:   "1.0",
		// 	Service:   NewPublicEthereumAPI(apiBackend),
		// 	Public:    true,
		// }, {
		// 	Namespace: "eth",
		// 	Version:   "1.0",
		// 	Service:   NewPublicBlockChainAPI(apiBackend),
		// 	Public:    true,
		// }, {
		// 	Namespace: "eth",
		// 	Version:   "1.0",
		// 	Service:   NewPublicTransactionPoolAPI(apiBackend, nonceLock),
		// 	Public:    true,
		// }, {
		// 	Namespace: "txpool",
		// 	Version:   "1.0",
		// 	Service:   NewPublicTxPoolAPI(apiBackend),
		// 	Public:    true,
		// }, {
		// 	Namespace: "debug",
		// 	Version:   "1.0",
		// 	Service:   NewPublicDebugAPI(apiBackend),
		// 	Public:    true,
		// }, {
		// 	Namespace: "debug",
		// 	Version:   "1.0",
		// 	Service:   NewPrivateDebugAPI(apiBackend),
		// }, {
		// 	Namespace: "eth",
		// 	Version:   "1.0",
		// 	Service:   NewPublicAccountAPI(apiBackend.AccountManager()),
		// 	Public:    true,
		// }, {
		// 	Namespace: "personal",
		// 	Version:   "1.0",
		// 	Service:   NewPrivateAccountAPI(apiBackend, nonceLock),
		// 	Public:    false,
		// },
	}
}
