package hmy

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	internal_common "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	commonRPC "github.com/harmony-one/harmony/internal/hmyapi/common"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/harmony-one/harmony/staking/availability"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/harmony-one/harmony/staking/network"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
)

// APIBackend An implementation of internal/hmyapi/Backend. Full client.
type APIBackend struct {
	hmy               *Harmony
	TotalStakingCache struct {
		sync.Mutex
		BlockHeight  int64
		TotalStaking *big.Int
	}
	apiCache singleflight.Group
}

// SingleFlightRequest ...
func (b *APIBackend) SingleFlightRequest(
	key string,
	fn func() (interface{}, error),
) (interface{}, error) {
	res, err, _ := b.apiCache.Do(key, fn)
	return res, err
}

// SingleFlightForgetKey ...
func (b *APIBackend) SingleFlightForgetKey(key string) {
	b.apiCache.Forget(key)
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
func (b *APIBackend) GetPoolTransaction(hash common.Hash) types.PoolTransaction {
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
	b.hmy.nodeAPI.AddPendingTransaction(signedTx)
	return nil
}

// ChainConfig ...
func (b *APIBackend) ChainConfig() *params.ChainConfig {
	return b.hmy.blockchain.Config()
}

// CurrentBlock ...
func (b *APIBackend) CurrentBlock() *types.Block {
	return types.NewBlockWithHeader(b.hmy.blockchain.CurrentHeader())
}

// GetReceipts ...
func (b *APIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.hmy.blockchain.GetReceiptsByHash(hash), nil
}

// EventMux ...
// TODO: this is not implemented or verified yet for harmony.
func (b *APIBackend) EventMux() *event.TypeMux { return b.hmy.eventMux }

const (
	// BloomBitsBlocks is the number of blocks a single bloom bit section vector
	// contains on the server side.
	BloomBitsBlocks uint64 = 4096
)

// BloomStatus ...
// TODO: this is not implemented or verified yet for harmony.
func (b *APIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.hmy.bloomIndexer.Sections()
	return BloomBitsBlocks, sections
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
func (b *APIBackend) GetPoolTransactions() (types.PoolTransactions, error) {
	pending, err := b.hmy.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.PoolTransactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

// GetPoolStats returns the number of pending and queued transactions
func (b *APIBackend) GetPoolStats() (pendingCount, queuedCount int) {
	return b.hmy.txPool.Stats()
}

// GetAccountNonce returns the nonce value of the given address for the given block number
func (b *APIBackend) GetAccountNonce(
	ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (uint64, error) {
	state, _, err := b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return 0, err
	}
	return state.GetNonce(address), state.Error()
}

// GetBalance returns balance of an given address.
func (b *APIBackend) GetBalance(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*big.Int, error) {
	state, _, err := b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	return state.GetBalance(address), state.Error()
}

// GetTransactionsHistory returns list of transactions hashes of address.
func (b *APIBackend) GetTransactionsHistory(address, txType, order string) ([]common.Hash, error) {
	return b.hmy.nodeAPI.GetTransactionsHistory(address, txType, order)
}

// GetStakingTransactionsHistory returns list of staking transactions hashes of address.
func (b *APIBackend) GetStakingTransactionsHistory(address, txType, order string) ([]common.Hash, error) {
	return b.hmy.nodeAPI.GetStakingTransactionsHistory(address, txType, order)
}

// GetTransactionsCount returns the number of regular transactions of address.
func (b *APIBackend) GetTransactionsCount(address, txType string) (uint64, error) {
	return b.hmy.nodeAPI.GetTransactionsCount(address, txType)
}

// GetStakingTransactionsCount returns the number of staking transactions of address.
func (b *APIBackend) GetStakingTransactionsCount(address, txType string) (uint64, error) {
	return b.hmy.nodeAPI.GetStakingTransactionsCount(address, txType)
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
// Note that cross shard txn is only for regular txns, not for staking txns, so the input txn hash
// is expected to be regular txn hash
func (b *APIBackend) ResendCx(ctx context.Context, txID common.Hash) (uint64, bool) {
	blockHash, blockNum, index := b.hmy.BlockChain().ReadTxLookupEntry(txID)
	if blockHash == (common.Hash{}) {
		return 0, false
	}

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

// GetElectedValidatorAddresses returns the address of elected validators for current epoch
func (b *APIBackend) GetElectedValidatorAddresses() []common.Address {
	list, _ := b.hmy.BlockChain().ReadShardState(b.hmy.BlockChain().CurrentBlock().Epoch())
	return list.StakedValidators().Addrs
}

// GetAllValidatorAddresses returns the up to date validator candidates for next epoch
func (b *APIBackend) GetAllValidatorAddresses() []common.Address {
	return b.hmy.BlockChain().ValidatorCandidates()
}

var (
	zero = numeric.ZeroDec()
)

// GetValidatorInformation returns the information of validator
func (b *APIBackend) GetValidatorInformation(
	addr common.Address, block *types.Block,
) (*staking.ValidatorRPCEnchanced, error) {
	bc := b.hmy.BlockChain()
	wrapper, err := bc.ReadValidatorInformationAt(addr, block.Root())
	if err != nil {
		s, _ := internal_common.AddressToBech32(addr)
		return nil, errors.Wrapf(err, "not found address in current state %s", s)
	}

	now := block.Epoch()
	// At the last block of epoch, block epoch is e while val.LastEpochInCommittee
	// is already updated to e+1. So need the >= check rather than ==
	inCommittee := wrapper.LastEpochInCommittee.Cmp(now) >= 0
	defaultReply := &staking.ValidatorRPCEnchanced{
		CurrentlyInCommittee: inCommittee,
		Wrapper:              *wrapper,
		Performance:          nil,
		ComputedMetrics:      nil,
		TotalDelegated:       wrapper.TotalDelegation(),
		EPoSStatus: effective.ValidatorStatus(
			inCommittee, wrapper.Status,
		).String(),
		EPoSWinningStake: nil,
		BootedStatus:     nil,
		Lifetime: &staking.AccumulatedOverLifetime{
			wrapper.BlockReward,
			wrapper.Counters,
			zero,
		},
	}

	snapshot, err := bc.ReadValidatorSnapshotAtEpoch(
		now, addr,
	)

	if err != nil {
		return defaultReply, nil
	}

	computed := availability.ComputeCurrentSigning(
		snapshot.Validator, wrapper,
	)
	beaconChainBlocks := uint64(
		b.hmy.BeaconChain().CurrentBlock().Header().Number().Int64(),
	) % shard.Schedule.BlocksPerEpoch()
	computed.BlocksLeftInEpoch = shard.Schedule.BlocksPerEpoch() - beaconChainBlocks

	if defaultReply.CurrentlyInCommittee {
		defaultReply.Performance = &staking.CurrentEpochPerformance{
			CurrentSigningPercentage: *computed,
		}
	}

	stats, err := bc.ReadValidatorStats(addr)
	if err != nil {
		// when validator has no stats, default boot-status to not booted
		notBooted := effective.NotBooted.String()
		defaultReply.BootedStatus = &notBooted
		return defaultReply, nil
	}

	// average apr cache keys
	key := fmt.Sprintf("apr-%s-%d", addr.Hex(), now.Uint64())
	prevKey := fmt.Sprintf("apr-%s-%d", addr.Hex(), now.Uint64()-1)

	// delete entry for previous epoch
	b.apiCache.Forget(prevKey)

	// calculate last APRHistoryLength epochs for averaging APR
	epochFrom := bc.Config().StakingEpoch
	nowMinus := big.NewInt(0).Sub(now, big.NewInt(staking.APRHistoryLength))
	if nowMinus.Cmp(epochFrom) > 0 {
		epochFrom = nowMinus
	}

	if len(stats.APRs) > 0 && stats.APRs[0].Epoch.Cmp(epochFrom) > 0 {
		epochFrom = stats.APRs[0].Epoch
	}

	epochToAPRs := map[int64]numeric.Dec{}
	for i := 0; i < len(stats.APRs); i++ {
		entry := stats.APRs[i]
		epochToAPRs[entry.Epoch.Int64()] = entry.Value
	}

	// at this point, validator is active and has apr's for the recent 100 epochs
	// compute average apr over history
	if avgAPR, err := b.SingleFlightRequest(
		key, func() (interface{}, error) {
			total := numeric.ZeroDec()
			count := 0
			for i := epochFrom.Int64(); i < now.Int64(); i++ {
				if apr, ok := epochToAPRs[i]; ok {
					total = total.Add(apr)
				}
				count++
			}
			if count == 0 {
				return nil, errors.New("no apr snapshots available")
			}
			return total.QuoInt64(int64(count)), nil
		},
	); err != nil {
		// could not compute average apr from snapshot
		// assign the latest apr available from stats
		defaultReply.Lifetime.APR = numeric.ZeroDec()
	} else {
		defaultReply.Lifetime.APR = avgAPR.(numeric.Dec)
	}

	if defaultReply.CurrentlyInCommittee {
		defaultReply.ComputedMetrics = stats
		defaultReply.EPoSWinningStake = &stats.TotalEffectiveStake
	}

	if !defaultReply.CurrentlyInCommittee {
		reason := stats.BootedStatus.String()
		defaultReply.BootedStatus = &reason
	}

	return defaultReply, nil
}

// GetMedianRawStakeSnapshot ..
func (b *APIBackend) GetMedianRawStakeSnapshot() (
	*committee.CompletedEPoSRound, error,
) {
	blockNr := b.CurrentBlock().NumberU64()
	key := fmt.Sprintf("median-%d", blockNr)

	// delete cache for previous block
	prevKey := fmt.Sprintf("median-%d", blockNr-1)
	b.apiCache.Forget(prevKey)

	res, err := b.SingleFlightRequest(
		key,
		func() (interface{}, error) {
			return committee.NewEPoSRound(b.hmy.BlockChain())
		},
	)
	if err != nil {
		return nil, err
	}
	return res.(*committee.CompletedEPoSRound), nil
}

// GetLatestChainHeaders ..
func (b *APIBackend) GetLatestChainHeaders() *block.HeaderPair {
	return &block.HeaderPair{
		BeaconHeader: b.hmy.BeaconChain().CurrentHeader(),
		ShardHeader:  b.hmy.BlockChain().CurrentHeader(),
	}
}

// GetTotalStakingSnapshot ..
func (b *APIBackend) GetTotalStakingSnapshot() *big.Int {
	b.TotalStakingCache.Lock()
	defer b.TotalStakingCache.Unlock()
	if b.TotalStakingCache.BlockHeight != -1 &&
		b.TotalStakingCache.BlockHeight > int64(rpc.LatestBlockNumber)-20 {
		return b.TotalStakingCache.TotalStaking
	}
	b.TotalStakingCache.BlockHeight = int64(rpc.LatestBlockNumber)
	candidates := b.hmy.BlockChain().ValidatorCandidates()
	if len(candidates) == 0 {
		b.TotalStakingCache.TotalStaking = big.NewInt(0)
		return b.TotalStakingCache.TotalStaking
	}
	stakes := big.NewInt(0)
	for i := range candidates {
		snapshot, _ := b.hmy.BlockChain().ReadValidatorSnapshot(candidates[i])
		validator, _ := b.hmy.BlockChain().ReadValidatorInformation(candidates[i])
		if !committee.IsEligibleForEPoSAuction(
			snapshot, validator,
		) {
			continue
		}
		for i := range validator.Delegations {
			stakes.Add(stakes, validator.Delegations[i].Amount)
		}
	}
	b.TotalStakingCache.TotalStaking = stakes
	return b.TotalStakingCache.TotalStaking
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

// GetDelegationsByDelegatorByBlock returns all delegation information of a delegator
func (b *APIBackend) GetDelegationsByDelegatorByBlock(
	delegator common.Address, block *types.Block,
) ([]common.Address, []*staking.Delegation) {
	addresses := []common.Address{}
	delegations := []*staking.Delegation{}
	delegationIndexes, err := b.hmy.BlockChain().
		ReadDelegationsByDelegatorAt(delegator, block.Number())
	if err != nil {
		return nil, nil
	}

	for i := range delegationIndexes {
		wrapper, err := b.hmy.BlockChain().ReadValidatorInformationAt(
			delegationIndexes[i].ValidatorAddress, block.Root(),
		)
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

// GetDelegationsByDelegator returns all delegation information of a delegator
func (b *APIBackend) GetDelegationsByDelegator(
	delegator common.Address,
) ([]common.Address, []*staking.Delegation) {
	block := b.hmy.BlockChain().CurrentBlock()
	return b.GetDelegationsByDelegatorByBlock(delegator, block)
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
func (b *APIBackend) GetCurrentStakingErrorSink() types.TransactionErrorReports {
	return b.hmy.nodeAPI.ReportStakingErrorSink()
}

// GetCurrentTransactionErrorSink ..
func (b *APIBackend) GetCurrentTransactionErrorSink() types.TransactionErrorReports {
	return b.hmy.nodeAPI.ReportPlainErrorSink()
}

// GetPendingCXReceipts ..
func (b *APIBackend) GetPendingCXReceipts() []*types.CXReceiptsProof {
	return b.hmy.nodeAPI.PendingCXReceipts()
}

// GetCurrentUtilityMetrics ..
func (b *APIBackend) GetCurrentUtilityMetrics() (*network.UtilityMetric, error) {
	return network.NewUtilityMetricSnapshot(b.hmy.BlockChain())
}

func (b *APIBackend) readAndUpdateRawStakes(
	epoch *big.Int,
	decider quorum.Decider,
	comm shard.Committee,
	rawStakes []effective.SlotPurchase,
	validatorSpreads map[common.Address]numeric.Dec,
) []effective.SlotPurchase {
	for i := range comm.Slots {
		slot := comm.Slots[i]
		slotAddr := slot.EcdsaAddress
		slotKey := slot.BLSPublicKey
		spread, ok := validatorSpreads[slotAddr]
		if !ok {
			snapshot, err := b.hmy.BlockChain().ReadValidatorSnapshotAtEpoch(epoch, slotAddr)
			if err != nil {
				continue
			}
			wrapper := snapshot.Validator
			spread = numeric.NewDecFromBigInt(wrapper.TotalDelegation()).
				QuoInt64(int64(len(wrapper.SlotPubKeys)))
			validatorSpreads[slotAddr] = spread
		}

		commonRPC.SetRawStake(decider, slotKey, spread)
		// add entry to array for median calculation
		rawStakes = append(rawStakes, effective.SlotPurchase{
			slotAddr,
			slotKey,
			spread,
			spread,
		})
	}
	return rawStakes
}

func (b *APIBackend) getSuperCommittees() (*quorum.Transition, error) {
	nowE := b.hmy.BlockChain().CurrentHeader().Epoch()
	thenE := new(big.Int).Sub(nowE, common.Big1)

	var (
		nowCommittee, prevCommittee *shard.State
		err                         error
	)
	nowCommittee, err = b.hmy.BlockChain().ReadShardState(nowE)
	if err != nil {
		return nil, err
	}
	prevCommittee, err = b.hmy.BlockChain().ReadShardState(thenE)
	if err != nil {
		return nil, err
	}

	stakedSlotsNow, stakedSlotsThen :=
		shard.ExternalSlotsAvailableForEpoch(nowE),
		shard.ExternalSlotsAvailableForEpoch(thenE)

	then, now :=
		quorum.NewRegistry(stakedSlotsThen),
		quorum.NewRegistry(stakedSlotsNow)

	rawStakes := []effective.SlotPurchase{}
	validatorSpreads := map[common.Address]numeric.Dec{}
	for _, comm := range prevCommittee.Shards {
		decider := quorum.NewDecider(quorum.SuperMajorityStake, comm.ShardID)
		// before staking skip computing
		if b.hmy.BlockChain().Config().IsStaking(prevCommittee.Epoch) {
			if _, err := decider.SetVoters(&comm, prevCommittee.Epoch); err != nil {
				return nil, err
			}
		}
		rawStakes = b.readAndUpdateRawStakes(thenE, decider, comm, rawStakes, validatorSpreads)
		then.Deciders[fmt.Sprintf("shard-%d", comm.ShardID)] = decider
	}
	then.MedianStake = effective.Median(rawStakes)

	rawStakes = []effective.SlotPurchase{}
	validatorSpreads = map[common.Address]numeric.Dec{}
	for _, comm := range nowCommittee.Shards {
		decider := quorum.NewDecider(quorum.SuperMajorityStake, comm.ShardID)
		if _, err := decider.SetVoters(&comm, nowCommittee.Epoch); err != nil {
			return nil, errors.Wrapf(
				err,
				"committee is only available from staking epoch: %v, current epoch: %v",
				b.hmy.BlockChain().Config().StakingEpoch,
				b.hmy.BlockChain().CurrentHeader().Epoch(),
			)
		}
		rawStakes = b.readAndUpdateRawStakes(nowE, decider, comm, rawStakes, validatorSpreads)
		now.Deciders[fmt.Sprintf("shard-%d", comm.ShardID)] = decider
	}
	now.MedianStake = effective.Median(rawStakes)

	return &quorum.Transition{then, now}, nil
}

// GetSuperCommittees ..
func (b *APIBackend) GetSuperCommittees() (*quorum.Transition, error) {
	nowE := b.hmy.BlockChain().CurrentHeader().Epoch()
	key := fmt.Sprintf("sc-%s", nowE.String())

	res, err := b.SingleFlightRequest(
		key, func() (interface{}, error) {
			thenE := new(big.Int).Sub(nowE, common.Big1)
			thenKey := fmt.Sprintf("sc-%s", thenE.String())
			b.apiCache.Forget(thenKey)
			return b.getSuperCommittees()
		})
	if err != nil {
		return nil, err
	}
	return res.(*quorum.Transition), err
}

// GetCurrentBadBlocks ..
func (b *APIBackend) GetCurrentBadBlocks() []core.BadBlock {
	return b.hmy.BlockChain().BadBlocks()
}

// GetLastCrossLinks ..
func (b *APIBackend) GetLastCrossLinks() ([]*types.CrossLink, error) {
	crossLinks := []*types.CrossLink{}
	for i := uint32(1); i < shard.Schedule.InstanceForEpoch(b.CurrentBlock().Epoch()).NumShards(); i++ {
		link, err := b.hmy.BlockChain().ReadShardLastCrossLink(i)
		if err != nil {
			return nil, err
		}
		crossLinks = append(crossLinks, link)
	}

	return crossLinks, nil
}

// GetNodeMetadata ..
func (b *APIBackend) GetNodeMetadata() commonRPC.NodeMetadata {
	cfg := nodeconfig.GetDefaultConfig()
	header := b.CurrentBlock().Header()
	var blockEpoch *uint64

	if header.ShardID() == shard.BeaconChainShardID {
		sched := shard.Schedule.InstanceForEpoch(header.Epoch())
		b := sched.BlocksPerEpoch()
		blockEpoch = &b
	}

	blsKeys := []string{}
	if cfg.ConsensusPubKey != nil {
		for _, key := range cfg.ConsensusPubKey.PublicKey {
			blsKeys = append(blsKeys, key.SerializeToHexStr())
		}
	}
	c := commonRPC.C{}
	c.TotalKnownPeers, c.Connected, c.NotConnected = b.hmy.nodeAPI.PeerConnectivity()

	return commonRPC.NodeMetadata{
		blsKeys,
		nodeconfig.GetVersion(),
		string(cfg.GetNetworkType()),
		*b.ChainConfig(),
		b.IsLeader(),
		b.GetShardID(),
		header.Epoch().Uint64(),
		blockEpoch,
		cfg.Role().String(),
		cfg.DNSZone,
		cfg.GetArchival(),
		b.hmy.nodeAPI.GetNodeBootTime(),
		c,
	}
}
