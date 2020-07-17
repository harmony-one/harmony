package chain

import (
	"fmt"
	"math/big"
	"sort"

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
	"github.com/pkg/errors"
)

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

// AccumulateRewardsAndCountSigs credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward
// This func also do IncrementValidatorSigningCounts for validators
func AccumulateRewardsAndCountSigs(
	bc engine.ChainReader, state *state.DB,
	header *block.Header, beaconChain engine.ChainReader,
) (reward.Reader, error) {
	blockNum := header.Number().Uint64()
	currentHeader := beaconChain.CurrentHeader()
	nowEpoch, blockNow := currentHeader.Epoch(), currentHeader.Number()

	if blockNum == 0 {
		// genesis block has no parent to reward.
		return network.EmptyPayout, nil
	}

	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() != shard.BeaconChainShardID {
		return network.EmptyPayout, nil

	}

	// After staking
	if headerE := header.Epoch(); bc.Config().IsStaking(headerE) &&
		bc.CurrentHeader().ShardID() == shard.BeaconChainShardID {
		utils.AnalysisStart("accumulateRewardBeaconchainSelfPayout", nowEpoch, blockNow)
		defaultReward := network.BaseStakedReward

		// Following is commented because the new econ-model has a flat-rate block reward
		// of 28 ONE per block assuming 4 shards and 8s block time:
		//// TODO Use cached result in off-chain db instead of full computation
		//_, percentageStaked, err := network.WhatPercentStakedNow(
		//	beaconChain, header.Time().Int64(),
		//)
		//if err != nil {
		//	return network.EmptyPayout, err
		//}
		//howMuchOff, adjustBy := network.Adjustment(*percentageStaked)
		//defaultReward = defaultReward.Add(adjustBy)
		//utils.Logger().Info().
		//	Str("percentage-token-staked", percentageStaked.String()).
		//	Str("how-much-off", howMuchOff.String()).
		//	Str("adjusting-by", adjustBy.String()).
		//	Str("block-reward", defaultReward.String()).
		//	Msg("dynamic adjustment of block-reward ")

		// If too much is staked, then possible to have negative reward,
		// not an error, just a possible economic situation, hence we return
		if defaultReward.IsNegative() {
			return network.EmptyPayout, nil
		}

		newRewards, beaconP, shardP :=
			big.NewInt(0), []reward.Payout{}, []reward.Payout{}

		// Take care of my own beacon chain committee, _ is missing, for slashing
		parentE, members, payable, missing, err := ballotResultBeaconchain(beaconChain, header)
		if err != nil {
			return network.EmptyPayout, err
		}
		subComm := shard.Committee{shard.BeaconChainShardID, members}

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
			// TODO Give out whatever leftover to the last voter/handle
			// what to do about share of those that didn't sign
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
				beaconP = append(beaconP, reward.Payout{
					ShardID:     shard.BeaconChainShardID,
					Addr:        voter.EarningAccount,
					NewlyEarned: due,
					EarningKey:  voter.Identity,
				})
			}
		}
		utils.AnalysisEnd("accumulateRewardBeaconchainSelfPayout", nowEpoch, blockNow)

		utils.AnalysisStart("accumulateRewardShardchainPayout", nowEpoch, blockNow)
		// Handle rewards for shardchain
		if cxLinks := header.CrossLinks(); len(cxLinks) > 0 {
			crossLinks := types.CrossLinks{}
			if err := rlp.DecodeBytes(cxLinks, &crossLinks); err != nil {
				return network.EmptyPayout, err
			}

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

			allPayables, allMissing := []slotPayable{}, []slotMissing{}

			for i := range crossLinks {

				cxLink := crossLinks[i]
				epoch, shardID := cxLink.Epoch(), cxLink.ShardID()
				if !bc.Config().IsStaking(epoch) {
					continue
				}
				shardState, err := bc.ReadShardState(epoch)

				if err != nil {
					return network.EmptyPayout, err
				}

				subComm, err := shardState.FindCommitteeByID(shardID)
				if err != nil {
					return network.EmptyPayout, err
				}

				payableSigners, missing, err := availability.BlockSigners(
					cxLink.Bitmap(), subComm,
				)
				if err != nil {
					return network.EmptyPayout, err
				}

				staked := subComm.StakedValidators()
				if err := availability.IncrementValidatorSigningCounts(
					beaconChain, staked, state, payableSigners, missing,
				); err != nil {
					return network.EmptyPayout, err
				}

				votingPower, err := lookupVotingPower(
					epoch, subComm,
				)

				if err != nil {
					return network.EmptyPayout, err
				}

				allSignersShare := numeric.ZeroDec()
				for j := range payableSigners {
					voter := votingPower.Voters[payableSigners[j].BLSPublicKey]
					if !voter.IsHarmonyNode {
						voterShare := voter.OverallPercent
						allSignersShare = allSignersShare.Add(voterShare)
					}
				}

				for j := range payableSigners {
					voter := votingPower.Voters[payableSigners[j].BLSPublicKey]
					if !voter.IsHarmonyNode && !voter.OverallPercent.IsZero() {
						due := defaultReward.Mul(
							voter.OverallPercent.Quo(allSignersShare),
						)
						allPayables = append(allPayables, slotPayable{
							Slot:    payableSigners[j],
							payout:  due.TruncateInt(),
							bucket:  i,
							index:   j,
							shardID: shardID,
						})
					}
				}

				for j := range missing {
					allMissing = append(allMissing, slotMissing{
						Slot:   missing[j],
						bucket: i,
						index:  j,
					})
				}
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
					shardP = append(shardP, reward.Payout{
						ShardID:     payable.shardID,
						Addr:        payable.EcdsaAddress,
						NewlyEarned: due,
						EarningKey:  payable.BLSPublicKey,
					})
				}
			}
			utils.AnalysisEnd("accumulateRewardShardchainPayout", nowEpoch, blockNow)
			return network.NewStakingEraRewardForRound(
				newRewards, missing, beaconP, shardP,
			), nil
		}
		return network.EmptyPayout, nil
	}

	// Before staking
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
			cur.Mul(network.BlockReward, big.NewInt(int64(i+1))).Div(cur, count)
			diff := big.NewInt(0).Sub(cur, last)
			state.AddBalance(account.EcdsaAddress, diff)
			totalAmount.Add(totalAmount, diff)
			last = cur
		}
	}

	if totalAmount.Cmp(network.BlockReward) != 0 {
		utils.Logger().Error().
			Int64("block-reward", network.BlockReward.Int64()).
			Int64("total-amount-paid-out", totalAmount.Int64()).
			Msg("Total paid out was not equal to block-reward")
		return nil, errors.Wrapf(
			network.ErrPayoutNotEqualBlockReward, "payout "+totalAmount.String(),
		)
	}

	return network.NewPreStakingEraRewarded(totalAmount), nil
}
