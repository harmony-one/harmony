package chain

import (
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/common/denominations"
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
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/pkg/errors"
)

var (
	// BlockReward is the block reward, to be split evenly among block signers.
	BlockReward = new(big.Int).Mul(big.NewInt(24), big.NewInt(denominations.One))
	// BaseStakedReward is the base block reward for epos.
	BaseStakedReward = numeric.NewDecFromBigInt(new(big.Int).Mul(
		big.NewInt(18), big.NewInt(denominations.One),
	))
	// BlockRewardStakedCase is the baseline block reward in staked case -
	totalTokens                  = numeric.NewDecFromBigInt(new(big.Int).Mul(big.NewInt(12600000000), big.NewInt(denominations.One)))
	targetStakedPercentage       = numeric.MustNewDecFromStr("0.35")
	dynamicAdjust                = numeric.MustNewDecFromStr("0.4")
	errPayoutNotEqualBlockReward = errors.New("total payout not equal to blockreward")
	noReward                     = big.NewInt(0)
)

func adjust(amount numeric.Dec) numeric.Dec {
	return amount.MulTruncate(
		numeric.NewDecFromBigInt(big.NewInt(denominations.One)),
	)
}

func ballotResultBeaconchain(
	bc engine.ChainReader, header *block.Header,
) (shard.SlotList, shard.SlotList, shard.SlotList, error) {
	return availability.BallotResult(bc, header, shard.BeaconChainShardID)
}

func whatPercentStakedNow(
	beaconchain engine.ChainReader,
	timestamp int64,
) (*numeric.Dec, error) {
	stakedNow := numeric.ZeroDec()
	// Only active validators' stake is counted in stake ratio because only their stake is under slashing risk
	active, err := beaconchain.ReadActiveValidatorList()
	if err != nil {
		return nil, err
	}

	soFarDoledOut, err := beaconchain.ReadBlockRewardAccumulator(
		beaconchain.CurrentHeader().Number().Uint64(),
	)

	if err != nil {
		return nil, err
	}

	dole := numeric.NewDecFromBigInt(soFarDoledOut)

	for i := range active {
		wrapper, err := beaconchain.ReadValidatorInformation(active[i])
		if err != nil {
			return nil, err
		}
		stakedNow = stakedNow.Add(
			numeric.NewDecFromBigInt(wrapper.TotalDelegation()),
		)
	}
	percentage := stakedNow.Quo(totalTokens.Mul(
		reward.PercentageForTimeStamp(timestamp),
	).Add(dole))
	utils.Logger().Info().
		Str("so-far-doled-out", dole.String()).
		Str("staked-percentage", percentage.String()).
		Str("currently-staked", stakedNow.String()).
		Msg("Computed how much staked right now")
	return &percentage, nil
}

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward
func AccumulateRewards(
	bc engine.ChainReader, state *state.DB, header *block.Header,
	rewarder reward.Distributor, slasher slash.Slasher,
	beaconChain engine.ChainReader,
) (*big.Int, error) {
	blockNum := header.Number().Uint64()

	if blockNum == 0 {
		// genesis block has no parent to reward.
		return noReward, nil
	}

	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() != shard.BeaconChainShardID {
		return noReward, nil
	}

	//// After staking
	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() == shard.BeaconChainShardID {

		defaultReward := BaseStakedReward

		// TODO Use cached result in off-chain db instead of full computation
		percentageStaked, err := whatPercentStakedNow(beaconChain, header.Time().Int64())
		if err != nil {
			return noReward, err
		}
		howMuchOff := targetStakedPercentage.Sub(*percentageStaked)
		adjustBy := adjust(
			howMuchOff.MulTruncate(numeric.NewDec(100)).Mul(dynamicAdjust),
		)
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
			return noReward, nil
		}

		newRewards := big.NewInt(0)

		// Take care of my own beacon chain committee, _ is missing, for slashing
		members, payable, _, err := ballotResultBeaconchain(beaconChain, header)
		if err != nil {
			return noReward, err
		}

		votingPower, err := votepower.Compute(members)
		if err != nil {
			return noReward, err
		}

		for beaconMember := range payable {
			// TODO Give out whatever leftover to the last voter/handle
			// what to do about share of those that didn't sign
			voter := votingPower.Voters[payable[beaconMember].BlsPublicKey]
			if !voter.IsHarmonyNode {
				snapshot, err := bc.ReadValidatorSnapshot(voter.EarningAccount)
				if err != nil {
					return noReward, err
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
				return noReward, err
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
					return noReward, err
				}

				subComm := shardState.FindCommitteeByID(cxLink.ShardID())
				// _ are the missing signers, later for slashing
				payableSigners, _, err := availability.BlockSigners(cxLink.Bitmap(), subComm)

				if err != nil {
					return noReward, err
				}

				votingPower, err := votepower.Compute(payableSigners)
				if err != nil {
					return noReward, err
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
						return noReward, err
					}
					due := resultsHandle[bucket][payThem].payout.TruncateInt()
					newRewards = new(big.Int).Add(newRewards, due)
					state.AddReward(snapshot, due)
				}
			}

			return newRewards, nil
		}
		return noReward, nil
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
		return noReward, nil
	}

	_, signers, _, err := availability.BallotResult(bc, header, header.ShardID())

	if err != nil {
		return noReward, err
	}

	totalAmount := rewarder.Award(
		BlockReward, signers, func(receipient common.Address, amount *big.Int) {
			payable = append(payable, struct {
				string
				common.Address
				*big.Int
			}{common2.MustAddressToBech32(receipient), receipient, amount},
			)
		},
	)

	if totalAmount.Cmp(BlockReward) != 0 {
		utils.Logger().Error().
			Int64("block-reward", BlockReward.Int64()).
			Int64("total-amount-paid-out", totalAmount.Int64()).
			Msg("Total paid out was not equal to block-reward")
		return nil, errors.Wrapf(
			errPayoutNotEqualBlockReward, "payout "+totalAmount.String(),
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
