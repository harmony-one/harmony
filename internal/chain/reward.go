package chain

import (
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
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/availability"
	"github.com/harmony-one/harmony/staking/network"
	"github.com/pkg/errors"
)

func ballotResultBeaconchain(
	bc engine.ChainReader, header *block.Header,
) (shard.SlotList, shard.SlotList, shard.SlotList, error) {
	return availability.BallotResult(bc, header, shard.BeaconChainShardID)
}

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward
func AccumulateRewards(
	bc engine.ChainReader, state *state.DB, header *block.Header,
	rewarder reward.Distributor, beaconChain engine.ChainReader,
) (*big.Int, error) {
	blockNum := header.Number().Uint64()

	if blockNum == 0 {
		// genesis block has no parent to reward.
		return network.NoReward, nil
	}

	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() != shard.BeaconChainShardID {
		return network.NoReward, nil
	}

	//// After staking
	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() == shard.BeaconChainShardID {

		defaultReward := network.BaseStakedReward

		// TODO Use cached result in off-chain db instead of full computation
		_, percentageStaked, err := network.WhatPercentStakedNow(beaconChain, header.Time().Int64())
		if err != nil {
			return network.NoReward, err
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
			return network.NoReward, nil
		}

		newRewards := big.NewInt(0)

		// Take care of my own beacon chain committee, _ is missing, for slashing
		members, payable, _, err := ballotResultBeaconchain(beaconChain, header)
		if err != nil {
			return network.NoReward, err
		}

		votingPower, err := votepower.Compute(members)
		if err != nil {
			return network.NoReward, err
		}

		for beaconMember := range payable {
			// TODO Give out whatever leftover to the last voter/handle
			// what to do about share of those that didn't sign
			voter := votingPower.Voters[payable[beaconMember].BlsPublicKey]
			if !voter.IsHarmonyNode {
				snapshot, err := bc.ReadValidatorSnapshot(voter.EarningAccount)
				if err != nil {
					return network.NoReward, err
				}
				due := defaultReward.Mul(
					voter.EffectivePercent.Quo(votepower.StakersShare),
				).RoundInt()
				newRewards = new(big.Int).Add(newRewards, due)
				state.AddReward(snapshot, due)
			}
		}

		// Handle rewards for shardchain
		if cxLinks := header.CrossLinks(); len(cxLinks) != 0 {
			crossLinks := types.CrossLinks{}
			err := rlp.DecodeBytes(cxLinks, &crossLinks)
			if err != nil {
				return network.NoReward, err
			}

			type slotPayable struct {
				payout numeric.Dec
				payee  common.Address
				bucket int
				index  int
			}

			allPayables := []slotPayable{}

			for i := range crossLinks {
				cxLink := crossLinks[i]
				if !bc.Config().IsStaking(cxLink.Epoch()) {
					continue
				}

				shardState, err := bc.ReadShardState(cxLink.Epoch())

				if err != nil {
					return network.NoReward, err
				}

				subComm := shardState.FindCommitteeByID(cxLink.ShardID())
				// _ are the missing signers, later for slashing
				payableSigners, _, err := availability.BlockSigners(cxLink.Bitmap(), subComm)

				if err != nil {
					return network.NoReward, err
				}

				votingPower, err := votepower.Compute(payableSigners)
				if err != nil {
					return network.NoReward, err
				}
				for j := range payableSigners {
					voter := votingPower.Voters[payableSigners[j].BlsPublicKey]
					if !voter.IsHarmonyNode && !voter.EffectivePercent.IsZero() {
						due := defaultReward.Mul(
							voter.EffectivePercent.Quo(votepower.StakersShare),
						)
						to := voter.EarningAccount
						allPayables = append(allPayables, slotPayable{
							payout: due,
							payee:  to,
							bucket: i,
							index:  j,
						})
					}
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
					snapshot, err := bc.ReadValidatorSnapshot(resultsHandle[bucket][payThem].payee)
					if err != nil {
						return network.NoReward, err
					}
					due := resultsHandle[bucket][payThem].payout.TruncateInt()
					newRewards = new(big.Int).Add(newRewards, due)
					state.AddReward(snapshot, due)
				}
			}

			return newRewards, nil
		}
		return network.NoReward, nil
	}

	//// Before staking
	payable := []struct {
		string
		common.Address
		*big.Int
	}{}

	parentHeader := bc.GetHeaderByHash(header.ParentHash())
	if parentHeader.Number().Cmp(common.Big0) == 0 {
		// Parent is an epoch block,
		// which is not signed in the usual manner therefore rewards nothing.
		return network.NoReward, nil
	}

	_, signers, _, err := availability.BallotResult(bc, header, header.ShardID())

	if err != nil {
		return network.NoReward, err
	}

	totalAmount := rewarder.Award(
		network.BlockReward, signers, func(receipient common.Address, amount *big.Int) {
			payable = append(payable, struct {
				string
				common.Address
				*big.Int
			}{common2.MustAddressToBech32(receipient), receipient, amount},
			)
		},
	)

	if totalAmount.Cmp(network.BlockReward) != 0 {
		utils.Logger().Error().
			Int64("block-reward", network.BlockReward.Int64()).
			Int64("total-amount-paid-out", totalAmount.Int64()).
			Msg("Total paid out was not equal to block-reward")
		return nil, errors.Wrapf(
			network.ErrPayoutNotEqualBlockReward, "payout "+totalAmount.String(),
		)
	}

	for i := range payable {
		state.AddBalance(payable[i].Address, payable[i].Int)
	}

	header.Logger(utils.Logger()).Debug().
		Int("NumAccounts", len(payable)).
		Str("TotalAmount", totalAmount.String()).
		Msg("[Block Reward] Successfully paid out block reward")

	return totalAmount, nil
}
