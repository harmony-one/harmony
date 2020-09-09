package hmy

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	commonRPC "github.com/harmony-one/harmony/rpc/common"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/harmony-one/harmony/staking/availability"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

var (
	zero = numeric.ZeroDec()
)

func (hmy *Harmony) readAndUpdateRawStakes(
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
			snapshot, err := hmy.BlockChain.ReadValidatorSnapshotAtEpoch(epoch, slotAddr)
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
			Addr:      slotAddr,
			Key:       slotKey,
			RawStake:  spread,
			EPoSStake: spread,
		})
	}
	return rawStakes
}

func (hmy *Harmony) getSuperCommittees() (*quorum.Transition, error) {
	nowE := hmy.BlockChain.CurrentHeader().Epoch()
	thenE := new(big.Int).Sub(nowE, common.Big1)

	var (
		nowCommittee, prevCommittee *shard.State
		err                         error
	)
	nowCommittee, err = hmy.BlockChain.ReadShardState(nowE)
	if err != nil {
		return nil, err
	}
	prevCommittee, err = hmy.BlockChain.ReadShardState(thenE)
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
		if hmy.BlockChain.Config().IsStaking(prevCommittee.Epoch) {
			if _, err := decider.SetVoters(&comm, prevCommittee.Epoch); err != nil {
				return nil, err
			}
		}
		rawStakes = hmy.readAndUpdateRawStakes(thenE, decider, comm, rawStakes, validatorSpreads)
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
				hmy.BlockChain.Config().StakingEpoch,
				hmy.BlockChain.CurrentHeader().Epoch(),
			)
		}
		rawStakes = hmy.readAndUpdateRawStakes(nowE, decider, comm, rawStakes, validatorSpreads)
		now.Deciders[fmt.Sprintf("shard-%d", comm.ShardID)] = decider
	}
	now.MedianStake = effective.Median(rawStakes)

	return &quorum.Transition{Previous: then, Current: now}, nil
}

// IsStakingEpoch ...
func (hmy *Harmony) IsStakingEpoch(epoch *big.Int) bool {
	return hmy.BlockChain.Config().IsStaking(epoch)
}

// IsPreStakingEpoch ...
func (hmy *Harmony) IsPreStakingEpoch(epoch *big.Int) bool {
	return hmy.BlockChain.Config().IsPreStaking(epoch)
}

// SendStakingTx adds a staking transaction
func (hmy *Harmony) SendStakingTx(ctx context.Context, signedStakingTx *staking.StakingTransaction) error {
	stx, _, _, _ := rawdb.ReadStakingTransaction(hmy.chainDb, signedStakingTx.Hash())
	if stx == nil {
		return hmy.NodeAPI.AddPendingStakingTransaction(signedStakingTx)
	}
	return ErrFinalizedTransaction
}

// GetStakingTransactionsHistory returns list of staking transactions hashes of address.
func (hmy *Harmony) GetStakingTransactionsHistory(address, txType, order string) ([]common.Hash, error) {
	return hmy.NodeAPI.GetStakingTransactionsHistory(address, txType, order)
}

// GetStakingTransactionsCount returns the number of staking transactions of address.
func (hmy *Harmony) GetStakingTransactionsCount(address, txType string) (uint64, error) {
	return hmy.NodeAPI.GetStakingTransactionsCount(address, txType)
}

// GetSuperCommittees ..
func (hmy *Harmony) GetSuperCommittees() (*quorum.Transition, error) {
	nowE := hmy.BlockChain.CurrentHeader().Epoch()
	key := fmt.Sprintf("sc-%s", nowE.String())

	res, err := hmy.SingleFlightRequest(
		key, func() (interface{}, error) {
			thenE := new(big.Int).Sub(nowE, common.Big1)
			thenKey := fmt.Sprintf("sc-%s", thenE.String())
			hmy.group.Forget(thenKey)
			return hmy.getSuperCommittees()
		})
	if err != nil {
		return nil, err
	}
	return res.(*quorum.Transition), err
}

// GetValidators returns validators for a particular epoch.
func (hmy *Harmony) GetValidators(epoch *big.Int) (*shard.Committee, error) {
	state, err := hmy.BlockChain.ReadShardState(epoch)
	if err != nil {
		return nil, err
	}
	for _, cmt := range state.Shards {
		if cmt.ShardID == hmy.ShardID {
			return &cmt, nil
		}
	}
	return nil, nil
}

// GetValidatorSelfDelegation returns the amount of staking after applying all delegated stakes
func (hmy *Harmony) GetValidatorSelfDelegation(addr common.Address) *big.Int {
	wrapper, err := hmy.BlockChain.ReadValidatorInformation(addr)
	if err != nil || wrapper == nil {
		return nil
	}
	if len(wrapper.Delegations) == 0 {
		return nil
	}
	return wrapper.Delegations[0].Amount
}

// GetElectedValidatorAddresses returns the address of elected validators for current epoch
func (hmy *Harmony) GetElectedValidatorAddresses() []common.Address {
	list, _ := hmy.BlockChain.ReadShardState(hmy.BlockChain.CurrentBlock().Epoch())
	return list.StakedValidators().Addrs
}

// GetAllValidatorAddresses returns the up to date validator candidates for next epoch
func (hmy *Harmony) GetAllValidatorAddresses() []common.Address {
	return hmy.BlockChain.ValidatorCandidates()
}

var (
	epochBlocksMap = map[common.Address]map[uint64]staking.EpochSigningEntry{}
)

func (hmy *Harmony) getEpochSigning(epoch *big.Int, addr common.Address) (staking.EpochSigningEntry, error) {
	entry := staking.EpochSigningEntry{}
	if validatorMap, ok := epochBlocksMap[addr]; ok {
		if val, ok := validatorMap[epoch.Uint64()]; ok {
			return val, nil
		}
	}

	snapshot, err := hmy.BlockChain.ReadValidatorSnapshotAtEpoch(epoch, addr)
	if err != nil {
		return entry, err
	}

	// the signing information is for the previous epoch
	prevEpoch := big.NewInt(0).Sub(epoch, common.Big1)
	entry.Epoch = prevEpoch
	entry.Blocks = snapshot.Validator.Counters

	// subtract previous epoch counters if exists
	prevEpochSnap, err := hmy.BlockChain.ReadValidatorSnapshotAtEpoch(prevEpoch, addr)
	if err == nil {
		entry.Blocks.NumBlocksSigned.Sub(
			entry.Blocks.NumBlocksSigned,
			prevEpochSnap.Validator.Counters.NumBlocksSigned,
		)
		entry.Blocks.NumBlocksToSign.Sub(
			entry.Blocks.NumBlocksToSign,
			prevEpochSnap.Validator.Counters.NumBlocksToSign,
		)
	}

	// update map when adding new entry, also remove an entry beyond last 30
	if _, ok := epochBlocksMap[addr]; !ok {
		epochBlocksMap[addr] = map[uint64]staking.EpochSigningEntry{}
	}
	epochBlocksMap[addr][epoch.Uint64()] = entry
	epochMinus30 := big.NewInt(0).Sub(epoch, big.NewInt(staking.SigningHistoryLength))
	delete(epochBlocksMap[addr], epochMinus30.Uint64())

	return entry, nil
}

// GetValidatorInformation returns the information of validator
func (hmy *Harmony) GetValidatorInformation(
	addr common.Address, block *types.Block,
) (*staking.ValidatorRPCEnhanced, error) {
	bc := hmy.BlockChain
	wrapper, err := bc.ReadValidatorInformationAt(addr, block.Root())
	if err != nil {
		s, _ := internal_common.AddressToBech32(addr)
		return nil, errors.Wrapf(err, "not found address in current state %s", s)
	}

	now := block.Epoch()
	// At the last block of epoch, block epoch is e while val.LastEpochInCommittee
	// is already updated to e+1. So need the >= check rather than ==
	inCommittee := wrapper.LastEpochInCommittee.Cmp(now) >= 0
	defaultReply := &staking.ValidatorRPCEnhanced{
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
		ActiveStatus:     wrapper.Validator.Status.String(),
		Lifetime: &staking.AccumulatedOverLifetime{
			BlockReward: wrapper.BlockReward,
			Signing:     wrapper.Counters,
			APR:         zero,
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
		hmy.BeaconChain.CurrentBlock().Header().Number().Int64(),
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

	latestAPR := numeric.ZeroDec()
	l := len(stats.APRs)
	if l > 0 {
		latestAPR = stats.APRs[l-1].Value
	}
	defaultReply.Lifetime.APR = latestAPR
	defaultReply.Lifetime.EpochAPRs = stats.APRs

	// average apr cache keys
	// key := fmt.Sprintf("apr-%s-%d", addr.Hex(), now.Uint64())
	// prevKey := fmt.Sprintf("apr-%s-%d", addr.Hex(), now.Uint64()-1)

	// delete entry for previous epoch
	// b.apiCache.Forget(prevKey)

	// calculate last APRHistoryLength epochs for averaging APR
	// epochFrom := bc.Config().StakingEpoch
	// nowMinus := big.NewInt(0).Sub(now, big.NewInt(staking.APRHistoryLength))
	// if nowMinus.Cmp(epochFrom) > 0 {
	// 	epochFrom = nowMinus
	// }

	// if len(stats.APRs) > 0 && stats.APRs[0].Epoch.Cmp(epochFrom) > 0 {
	// 	epochFrom = stats.APRs[0].Epoch
	// }

	// epochToAPRs := map[int64]numeric.Dec{}
	// for i := 0; i < len(stats.APRs); i++ {
	// 	entry := stats.APRs[i]
	// 	epochToAPRs[entry.Epoch.Int64()] = entry.Value
	// }

	// at this point, validator is active and has apr's for the recent 100 epochs
	// compute average apr over history
	// if avgAPR, err := b.SingleFlightRequest(
	// 	key, func() (interface{}, error) {
	// 		total := numeric.ZeroDec()
	// 		count := 0
	// 		for i := epochFrom.Int64(); i < now.Int64(); i++ {
	// 			if apr, ok := epochToAPRs[i]; ok {
	// 				total = total.Add(apr)
	// 			}
	// 			count++
	// 		}
	// 		if count == 0 {
	// 			return nil, errors.New("no apr snapshots available")
	// 		}
	// 		return total.QuoInt64(int64(count)), nil
	// 	},
	// ); err != nil {
	// 	// could not compute average apr from snapshot
	// 	// assign the latest apr available from stats
	// 	defaultReply.Lifetime.APR = numeric.ZeroDec()
	// } else {
	// 	defaultReply.Lifetime.APR = avgAPR.(numeric.Dec)
	// }

	epochBlocks := []staking.EpochSigningEntry{}
	epochFrom := bc.Config().StakingEpoch
	nowMinus := big.NewInt(0).Sub(now, big.NewInt(staking.SigningHistoryLength))
	if nowMinus.Cmp(epochFrom) > 0 {
		epochFrom = nowMinus
	}
	for i := now.Int64(); i > epochFrom.Int64(); i-- {
		epoch := big.NewInt(i)
		entry, err := hmy.getEpochSigning(epoch, addr)
		if err != nil {
			break
		}
		epochBlocks = append(epochBlocks, entry)
	}
	defaultReply.Lifetime.EpochBlocks = epochBlocks

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
func (hmy *Harmony) GetMedianRawStakeSnapshot() (
	*committee.CompletedEPoSRound, error,
) {
	blockNum := hmy.CurrentBlock().NumberU64()
	key := fmt.Sprintf("median-%d", blockNum)

	// delete cache for previous block
	prevKey := fmt.Sprintf("median-%d", blockNum-1)
	hmy.group.Forget(prevKey)

	res, err := hmy.SingleFlightRequest(
		key,
		func() (interface{}, error) {
			// Compute for next epoch
			epoch := big.NewInt(0).Add(hmy.CurrentBlock().Epoch(), big.NewInt(1))
			return committee.NewEPoSRound(epoch, hmy.BlockChain)
		},
	)
	if err != nil {
		return nil, err
	}
	return res.(*committee.CompletedEPoSRound), nil
}

// GetDelegationsByValidator returns all delegation information of a validator
func (hmy *Harmony) GetDelegationsByValidator(validator common.Address) []*staking.Delegation {
	wrapper, err := hmy.BlockChain.ReadValidatorInformation(validator)
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
func (hmy *Harmony) GetDelegationsByDelegator(
	delegator common.Address,
) ([]common.Address, []*staking.Delegation) {
	block := hmy.BlockChain.CurrentBlock()
	return hmy.GetDelegationsByDelegatorByBlock(delegator, block)
}

// GetDelegationsByDelegatorByBlock returns all delegation information of a delegator
func (hmy *Harmony) GetDelegationsByDelegatorByBlock(
	delegator common.Address, block *types.Block,
) ([]common.Address, []*staking.Delegation) {
	addresses := []common.Address{}
	delegations := []*staking.Delegation{}
	delegationIndexes, err := hmy.BlockChain.
		ReadDelegationsByDelegatorAt(delegator, block.Number())
	if err != nil {
		return nil, nil
	}

	for i := range delegationIndexes {
		wrapper, err := hmy.BlockChain.ReadValidatorInformationAt(
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

// GetAllUndelegatedDelegators returns all of the delegators that undelegated for the given epoch
func (hmy *Harmony) GetAllUndelegatedDelegators(
	ctx context.Context, epoch *big.Int,
) (map[common.Address]interface{}, error) {
	if !hmy.IsPreStakingEpoch(epoch) {
		return nil, fmt.Errorf("not pre-staking epoch or later")
	}

	currBlockNum := core.EpochFirstBlock(epoch)
	if hmy.IsPreStakingEpoch(new(big.Int).Sub(epoch, big.NewInt(1))) {
		// undelegations for the last block of an epoch do not get applied until the end of the following epoch.
		currBlockNum = new(big.Int).Sub(currBlockNum, big.NewInt(1))
	}

	delegators := map[common.Address]interface{}{}
	currBlock, err := hmy.BlockByNumber(ctx, rpc.BlockNumber(currBlockNum.Uint64()))
	if err != nil || currBlock == nil {
		// Block not found, so return all delegators found so far
		return delegators, nil
	}
	for currBlock.Epoch().Cmp(epoch) != 1 {
		for _, stx := range currBlock.StakingTransactions() {
			if stx.StakingType() != staking.DirectiveUndelegate {
				continue
			}
			sender, err := stx.SenderAddress()
			if err != nil {
				return nil, err
			}
			if _, ok := delegators[sender]; !ok {
				delegators[sender] = struct{}{}
			}
		}
		currBlock, err = hmy.BlockByNumber(ctx, rpc.BlockNumber(currBlock.NumberU64()+1))
		if err != nil || currBlock == nil {
			// Block not found, so return all delegators found so far
			break
		}
	}
	return delegators, nil
}

type undelegationChanges struct {
	// Changes map of all undelegation amount (in Atto) changes per validator
	Changes map[common.Address]*big.Int
	// Total amount (in Atto) of undelegation changes for all validators
	Total *big.Int
}

// GetUndelegationChange gets all of the undelegation deltas from block initBlockNum to block resultBlockNum
func (hmy *Harmony) GetUndelegationChange(
	ctx context.Context, delegator common.Address,
	initBlockNum rpc.BlockNumber, resultBlockNum rpc.BlockNumber,
) (*undelegationChanges, error) {
	initBlk, err := hmy.BlockByNumber(ctx, initBlockNum)
	if err != nil {
		return nil, err
	}
	resultBlk, err := hmy.BlockByNumber(ctx, resultBlockNum)
	if err != nil {
		return nil, err
	}

	initVals, initDels := hmy.GetDelegationsByDelegatorByBlock(delegator, initBlk)
	if len(initVals) != len(initDels) {
		return nil, fmt.Errorf("validator & delegation slice length miss-match for delegation info")
	}
	resultVals, resultDels := hmy.GetDelegationsByDelegatorByBlock(delegator, resultBlk)
	if len(resultVals) != len(resultDels) {
		return nil, fmt.Errorf("validator & delegation slice length miss-match for delegation info")
	}
	resultDelsByVals := map[common.Address]*staking.Delegation{}
	for i := range resultVals {
		resultDelsByVals[resultVals[i]] = resultDels[i]
	}

	deltas := &undelegationChanges{
		Changes: map[common.Address]*big.Int{},
		Total:   big.NewInt(0),
	}
	for i := range initVals {
		valAddr := initVals[i]
		initDel := initDels[i]
		resultDel, ok := resultDelsByVals[valAddr]
		if !ok {
			deltas.Changes[valAddr] = initDel.Amount
		} else {
			deltas.Changes[valAddr] = new(big.Int).Sub(initDel.Amount, resultDel.Amount)
		}
		deltas.Total = new(big.Int).Add(deltas.Total, deltas.Changes[valAddr])
	}
	return deltas, nil
}

// GetTotalStakingSnapshot ..
func (hmy *Harmony) GetTotalStakingSnapshot() *big.Int {
	if stake := hmy.totalStakeCache.pop(hmy.CurrentBlock().NumberU64()); stake != nil {
		return stake
	}
	currHeight := hmy.CurrentBlock().NumberU64()
	candidates := hmy.BlockChain.ValidatorCandidates()
	if len(candidates) == 0 {
		stake := big.NewInt(0)
		hmy.totalStakeCache.push(currHeight, stake)
		return stake
	}
	stakes := big.NewInt(0)
	for i := range candidates {
		snapshot, _ := hmy.BlockChain.ReadValidatorSnapshot(candidates[i])
		validator, _ := hmy.BlockChain.ReadValidatorInformation(candidates[i])
		if !committee.IsEligibleForEPoSAuction(
			snapshot, validator,
		) {
			continue
		}
		for i := range validator.Delegations {
			stakes.Add(stakes, validator.Delegations[i].Amount)
		}
	}
	hmy.totalStakeCache.push(currHeight, stakes)
	return stakes
}

// GetCurrentStakingErrorSink ..
func (hmy *Harmony) GetCurrentStakingErrorSink() types.TransactionErrorReports {
	return hmy.NodeAPI.ReportStakingErrorSink()
}

// totalStakeCache ..
type totalStakeCache struct {
	sync.Mutex
	cachedBlockHeight uint64
	stake             *big.Int
	// duration is in blocks
	duration uint64
}

// newTotalStakeCache ..
func newTotalStakeCache(duration uint64) *totalStakeCache {
	return &totalStakeCache{
		cachedBlockHeight: 0,
		stake:             nil,
		duration:          duration,
	}
}

func (c *totalStakeCache) push(currBlockHeight uint64, stake *big.Int) {
	c.Lock()
	defer c.Unlock()
	c.cachedBlockHeight = currBlockHeight
	c.stake = stake
}

func (c *totalStakeCache) pop(currBlockHeight uint64) *big.Int {
	if currBlockHeight > c.cachedBlockHeight+c.duration {
		return nil
	}
	return c.stake
}
