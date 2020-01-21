package hmy

import (
	"context"
	"errors"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
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
	staking "github.com/harmony-one/harmony/staking/types"
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
	receipts := b.hmy.blockchain.GetReceiptsByHash(blockHash)
	if receipts == nil {
		return nil, errors.New("Missing receipts")
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

// HeaderByHash ...
func (b *APIBackend) HeaderByHash(ctx context.Context, blockHash common.Hash) (*block.Header, error) {
	header := b.hmy.blockchain.GetHeaderByHash(blockHash)
	if header == nil {
		return nil, errors.New("Header is not found")
	}
	return header, nil
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
func (b *APIBackend) GetBalance(address common.Address) (*big.Int, error) {
	return b.hmy.nodeAPI.GetBalanceOfAddress(address)
}

// GetTransactionsHistory returns list of transactions hashes of address.
func (b *APIBackend) GetTransactionsHistory(address, txType, order string) ([]common.Hash, error) {
	hashes, err := b.hmy.nodeAPI.GetTransactionsHistory(address, txType, order)
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

// GetShardID returns shardID of this node
func (b *APIBackend) GetShardID() uint32 {
	return b.hmy.shardID
}

// GetValidators returns validators for a particular epoch.
func (b *APIBackend) GetValidators(epoch *big.Int) (*shard.Committee, error) {
	state, err := b.hmy.BlockChain().ReadShardState(epoch)
	if err != nil {
		return nil, err
	}
	for _, committee := range state.Shards {
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

// IsLeader exposes if node is currently leader
func (b *APIBackend) IsLeader() bool {
	return b.hmy.nodeAPI.IsCurrentlyLeader()
}

// SendStakingTx adds a staking transaction
func (b *APIBackend) SendStakingTx(
	ctx context.Context,
	newStakingTx *staking.StakingTransaction) error {
	b.hmy.nodeAPI.AddPendingStakingTransaction(newStakingTx)
	return nil
}

// GetActiveValidatorAddresses returns the address of active validators for current epoch
func (b *APIBackend) GetActiveValidatorAddresses() []common.Address {
	list, _ := b.hmy.BlockChain().ReadActiveValidatorList()
	return list
}

// GetAllValidatorAddresses returns the up to date validator candidates for next epoch
func (b *APIBackend) GetAllValidatorAddresses() []common.Address {
	return b.hmy.BlockChain().ValidatorCandidates()
}

// GetValidatorInformation returns the information of validator
func (b *APIBackend) GetValidatorInformation(addr common.Address) *staking.Validator {
	val, _ := b.hmy.BlockChain().ReadValidatorInformation(addr)
	return &val.Validator
}

var (
	two = big.NewInt(2)
)

// GetMedianRawStakeSnapshot  ..
func (b *APIBackend) GetMedianRawStakeSnapshot() *big.Int {
	candidates := b.hmy.BlockChain().ValidatorCandidates()
	if len(candidates) == 0 {
		return big.NewInt(0)
	}
	stakes := []*big.Int{}
	for i := range candidates {
		validator, _ := b.hmy.BlockChain().ReadValidatorInformation(candidates[i])
		stake := big.NewInt(0)
		validator.GetAddress()
		for i := range validator.Delegations {
			stake.Add(stake, validator.Delegations[i].Amount)
		}
		stakes = append(stakes, stake)
	}
	sort.SliceStable(
		stakes,
		func(i, j int) bool { return stakes[i].Cmp(stakes[j]) == -1 },
	)

	if l := len(stakes); l > 320 {
		stakes = stakes[:320]
	}

	const isEven = 0
	switch l := len(stakes); l % 2 {
	case isEven:
		left := stakes[(l/2)-1]
		right := stakes[l/2]
		return new(big.Int).Div(new(big.Int).Add(left, right), two)
	default:
		return stakes[l/2]
	}
}

// GetValidatorStats returns the stats of validator
func (b *APIBackend) GetValidatorStats(addr common.Address) *staking.ValidatorStats {
	val, _ := b.hmy.BlockChain().ReadValidatorStats(addr)
	return val
}

// GetDelegationsByValidator returns all delegation information of a validator
func (b *APIBackend) GetDelegationsByValidator(validator common.Address) []*staking.Delegation {
	wrapper, err := b.hmy.BlockChain().ReadValidatorInformation(validator)
	if err != nil || wrapper == nil {
		return nil
	}
	delegations := []*staking.Delegation{}
	for i := range wrapper.Delegations {
		delegations = append(delegations, &wrapper.Delegations[i])
	}
	return delegations
}

// GetDelegationsByDelegator returns all delegation information of a delegator
func (b *APIBackend) GetDelegationsByDelegator(delegator common.Address) ([]common.Address, []*staking.Delegation) {
	addresses := []common.Address{}
	delegations := []*staking.Delegation{}
	delegationIndexes, err := b.hmy.BlockChain().ReadDelegationsByDelegator(delegator)
	if err != nil {
		return nil, nil
	}

	for i := range delegationIndexes {
		wrapper, err := b.hmy.BlockChain().ReadValidatorInformation(delegationIndexes[i].ValidatorAddress)
		if err != nil || wrapper == nil {
			return nil, nil
		}

		if uint64(len(wrapper.Delegations)) > delegationIndexes[i].Index {
			delegations = append(delegations, &wrapper.Delegations[delegationIndexes[i].Index])
		} else {
			delegations = append(delegations, nil)
		}
		addresses = append(addresses, delegationIndexes[i].ValidatorAddress)
	}
	return addresses, delegations
}

// GetValidatorSelfDelegation returns the amount of staking after applying all delegated stakes
func (b *APIBackend) GetValidatorSelfDelegation(addr common.Address) *big.Int {
	wrapper, err := b.hmy.BlockChain().ReadValidatorInformation(addr)
	if err != nil || wrapper == nil {
		return nil
	}
	if len(wrapper.Delegations) == 0 {
		return nil
	}
	return wrapper.Delegations[0].Amount
}

// GetShardState ...
func (b *APIBackend) GetShardState() (*shard.State, error) {
	return b.hmy.BlockChain().ReadShardState(b.hmy.BlockChain().CurrentHeader().Epoch())
}

// GetCurrentStakingErrorSink ..
func (b *APIBackend) GetCurrentStakingErrorSink() []staking.RPCTransactionError {
	return b.hmy.nodeAPI.ErroredStakingTransactionSink()
}

// GetCurrentTransactionErrorSink ..
func (b *APIBackend) GetCurrentTransactionErrorSink() []types.RPCTransactionError {
	return b.hmy.nodeAPI.ErroredTransactionSink()
}

// IsBeaconChainExplorerNode ..
func (b *APIBackend) IsBeaconChainExplorerNode() bool {
	return b.hmy.nodeAPI.IsBeaconChainExplorerNode()
}

// GetPendingCXReceipts ..
func (b *APIBackend) GetPendingCXReceipts() []*types.CXReceiptsProof {
	return b.hmy.nodeAPI.PendingCXReceipts()
}
