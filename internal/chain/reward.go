package chain

import (
	"fmt"
	"math/big"
	"sort"

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
	"golang.org/x/sync/singleflight"
)

func ballotResultBeaconchain(
	bc engine.ChainReader, header *block.Header,
) (shard.SlotList, shard.SlotList, shard.SlotList, error) {
	return availability.BallotResult(bc, header, shard.BeaconChainShardID)
}

var (
	superCommitteeCache, votingPowerCache singleflight.Group
	maxMemoryBehindShard                  = big.NewInt(5)
)

func lookupVotingPower(
	epochIncoming *big.Int, shardID uint32,
	epochNow *big.Int, beaconChain engine.ChainReader,
) (*votepower.Roster, error) {
	key := fmt.Sprintf("%s-%d", epochIncoming.String(), shardID)
	results, err, _ := votingPowerCache.Do(
		key, func() (interface{}, error) {
			subComm, err := lookupSubCommittee(
				epochIncoming, shardID, epochNow, beaconChain,
			)
			if err != nil {
				return nil, err
			}
			votingPower, err := votepower.Compute(subComm.Slots)
			if err != nil {
				return nil, err
			}
			return votingPower, nil
		},
	)
	if err != nil {
		return nil, err
	}
	if epochIncoming.Abs(new(big.Int).Sub(
		epochNow, epochIncoming,
	)).Cmp(maxMemoryBehindShard) == 1 {
		votingPowerCache.Forget(key)
	}
	return results.(*votepower.Roster), nil
}

func lookupSubCommittee(
	epochIncoming *big.Int, shardID uint32,
	epochNow *big.Int, beaconChain engine.ChainReader,
) (*shard.Committee, error) {
	key := fmt.Sprintf("%s-%d", epochIncoming.String(), shardID)
	results, err, _ := superCommitteeCache.Do(
		key, func() (interface{}, error) {
			superCommittee, err := beaconChain.ReadShardState(epochIncoming)
			if err != nil {
				return nil, err
			}
			subComm, err := superCommittee.FindCommitteeByID(shardID)
			if err != nil {
				return nil, err
			}
			return subComm, nil
		},
	)
	if err != nil {
		return nil, err
	}
	if epochIncoming.Abs(new(big.Int).Sub(
		epochNow, epochIncoming,
	)).Cmp(maxMemoryBehindShard) == 1 {
		superCommitteeCache.Forget(key)
	}

	return results.(*shard.Committee), nil
}

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward
func AccumulateRewards(
	bc engine.ChainReader, state *state.DB,
	header *block.Header, beaconChain engine.ChainReader,
) (reward.Reader, error) {
	blockNum := header.Number().Uint64()
	beaconEpochNow := beaconChain.CurrentHeader().Epoch()

	if blockNum == 0 {
		// genesis block has no parent to reward.
		return network.EmptyPayout, nil
	}

	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() != shard.BeaconChainShardID {
		return network.EmptyPayout, nil
	}

	// After staking
	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() == shard.BeaconChainShardID {
		defaultReward := network.BaseStakedReward
		// TODO Use cached result in off-chain db instead of full computation
		_, percentageStaked, err := network.WhatPercentStakedNow(
			beaconChain, header.Time().Int64(),
		)
		if err != nil {
			return network.EmptyPayout, err
		}
		howMuchOff, adjustBy := network.Adjustment(*percentageStaked)
		defaultReward = defaultReward.Add(adjustBy)
		utils.Logger().Info().
			Str("percentage-token-staked", percentageStaked.String()).
			Str("how-much-off", howMuchOff.String()).
			Str("adjusting-by", adjustBy.String()).
			Str("block-reward", defaultReward.String()).
			Msg("dynamic adjustment of block-reward ")
		// If too much is staked, then possible to have negative reward,
		// not an error, just a possible economic situation, hence we return
		if defaultReward.IsNegative() {
			return network.EmptyPayout, nil
		}

		newRewards := big.NewInt(0)

		// Take care of my own beacon chain committee, _ is missing, for slashing
		members, payable, missing, err := ballotResultBeaconchain(beaconChain, header)
		if err != nil {
			return network.EmptyPayout, err
		}

		if err := availability.IncrementValidatorSigningCounts(
			beaconChain,
			shard.Committee{shard.BeaconChainShardID, members}.StakedValidators(),
			state,
			payable,
			missing,
		); err != nil {
			return network.EmptyPayout, err
		}

		votingPower, err := votepower.Compute(members)
		if err != nil {
			return network.EmptyPayout, err
		}

		for beaconMember := range payable {
			// TODO Give out whatever leftover to the last voter/handle
			// what to do about share of those that didn't sign
			voter := votingPower.Voters[payable[beaconMember].BlsPublicKey]
			if !voter.IsHarmonyNode {
				snapshot, err := bc.ReadValidatorSnapshot(voter.EarningAccount)
				if err != nil {
					return network.EmptyPayout, err
				}
				due := defaultReward.Mul(
					voter.EffectivePercent.Quo(votepower.StakersShare),
				).RoundInt()
				newRewards = new(big.Int).Add(newRewards, due)
				if err := state.AddReward(snapshot, due); err != nil {
					return network.EmptyPayout, err
				}
			}
		}

		// Handle rewards for shardchain
		if cxLinks := header.CrossLinks(); len(cxLinks) > 0 {
			crossLinks := types.CrossLinks{}
			if err := rlp.DecodeBytes(cxLinks, &crossLinks); err != nil {
				return network.EmptyPayout, err
			}

			type slotPayable struct {
				shard.Slot
				payout *big.Int
				bucket int
				index  int
			}

			type slotMissing struct {
				shard.Slot
				bucket int
				index  int
			}

			allPayables, allMissing := []slotPayable{}, []slotMissing{}

			for i := range crossLinks {
				cxLink := crossLinks[i]
				if !bc.Config().IsStaking(cxLink.Epoch()) {
					continue
				}
				epoch, shardID := cxLink.Epoch(), cxLink.ShardID()

				subComm, err := lookupSubCommittee(
					epoch, shardID, beaconEpochNow, beaconChain,
				)

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
					epoch, shardID, beaconEpochNow, beaconChain,
				)

				if err != nil {
					return network.EmptyPayout, err
				}
				for j := range payableSigners {
					voter := votingPower.Voters[payableSigners[j].BlsPublicKey]
					if !voter.IsHarmonyNode && !voter.EffectivePercent.IsZero() {
						due := defaultReward.Mul(
							voter.EffectivePercent.Quo(votepower.StakersShare),
						)
						allPayables = append(allPayables, slotPayable{
							Slot:   payableSigners[j],
							payout: due.TruncateInt(),
							bucket: i,
							index:  j,
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
					snapshot, err := bc.ReadValidatorSnapshot(
						resultsHandle[bucket][payThem].EcdsaAddress,
					)
					if err != nil {
						return network.EmptyPayout, err
					}
					due := resultsHandle[bucket][payThem].payout
					newRewards.Add(newRewards, due)
					if err := state.AddReward(snapshot, due); err != nil {
						return network.EmptyPayout, err
					}
				}
			}

			return network.NewStakingEraRewardForRound(newRewards, missing), nil
		}
		return network.EmptyPayout, nil
	}

	// Before staking
	parentHeader := bc.GetHeaderByHash(header.ParentHash())
	if parentHeader.Number().Cmp(common.Big0) == 0 {
		// Parent is an epoch block,
		// which is not signed in the usual manner therefore rewards nothing.
		return network.EmptyPayout, nil
	}

	_, signers, _, err := availability.BallotResult(bc, header, header.ShardID())

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
