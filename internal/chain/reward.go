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
	"github.com/harmony-one/harmony/staking/economics"
	"github.com/pkg/errors"
)

type slotPayable struct {
	payout  numeric.Dec
	payee   common.Address
	shardID uint32
	bucket  int
	index   int
}

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
) (*economics.Produced, error) {
	blockNum := header.Number().Uint64()

	if blockNum == 0 {
		// genesis block has no parent to reward.
		return economics.NewNoReward(blockNum), nil
	}

	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() != shard.BeaconChainShardID {
		return economics.NewNoReward(blockNum), nil
	}

	//// After staking
	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() == shard.BeaconChainShardID {
		defaultReward := economics.BaseStakedReward
		// TODO Use cached result in off-chain db instead of full computation
		_, percentageStaked, err := economics.WhatPercentStakedNow(
			beaconChain, header.Time().Int64(),
		)
		if err != nil {
			return economics.NewNoReward(blockNum), err
		}
		howMuchOff, adjustBy := economics.Adjustment(*percentageStaked)
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
			return economics.NewNoReward(blockNum), nil
		}

		newRewards := big.NewInt(0)
		// Take care of my own beacon chain committee, _ is missing, for slashing
		members, payable, _, err := ballotResultBeaconchain(beaconChain, header)
		allRewards := make(map[common.Address]map[uint32]*big.Int, len(payable))

		if err != nil {
			return economics.NewNoReward(blockNum), err
		}

		votingPower, err := votepower.Compute(members)
		if err != nil {
			return economics.NewNoReward(blockNum), err
		}

		for beaconMember := range payable {
			// TODO Give out whatever leftover to the last voter/handle
			// what to do about share of those that didn't sign
			voter := votingPower.Voters[payable[beaconMember].BlsPublicKey]
			if !voter.IsHarmonyNode {
				snapshot, err := bc.ReadValidatorSnapshot(voter.EarningAccount)
				if err != nil {
					return economics.NewNoReward(blockNum), err
				}
				due := defaultReward.Mul(
					voter.EffectivePercent.Quo(votepower.StakersShare),
				).RoundInt()
				newRewards.Add(newRewards, due)
				allRewards[voter.EarningAccount] = map[uint32]*big.Int{
					shard.BeaconChainShardID: new(big.Int).Set(due),
				}
				state.AddReward(snapshot, due)
			}
		}

		// Handle rewards for shardchain
		if cxLinks := header.CrossLinks(); len(cxLinks) != 0 {
			crossLinks := types.CrossLinks{}
			if err := rlp.DecodeBytes(cxLinks, &crossLinks); err != nil {
				return economics.NewNoReward(blockNum), err
			}
			allPayables := []slotPayable{}

			for i := range crossLinks {
				cxLink := crossLinks[i]
				if !bc.Config().IsStaking(cxLink.Epoch()) {
					continue
				}

				shardState, err := bc.ReadShardState(cxLink.Epoch())

				if err != nil {
					return economics.NewNoReward(blockNum), err
				}

				subComm := shardState.FindCommitteeByID(cxLink.ShardID())
				// _ are the missing signers, later for slashing
				payableSigners, _, err := availability.BlockSigners(
					cxLink.Bitmap(), subComm,
				)

				if err != nil {
					return economics.NewNoReward(blockNum), err
				}

				votingPower, err := votepower.Compute(payableSigners)
				if err != nil {
					return economics.NewNoReward(blockNum), err
				}

				for j := range payableSigners {
					voter := votingPower.Voters[payableSigners[j].BlsPublicKey]
					if !voter.IsHarmonyNode && !voter.EffectivePercent.IsZero() {
						due := defaultReward.Mul(
							voter.EffectivePercent.Quo(votepower.StakersShare),
						)
						to := voter.EarningAccount
						allPayables = append(allPayables, slotPayable{
							payout:  due,
							payee:   to,
							shardID: cxLink.ShardID(),
							bucket:  i,
							index:   j,
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

			// Finally do the pay and record the payouts
			for bucket := range resultsHandle {
				for payThem := range resultsHandle[bucket] {
					addr := resultsHandle[bucket][payThem].payee
					shardID := resultsHandle[bucket][payThem].shardID
					snapshot, err := bc.ReadValidatorSnapshot(addr)
					if err != nil {
						return economics.NewNoReward(blockNum), err
					}
					due := resultsHandle[bucket][payThem].payout.TruncateInt()
					newRewards.Add(newRewards, due)

					if current, exists := allRewards[addr][shardID]; exists {
						current.Add(current, due)
					} else {
						allRewards[addr][shardID] = new(big.Int).Set(due)
					}

					state.AddReward(snapshot, due)
				}
			}

			rewardRecord, i := make([]votepower.ValidatorReward, len(allRewards)), 0

			for validator, payout := range allRewards {
				rewardRecord[i] = votepower.ValidatorReward{
					validator, blockNum, []votepower.ShardReward{},
				}
				bookie := rewardRecord[i].ByShards
				for shardID, reward := range payout {
					bookie = append(bookie, votepower.ShardReward{shardID, reward})
				}
				i++
			}

			return economics.NewProduced(
				blockNum,
				rewardRecord,
				newRewards,
			), nil
		}
		return economics.NewNoReward(blockNum), nil
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
		return economics.NewNoReward(blockNum), nil
	}

	_, signers, _, err := availability.BallotResult(bc, header, header.ShardID())

	if err != nil {
		return economics.NewNoReward(blockNum), err
	}

	totalAmount := rewarder.Award(
		economics.BlockReward, signers, func(receipient common.Address, amount *big.Int) {
			payable = append(payable, struct {
				string
				common.Address
				*big.Int
			}{common2.MustAddressToBech32(receipient), receipient, amount},
			)
		},
	)

	if totalAmount.Cmp(economics.BlockReward) != 0 {
		utils.Logger().Error().
			Int64("block-reward", economics.BlockReward.Int64()).
			Int64("total-amount-paid-out", totalAmount.Int64()).
			Msg("Total paid out was not equal to block-reward")
		return nil, errors.Wrapf(
			economics.ErrPayoutNotEqualBlockReward, "payout "+totalAmount.String(),
		)
	}

	for i := range payable {
		state.AddBalance(payable[i].Address, payable[i].Int)
	}

	header.Logger(utils.Logger()).Debug().
		Int("NumAccounts", len(payable)).
		Str("TotalAmount", totalAmount.String()).
		Msg("[Block Reward] Successfully paid out block reward")

	return economics.NewProduced(
		blockNum,
		[]votepower.ValidatorReward{},
		totalAmount,
	), nil
}
