package economics

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
)

const (
	numBlocksPerYear = 300_000_000
)

var (
	// BlockReward is the block reward, to be split evenly among block signers.
	BlockReward = new(big.Int).Mul(big.NewInt(24), big.NewInt(denominations.One))
	// BaseStakedReward is the base block reward for epos.
	BaseStakedReward = numeric.NewDecFromBigInt(new(big.Int).Mul(
		big.NewInt(18), big.NewInt(denominations.One),
	))
	// BlockRewardStakedCase is the baseline block reward in staked case -
	totalTokens = numeric.NewDecFromBigInt(
		new(big.Int).Mul(big.NewInt(12_600_000_000), big.NewInt(denominations.One)),
	)
	targetStakedPercentage = numeric.MustNewDecFromStr("0.35")
	dynamicAdjust          = numeric.MustNewDecFromStr("0.4")
	oneHundred             = numeric.NewDec(100)
	potentialAdjust        = oneHundred.Mul(dynamicAdjust)
	zero                   = numeric.ZeroDec()
	blocksPerYear          = numeric.NewDec(numBlocksPerYear)
	// ErrPayoutNotEqualBlockReward ..
	ErrPayoutNotEqualBlockReward = errors.New("total payout not equal to blockreward")
	// NoReward ..
	NoReward = common.Big0
)

func adjust(amount numeric.Dec) numeric.Dec {
	return amount.MulTruncate(
		numeric.NewDecFromBigInt(big.NewInt(denominations.One)),
	)
}

// Adjustment ..
func Adjustment(percentageStaked numeric.Dec) (numeric.Dec, numeric.Dec) {
	howMuchOff := targetStakedPercentage.Sub(percentageStaked)
	adjustBy := adjust(
		howMuchOff.MulTruncate(oneHundred).Mul(dynamicAdjust),
	)
	return howMuchOff, adjustBy
}

// Snapshot returns
// (soFarDoledOut, stakedNow, stakedPercentage, error)
func Snapshot(
	beaconchain engine.ChainReader,
	timestamp int64,
	includeAPRs bool,
) (*big.Int, []computedAPR, *numeric.Dec, error) {
	stakedNow, rates, junk :=
		numeric.ZeroDec(), []computedAPR{}, numeric.ZeroDec()
	// Only active validators' stake is counted in
	// stake ratio because only their stake is under slashing risk
	active, err := beaconchain.ReadActiveValidatorList()
	if err != nil {
		return nil, nil, nil, err
	}
	if includeAPRs {
		rates = make([]computedAPR, len(active))
	}
	soFarDoledOut, err := beaconchain.ReadBlockRewardAccumulator(
		beaconchain.CurrentHeader().Number().Uint64(),
	)

	if err != nil {
		return nil, nil, nil, err
	}

	dole := numeric.NewDecFromBigInt(soFarDoledOut)

	for i := range active {
		wrapper, err := beaconchain.ReadValidatorInformation(active[i])
		if err != nil {
			return nil, nil, nil, err
		}
		total := wrapper.TotalDelegation()
		stakedNow = stakedNow.Add(numeric.NewDecFromBigInt(total))
		if includeAPRs {
			rates[i] = computedAPR{active[i], total, junk, numeric.ZeroDec()}
		}
	}

	circulatingSupply := totalTokens.Mul(
		reward.PercentageForTimeStamp(timestamp),
	).Add(dole)

	for i := range rates {
		rates[i].StakeRatio = numeric.NewDecFromBigInt(
			rates[i].TotalStakedToken,
		).Quo(circulatingSupply)

		if reward := BaseStakedReward.Sub(
			rates[i].StakeRatio.Sub(targetStakedPercentage).Mul(potentialAdjust),
		); reward.GT(zero) {
			rates[i].ComputedAPR = blocksPerYear.Mul(reward).Quo(stakedNow)
		}

	}

	percentage := stakedNow.Quo(circulatingSupply)
	utils.Logger().Info().
		Str("so-far-doled-out", dole.String()).
		Str("staked-percentage", percentage.String()).
		Str("currently-staked", stakedNow.String()).
		Msg("Computed how much staked right now")
	return soFarDoledOut, rates, &percentage, nil
}
