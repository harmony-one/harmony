package network

import (
	"errors"
	"math/big"

	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	stakingReward "github.com/harmony-one/harmony/staking/reward"
)

var (
	// ErrPayoutNotEqualBlockReward ..
	ErrPayoutNotEqualBlockReward = errors.New(
		"total payout not equal to blockreward",
	)
	// EmptyPayout ..
	EmptyPayout = noReward{}

	targetStakedPercentage = numeric.MustNewDecFromStr("0.35")
	dynamicAdjust          = numeric.MustNewDecFromStr("0.4")
)

type ignoreMissing struct{}

func (ignoreMissing) MissingSigners() shard.SlotList {
	return shard.SlotList{}
}

type noReward struct{ ignoreMissing }

func (noReward) ReadRoundResult() *reward.CompletedRound {
	return &reward.CompletedRound{
		Total:   big.NewInt(0),
		Payouts: []reward.Payout{},
	}
}

type preStakingEra struct {
	ignoreMissing
	payout *big.Int
}

// NewPreStakingEraRewarded ..
func NewPreStakingEraRewarded(totalAmount *big.Int) reward.Reader {
	return &preStakingEra{ignoreMissing{}, totalAmount}
}

func (p *preStakingEra) ReadRoundResult() *reward.CompletedRound {
	return &reward.CompletedRound{
		Total:   p.payout,
		Payouts: []reward.Payout{},
	}
}

type stakingEra struct {
	reward.CompletedRound
}

// NewStakingEraRewardForRound ..
func NewStakingEraRewardForRound(
	totalPayout *big.Int,
	payouts []reward.Payout,
) reward.Reader {
	return &stakingEra{
		CompletedRound: reward.CompletedRound{
			Total:   totalPayout,
			Payouts: payouts,
		},
	}
}

// ReadRoundResult ..
func (r *stakingEra) ReadRoundResult() *reward.CompletedRound {
	return &r.CompletedRound
}

func adjust(amount numeric.Dec) numeric.Dec {
	return amount.MulTruncate(
		numeric.NewDecFromBigInt(big.NewInt(denominations.One)),
	)
}

// Adjustment ..
func Adjustment(percentageStaked numeric.Dec) (numeric.Dec, numeric.Dec) {
	howMuchOff := targetStakedPercentage.Sub(percentageStaked)
	adjustBy := adjust(
		howMuchOff.MulTruncate(numeric.NewDec(100)).Mul(dynamicAdjust),
	)
	return howMuchOff, adjustBy
}

// WhatPercentStakedNow ..
func WhatPercentStakedNow(
	beaconchain engine.ChainReader,
	timestamp int64,
) (*big.Int, *numeric.Dec, error) {
	stakedNow := numeric.ZeroDec()
	// Only elected validators' stake is counted in stake ratio because only their stake is under slashing risk
	active, err := beaconchain.ReadShardState(beaconchain.CurrentBlock().Epoch())
	if err != nil {
		return nil, nil, err
	}

	soFarDoledOut, err := beaconchain.ReadBlockRewardAccumulator(
		beaconchain.CurrentHeader().Number().Uint64(),
	)

	if err != nil {
		return nil, nil, err
	}

	dole := numeric.NewDecFromBigIntWithPrec(soFarDoledOut, 18)

	for _, electedValAdr := range active.StakedValidators().Addrs {
		wrapper, err := beaconchain.ReadValidatorInformation(electedValAdr)
		if err != nil {
			return nil, nil, err
		}
		stakedNow = stakedNow.Add(
			numeric.NewDecFromBigIntWithPrec(wrapper.TotalDelegation(), 18),
		)
	}
	percentage := stakedNow.Quo(stakingReward.TotalInitialTokens.Mul(
		reward.PercentageForTimeStamp(timestamp),
	).Add(dole))
	utils.Logger().Info().
		Str("so-far-doled-out", dole.String()).
		Str("staked-percentage", percentage.String()).
		Str("currently-staked", stakedNow.String()).
		Msg("Computed how much staked right now")
	return soFarDoledOut, &percentage, nil
}
