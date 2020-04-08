package apiv1

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	commonRPC "github.com/harmony-one/harmony/internal/hmyapi/common"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/harmony-one/harmony/staking/network"
	staking "github.com/harmony-one/harmony/staking/types"
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
// implementations:
//   * hmy/api_backend.go
type Backend interface {
	NetVersion() uint64
	ProtocolVersion() int
	ChainDb() ethdb.Database
	EventMux() *event.TypeMux
	RPCGasCap() *big.Int // global gas cap for hmy_call over rpc: DoS protection
	HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*block.Header, error)
	BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error)
	StateAndHeaderByNumber(
		ctx context.Context, blockNr rpc.BlockNumber,
	) (*state.DB, *block.Header, error)
	GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error)
	GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error)

	GetEVM(ctx context.Context, msg core.Message, state *state.DB, header *block.Header) (*vm.EVM, func() error, error)
	SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
	SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription
	// TxPool API
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	// GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error)
	GetPoolTransactions() (types.PoolTransactions, error)
	GetPoolTransaction(txHash common.Hash) types.PoolTransaction
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	// Stats() (pending int, queued int)
	// TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions)
	SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription
	ChainConfig() *params.ChainConfig
	CurrentBlock() *types.Block
	// Get balance
	GetBalance(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*big.Int, error)
	// Get validators for a particular epoch
	GetValidators(epoch *big.Int) (*shard.Committee, error)
	GetShardID() uint32
	// Get transactions history for an address
	GetTransactionsHistory(address, txType, order string) ([]common.Hash, error)
	// Get staking transactions history for an address
	GetStakingTransactionsHistory(address, txType, order string) ([]common.Hash, error)
	// retrieve the blockHash using txID and add blockHash to CxPool for resending
	ResendCx(ctx context.Context, txID common.Hash) (uint64, bool)
	IsLeader() bool
	SendStakingTx(ctx context.Context, newStakingTx *staking.StakingTransaction) error
	GetElectedValidatorAddresses() []common.Address
	GetAllValidatorAddresses() []common.Address
	GetValidatorInformation(addr common.Address, block *types.Block) (*staking.ValidatorRPCEnchanced, error)
	GetDelegationsByValidator(validator common.Address) []*staking.Delegation
	GetDelegationsByDelegator(delegator common.Address) ([]common.Address, []*staking.Delegation)
	GetValidatorSelfDelegation(addr common.Address) *big.Int
	GetShardState() (*shard.State, error)
	GetCurrentStakingErrorSink() []staking.RPCTransactionError
	GetCurrentTransactionErrorSink() []types.RPCTransactionError
	GetMedianRawStakeSnapshot() (*committee.CompletedEPoSRound, error)
	GetPendingCXReceipts() []*types.CXReceiptsProof
	GetCurrentUtilityMetrics() (*network.UtilityMetric, error)
	GetSuperCommittees() (*quorum.Transition, error)
	GetTotalStakingSnapshot() *big.Int
	GetCurrentBadBlocks() []core.BadBlock
	GetLastCrossLinks() ([]*types.CrossLink, error)
	GetLatestChainHeaders() *block.HeaderPair
	GetNodeMetadata() commonRPC.NodeMetadata
}
