package chain

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/harmony-one/harmony/internal/ctxerror"

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
	parentHeader := bc.GetHeaderByHash(header.ParentHash())
	if parentHeader == nil {
		return nil, nil, nil, ctxerror.New(
			"cannot find parent block header in DB",
			"parentHash", header.ParentHash(),
		)
	}
	parentShardState, err := bc.ReadShardState(parentHeader.Epoch())
	if err != nil {
		return nil, nil, nil, ctxerror.New(
			"cannot read shard state", "epoch", parentHeader.Epoch(),
		).WithCause(err)
	}

	return availability.BallotResult(parentHeader, header, parentShardState, shard.BeaconChainShardID)
}

var (
	votingPowerCache singleflight.Group
)

func lookupVotingPower(
	epoch, beaconCurrentEpoch *big.Int, subComm *shard.Committee,
) (*votepower.Roster, error) {
	key := fmt.Sprintf("%s-%d", epoch.String(), subComm.ShardID)
	results, err, _ := votingPowerCache.Do(
		key, func() (interface{}, error) {
			votingPower, err := votepower.Compute(subComm, epoch)
			if err != nil {
				return nil, err
			}
			return votingPower, nil
		},
	)
	if err != nil {
		return nil, err
	}

	// TODO consider if this is the best way to clear the cache
	if new(big.Int).Sub(beaconCurrentEpoch, epoch).Cmp(common.Big3) == 1 {
		go func() {
			votingPowerCache.Forget(key)
		}()
	}

	return results.(*votepower.Roster), nil
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
		members, payable, missing, err := ballotResultBeaconchain(beaconChain, header)
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
		beaconCurrentEpoch := beaconChain.CurrentHeader().Epoch()
		votingPower, err := lookupVotingPower(
			headerE, beaconCurrentEpoch, &subComm,
		)
		if err != nil {
			return network.EmptyPayout, err
		}

		beaconExternalShare := shard.Schedule.InstanceForEpoch(
			headerE,
		).ExternalVotePercent()
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
					voter.OverallPercent.Quo(beaconExternalShare),
				).RoundInt()
				newRewards.Add(newRewards, due)
				if err := state.AddReward(snapshot, due); err != nil {
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
					epoch, beaconCurrentEpoch, subComm,
				)

				if err != nil {
					return network.EmptyPayout, err
				}

				shardExternalShare := shard.Schedule.InstanceForEpoch(
					epoch,
				).ExternalVotePercent()
				for j := range payableSigners {
					voter := votingPower.Voters[payableSigners[j].BLSPublicKey]
					if !voter.IsHarmonyNode && !voter.OverallPercent.IsZero() {
						due := defaultReward.Mul(
							voter.OverallPercent.Quo(shardExternalShare),
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
					if err := state.AddReward(snapshot, due); err != nil {
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
	// TODO ek – retrieving by parent number (blockNum - 1) doesn't work,
	//  while it is okay with hash.  Sounds like DB inconsistency.
	//  Figure out why.
	parentHeader := bc.GetHeaderByHash(header.ParentHash())
	if parentHeader == nil {
		return network.EmptyPayout, ctxerror.New(
			"cannot find parent block header in DB",
			"parentHash", header.ParentHash(),
		)
	}
	if parentHeader.Number().Cmp(common.Big0) == 0 {
		// Parent is an epoch block,
		// which is not signed in the usual manner therefore rewards nothing.
		return network.EmptyPayout, nil
	}
	parentShardState, err := bc.ReadShardState(parentHeader.Epoch())
	if err != nil {
		return nil, ctxerror.New(
			"cannot read shard state", "epoch", parentHeader.Epoch(),
		).WithCause(err)
	}
	_, signers, _, err := availability.BallotResult(parentHeader, header, parentShardState, header.ShardID())

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
