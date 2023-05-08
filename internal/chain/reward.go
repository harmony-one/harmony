package chain

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/harmony-one/harmony/internal/params"
	lru "github.com/hashicorp/golang-lru"

	"github.com/harmony-one/harmony/numeric"
	types2 "github.com/harmony-one/harmony/staking/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/availability"
	"github.com/harmony-one/harmony/staking/network"
	stakingReward "github.com/harmony-one/harmony/staking/reward"
	"github.com/pkg/errors"
)

// timeout constant
const (
	// AsyncBlockProposalTimeout is the timeout which will abort the async block proposal.
	AsyncBlockProposalTimeout = 9 * time.Second
	// RewardFrequency the number of blocks between each aggregated reward distribution
	RewardFrequency = 64
)

type slotPayable struct {
	shard.Slot
	payout  *big.Int
	bucket  int
	index   int
	shardID uint32
}

type slotMissing struct {
	shard.Slot
	bucket int
	index  int
}

func ballotResultBeaconchain(
	bc engine.ChainReader, header *block.Header,
) (*big.Int, shard.SlotList, shard.SlotList, shard.SlotList, error) {
	parentHeader := bc.GetHeaderByHash(header.ParentHash())
	if parentHeader == nil {
		return nil, nil, nil, nil, errors.Errorf(
			"cannot find parent block header in DB %s",
			header.ParentHash().Hex(),
		)
	}
	parentShardState, err := bc.ReadShardState(parentHeader.Epoch())
	if err != nil {
		return nil, nil, nil, nil, errors.Errorf(
			"cannot read shard state %v", parentHeader.Epoch(),
		)
	}

	members, payable, missing, err :=
		availability.BallotResult(parentHeader, header, parentShardState, shard.BeaconChainShardID)
	return parentHeader.Epoch(), members, payable, missing, err
}

var (
	votingPowerCache, _   = lru.New(16)
	delegateShareCache, _ = lru.New(1024)
)

func lookupVotingPower(
	epoch *big.Int, subComm *shard.Committee,
) (*votepower.Roster, error) {
	// Look up
	key := fmt.Sprintf("%s-%d", epoch.String(), subComm.ShardID)
	if b, ok := votingPowerCache.Get(key); ok {
		return b.(*votepower.Roster), nil
	}

	// If not found, construct
	votingPower, err := votepower.Compute(subComm, epoch)
	if err != nil {
		return nil, err
	}

	// Put in cache
	votingPowerCache.Add(key, votingPower)
	return votingPower, nil
}

// Lookup or compute the shares of stake for all delegators in a validator
func lookupDelegatorShares(
	snapshot *types2.ValidatorSnapshot,
) (map[common.Address]numeric.Dec, error) {
	epoch := snapshot.Epoch
	validatorSnapshot := snapshot.Validator

	// Look up
	key := fmt.Sprintf("%s-%s", epoch.String(), validatorSnapshot.Address.Hex())

	if b, ok := delegateShareCache.Get(key); ok {
		return b.(map[common.Address]numeric.Dec), nil
	}

	// If not found, construct
	result := map[common.Address]numeric.Dec{}

	totalDelegationDec := numeric.NewDecFromBigInt(validatorSnapshot.TotalDelegation())
	if totalDelegationDec.IsZero() {
		utils.Logger().Info().
			RawJSON("validator-snapshot", []byte(validatorSnapshot.String())).
			Msg("zero total delegation during AddReward delegation payout")
		return result, nil
	}

	for i := range validatorSnapshot.Delegations {
		delegation := validatorSnapshot.Delegations[i]
		// NOTE percentage = <this_delegator_amount>/<total_delegation>
		percentage := numeric.NewDecFromBigInt(delegation.Amount).Quo(totalDelegationDec)
		result[delegation.DelegatorAddress] = percentage
	}

	// Put in cache
	delegateShareCache.Add(key, result)
	return result, nil
}

// Handle block rewards during pre-staking era
func accumulateRewardsAndCountSigsBeforeStaking(
	bc engine.ChainReader, state *state.DB,
	header *block.Header, sigsReady chan bool,
) (reward.Reader, error) {
	parentHeader := bc.GetHeaderByHash(header.ParentHash())

	if parentHeader == nil {
		return network.EmptyPayout, errors.Errorf(
			"cannot find parent block header in DB at parent hash %s",
			header.ParentHash().Hex(),
		)
	}
	if parentHeader.Number().Cmp(common.Big0) == 0 {
		// Parent is an epoch block,
		// which is not signed in the usual manner therefore rewards nothing.
		return network.EmptyPayout, nil
	}
	parentShardState, err := bc.ReadShardState(parentHeader.Epoch())
	if err != nil {
		return nil, errors.Wrapf(
			err, "cannot read shard state at epoch %v", parentHeader.Epoch(),
		)
	}

	// Block here until the commit sigs are ready or timeout.
	// sigsReady signal indicates that the commit sigs are already populated in the header object.
	if err := waitForCommitSigs(sigsReady); err != nil {
		return network.EmptyPayout, err
	}

	_, signers, _, err := availability.BallotResult(
		parentHeader, header, parentShardState, header.ShardID(),
	)

	if err != nil {
		return network.EmptyPayout, err
	}

	totalAmount := big.NewInt(0)

	{
		last := big.NewInt(0)
		count := big.NewInt(int64(len(signers)))
		for i, account := range signers {
			cur := big.NewInt(0)
			cur.Mul(stakingReward.PreStakedBlocks, big.NewInt(int64(i+1))).Div(cur, count)
			diff := big.NewInt(0).Sub(cur, last)
			state.AddBalance(account.EcdsaAddress, diff)
			totalAmount.Add(totalAmount, diff)
			last = cur
		}
	}

	if totalAmount.Cmp(stakingReward.PreStakedBlocks) != 0 {
		utils.Logger().Error().
			Int64("block-reward", stakingReward.PreStakedBlocks.Int64()).
			Int64("total-amount-paid-out", totalAmount.Int64()).
			Msg("Total paid out was not equal to block-reward")
		return nil, errors.Wrapf(
			network.ErrPayoutNotEqualBlockReward, "payout "+totalAmount.String(),
		)
	}

	return network.NewPreStakingEraRewarded(totalAmount), nil
}

// getDefaultStakingReward returns the static default reward based on the the block production interval and the chain.
func getDefaultStakingReward(bc engine.ChainReader, epoch *big.Int, blockNum uint64) numeric.Dec {
	defaultReward := stakingReward.StakedBlocks

	// the block reward is adjusted accordingly based on 5s and 3s block time forks
	if bc.Config().ChainID == params.TestnetChainID && bc.Config().FiveSecondsEpoch.Cmp(big.NewInt(16500)) == 0 {
		// Testnet:
		// This is testnet requiring the one-off forking logic
		if blockNum > 634644 {
			defaultReward = stakingReward.FiveSecStakedBlocks
			if blockNum > 636507 {
				defaultReward = stakingReward.StakedBlocks
				if blockNum > 639341 {
					defaultReward = stakingReward.FiveSecStakedBlocks
				}
			}
		}
		if bc.Config().IsTwoSeconds(epoch) {
			defaultReward = stakingReward.TwoSecStakedBlocks
		}
	} else {
		// Mainnet (other nets):
		if bc.Config().IsTwoSeconds(epoch) {
			defaultReward = stakingReward.TwoSecStakedBlocks
		} else if bc.Config().IsFiveSeconds(epoch) {
			defaultReward = stakingReward.FiveSecStakedBlocks
		}
	}

	return defaultReward
}

// AccumulateRewardsAndCountSigs credits the coinbase of the given block with the mining reward and
// also does IncrementValidatorSigningCounts for validators.
// param sigsReady: Signal indicating that commit sigs are already populated in the header object.
func AccumulateRewardsAndCountSigs(
	bc engine.ChainReader, state *state.DB,
	header *block.Header, beaconChain engine.ChainReader, sigsReady chan bool,
) (reward.Reader, error) {
	blockNum := header.Number().Uint64()
	epoch := header.Epoch()
	isBeaconChain := bc.CurrentHeader().ShardID() == shard.BeaconChainShardID

	if blockNum == 0 {
		err := waitForCommitSigs(sigsReady) // wait for commit signatures, or timeout and return err.
		return network.EmptyPayout, err
	}

	// Pre-staking era
	if !bc.Config().IsStaking(epoch) {
		return accumulateRewardsAndCountSigsBeforeStaking(bc, state, header, sigsReady)
	}

	// Rewards are accumulated only in the beaconchain, so just wait for commit sigs and return.
	if !isBeaconChain {
		err := waitForCommitSigs(sigsReady)
		return network.EmptyPayout, err
	}

	defaultReward := getDefaultStakingReward(bc, epoch, blockNum)
	if defaultReward.IsNegative() { // TODO: Figure out whether that's possible.
		return network.EmptyPayout, nil
	}

	// Handle rewards on pre-aggregated rewards era.
	if !bc.Config().IsAggregatedRewardEpoch(header.Epoch()) {
		return distributeRewardBeforeAggregateEpoch(bc, state, header, beaconChain, defaultReward, sigsReady)
	}

	// Aggregated Rewards Era: Rewards are aggregated every 64 blocks.

	// Wait for commit signatures, or timeout and return err.
	if err := waitForCommitSigs(sigsReady); err != nil {
		return network.EmptyPayout, err
	}

	// Only do reward distribution at the 63th block in the modulus.
	if blockNum%RewardFrequency != RewardFrequency-1 {
		return network.EmptyPayout, nil
	}

	return distributeRewardAfterAggregateEpoch(bc, state, header, beaconChain, defaultReward)
}

func waitForCommitSigs(sigsReady chan bool) error {
	select {
	case success := <-sigsReady:
		if !success {
			return errors.New("Failed to get commit sigs")
		}
		utils.Logger().Info().Msg("Commit sigs are ready")
	case <-time.After(AsyncBlockProposalTimeout):
		return errors.New("Timeout waiting for commit sigs for reward calculation")
	}
	return nil
}

func distributeRewardAfterAggregateEpoch(bc engine.ChainReader, state *state.DB, header *block.Header, beaconChain engine.ChainReader,
	defaultReward numeric.Dec) (reward.Reader, error) {
	newRewards, payouts :=
		big.NewInt(0), []reward.Payout{}

	allPayables := []slotPayable{}
	curBlockNum := header.Number().Uint64()

	allCrossLinks := types.CrossLinks{}
	startTime := time.Now()
	// loop through [0...63] position in the modulus index of the 64 blocks
	// Note the current block is at position 63 of the modulus.
	for i := curBlockNum - RewardFrequency + 1; i <= curBlockNum; i++ {
		if i < 0 {
			continue
		}

		var curHeader *block.Header
		if i == curBlockNum {
			// When it's the current block (63th), we should use the provided header since it's not written in db yet.
			curHeader = header
		} else {
			curHeader = bc.GetHeaderByNumber(i)
		}

		// Put shard 0 signatures as a crosslink for easy and consistent processing as other shards' crosslinks
		allCrossLinks = append(allCrossLinks, *types.NewCrossLink(curHeader, bc.GetHeaderByHash(curHeader.ParentHash())))

		// Put the real crosslinks in the list
		if cxLinks := curHeader.CrossLinks(); len(cxLinks) > 0 {
			crossLinks := types.CrossLinks{}
			if err := rlp.DecodeBytes(cxLinks, &crossLinks); err != nil {
				return network.EmptyPayout, err
			}
			allCrossLinks = append(allCrossLinks, crossLinks...)
		}
	}

	for i := range allCrossLinks {
		cxLink := allCrossLinks[i]
		if !bc.Config().IsStaking(cxLink.Epoch()) {
			continue
		}
		utils.Logger().Info().Msg(fmt.Sprintf("allCrossLinks shard %d block %d", cxLink.ShardID(), cxLink.BlockNum()))
		payables, _, err := processOneCrossLink(bc, state, cxLink, defaultReward, i)

		if err != nil {
			return network.EmptyPayout, err
		}

		allPayables = append(allPayables, payables...)
	}

	// Aggregate all the rewards for each validator
	allValidatorPayable := map[common.Address]*big.Int{}
	allAddresses := []common.Address{}
	for _, payThem := range allPayables {
		if _, ok := allValidatorPayable[payThem.EcdsaAddress]; !ok {
			allValidatorPayable[payThem.EcdsaAddress] = big.NewInt(0).SetBytes(payThem.payout.Bytes())
		} else {
			allValidatorPayable[payThem.EcdsaAddress] = big.NewInt(0).Add(allValidatorPayable[payThem.EcdsaAddress], payThem.payout)
		}

		payouts = append(payouts, reward.Payout{
			Addr:        payThem.EcdsaAddress,
			NewlyEarned: payThem.payout,
			EarningKey:  payThem.BLSPublicKey,
		})
	}

	for addr, _ := range allValidatorPayable {
		allAddresses = append(allAddresses, addr)
	}

	// always sort validators by address before rewarding
	sort.SliceStable(allAddresses,
		func(i, j int) bool {
			return bytes.Compare(allAddresses[i][:], allAddresses[j][:]) < 0
		},
	)

	// Finally do the pay
	startTimeLocal := time.Now()
	for _, addr := range allAddresses {
		snapshot, err := bc.ReadValidatorSnapshot(addr)
		if err != nil {
			return network.EmptyPayout, err
		}
		due := allValidatorPayable[addr]
		newRewards.Add(newRewards, due)

		shares, err := lookupDelegatorShares(snapshot)
		if err != nil {
			return network.EmptyPayout, err
		}
		if err := state.AddReward(snapshot.Validator, due, shares); err != nil {
			return network.EmptyPayout, err
		}
	}
	utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTimeLocal).Milliseconds()).Msg("After Chain Reward (AddReward)")
	utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("After Chain Reward")

	return network.NewStakingEraRewardForRound(
		newRewards, payouts,
	), nil
}

func distributeRewardBeforeAggregateEpoch(bc engine.ChainReader, state *state.DB, header *block.Header, beaconChain engine.ChainReader,
	defaultReward numeric.Dec, sigsReady chan bool) (reward.Reader, error) {
	newRewards, payouts :=
		big.NewInt(0), []reward.Payout{}

	allPayables := []slotPayable{}
	if cxLinks := header.CrossLinks(); len(cxLinks) > 0 {

		startTime := time.Now()
		crossLinks := types.CrossLinks{}
		if err := rlp.DecodeBytes(cxLinks, &crossLinks); err != nil {
			return network.EmptyPayout, err
		}
		utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("Decode Cross Links")

		startTime = time.Now()
		for i := range crossLinks {
			cxLink := crossLinks[i]
			payables, _, err := processOneCrossLink(bc, state, cxLink, defaultReward, i)

			if err != nil {
				return network.EmptyPayout, err
			}

			allPayables = append(allPayables, payables...)
		}

		resultsHandle := make([][]slotPayable, len(crossLinks))
		for i := range resultsHandle {
			resultsHandle[i] = []slotPayable{}
		}

		for _, payThem := range allPayables {
			bucket := payThem.bucket
			resultsHandle[bucket] = append(resultsHandle[bucket], payThem)
		}

		// Check if any errors and sort each bucket to enforce order
		for bucket := range resultsHandle {
			sort.SliceStable(resultsHandle[bucket],
				func(i, j int) bool {
					return resultsHandle[bucket][i].index < resultsHandle[bucket][j].index
				},
			)
		}

		// Finally do the pay
		startTimeLocal := time.Now()
		for bucket := range resultsHandle {
			for payThem := range resultsHandle[bucket] {
				payable := resultsHandle[bucket][payThem]
				snapshot, err := bc.ReadValidatorSnapshot(
					payable.EcdsaAddress,
				)
				if err != nil {
					return network.EmptyPayout, err
				}
				due := resultsHandle[bucket][payThem].payout
				newRewards.Add(newRewards, due)

				shares, err := lookupDelegatorShares(snapshot)
				if err != nil {
					return network.EmptyPayout, err
				}
				if err := state.AddReward(snapshot.Validator, due, shares); err != nil {
					return network.EmptyPayout, err
				}
				payouts = append(payouts, reward.Payout{
					Addr:        payable.EcdsaAddress,
					NewlyEarned: due,
					EarningKey:  payable.BLSPublicKey,
				})
			}
		}
		utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTimeLocal).Milliseconds()).Msg("Shard Chain Reward (AddReward)")
		utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("Shard Chain Reward")
	}

	// Block here until the commit sigs are ready or timeout.
	// sigsReady signal indicates that the commit sigs are already populated in the header object.
	if err := waitForCommitSigs(sigsReady); err != nil {
		return network.EmptyPayout, err
	}

	startTime := time.Now()
	// Take care of my own beacon chain committee, _ is missing, for slashing
	parentE, members, payable, missing, err := ballotResultBeaconchain(beaconChain, header)
	if err != nil {
		return network.EmptyPayout, errors.Wrapf(err, "shard 0 block %d reward error with bitmap %x", header.Number(), header.LastCommitBitmap())
	}
	subComm := shard.Committee{ShardID: shard.BeaconChainShardID, Slots: members}

	if err := availability.IncrementValidatorSigningCounts(
		beaconChain,
		subComm.StakedValidators(),
		state,
		payable,
		missing,
	); err != nil {
		return network.EmptyPayout, err
	}
	votingPower, err := lookupVotingPower(
		parentE, &subComm,
	)
	if err != nil {
		return network.EmptyPayout, err
	}

	allSignersShare := numeric.ZeroDec()
	for j := range payable {
		voter := votingPower.Voters[payable[j].BLSPublicKey]
		if !voter.IsHarmonyNode {
			voterShare := voter.OverallPercent
			allSignersShare = allSignersShare.Add(voterShare)
		}
	}
	for beaconMember := range payable {
		blsKey := payable[beaconMember].BLSPublicKey
		voter := votingPower.Voters[blsKey]
		if !voter.IsHarmonyNode {
			snapshot, err := bc.ReadValidatorSnapshot(voter.EarningAccount)
			if err != nil {
				return network.EmptyPayout, err
			}
			due := defaultReward.Mul(
				voter.OverallPercent.Quo(allSignersShare),
			).RoundInt()
			newRewards.Add(newRewards, due)

			shares, err := lookupDelegatorShares(snapshot)
			if err != nil {
				return network.EmptyPayout, err
			}
			if err := state.AddReward(snapshot.Validator, due, shares); err != nil {
				return network.EmptyPayout, err
			}
			payouts = append(payouts, reward.Payout{
				Addr:        voter.EarningAccount,
				NewlyEarned: due,
				EarningKey:  voter.Identity,
			})
		}
	}
	utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("Beacon Chain Reward")

	return network.NewStakingEraRewardForRound(
		newRewards, payouts,
	), nil
}

func processOneCrossLink(bc engine.ChainReader, state *state.DB, cxLink types.CrossLink, defaultReward numeric.Dec, bucket int) ([]slotPayable, []slotMissing, error) {
	epoch, shardID := cxLink.Epoch(), cxLink.ShardID()
	if !bc.Config().IsStaking(epoch) {
		return nil, nil, nil
	}
	startTimeLocal := time.Now()
	shardState, err := bc.ReadShardState(epoch)
	utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTimeLocal).Milliseconds()).Msg("Shard Chain Reward (ReadShardState)")

	if err != nil {
		return nil, nil, err
	}

	subComm, err := shardState.FindCommitteeByID(shardID)
	if err != nil {
		return nil, nil, err
	}

	startTimeLocal = time.Now()
	payableSigners, missing, err := availability.BlockSigners(
		cxLink.Bitmap(), subComm,
	)
	utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTimeLocal).Milliseconds()).Msg("Shard Chain Reward (BlockSigners)")

	if err != nil {
		return nil, nil, errors.Wrapf(err, "shard %d block %d reward error with bitmap %x", shardID, cxLink.BlockNum(), cxLink.Bitmap())
	}

	staked := subComm.StakedValidators()
	startTimeLocal = time.Now()
	if err := availability.IncrementValidatorSigningCounts(
		bc, staked, state, payableSigners, missing,
	); err != nil {
		return nil, nil, err
	}
	utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTimeLocal).Milliseconds()).Msg("Shard Chain Reward (IncrementValidatorSigningCounts)")

	startTimeLocal = time.Now()
	votingPower, err := lookupVotingPower(
		epoch, subComm,
	)
	utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTimeLocal).Milliseconds()).Msg("Shard Chain Reward (lookupVotingPower)")

	if err != nil {
		return nil, nil, err
	}

	allSignersShare := numeric.ZeroDec()
	for j := range payableSigners {
		voter := votingPower.Voters[payableSigners[j].BLSPublicKey]
		if !voter.IsHarmonyNode {
			voterShare := voter.OverallPercent
			allSignersShare = allSignersShare.Add(voterShare)
		}
	}

	allPayables, allMissing := []slotPayable{}, []slotMissing{}
	startTimeLocal = time.Now()
	for j := range payableSigners {
		voter := votingPower.Voters[payableSigners[j].BLSPublicKey]
		if !voter.IsHarmonyNode && !voter.OverallPercent.IsZero() {
			due := defaultReward.Mul(
				voter.OverallPercent.Quo(allSignersShare),
			)
			allPayables = append(allPayables, slotPayable{
				Slot:    payableSigners[j],
				payout:  due.TruncateInt(),
				bucket:  bucket,
				index:   j,
				shardID: shardID,
			})
		}
	}
	utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTimeLocal).Milliseconds()).Msg("Shard Chain Reward (allPayables)")

	for j := range missing {
		allMissing = append(allMissing, slotMissing{
			Slot:   missing[j],
			bucket: bucket,
			index:  j,
		})
	}
	return allPayables, allMissing, nil
}
