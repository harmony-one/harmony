package votepower

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

var (
	// HarmonysShare ..
	HarmonysShare = numeric.MustNewDecFromStr("0.68")
	// StakersShare ..
	StakersShare = numeric.MustNewDecFromStr("0.32")
)

type stakedVoter struct {
	IsActive, IsHarmonyNode bool
	EarningAccount          common.Address
	Identity                shard.BlsPublicKey
	EffectivePercent        numeric.Dec
	RawStake                numeric.Dec
}

// Roster ..
type Roster struct {
	Voters                          map[shard.BlsPublicKey]stakedVoter
	OurVotingPowerTotalPercentage   numeric.Dec
	TheirVotingPowerTotalPercentage numeric.Dec
	RawStakedTotal                  numeric.Dec
	HmySlotCount                    int64
}

// Compute creates a new roster based off the shard.SlotList
func Compute(staked shard.SlotList) *Roster {
	roster := NewRoster()
	for i := range staked {
		if staked[i].TotalStake == nil {
			roster.HmySlotCount++
		} else {
			roster.RawStakedTotal = roster.RawStakedTotal.Add(
				*staked[i].TotalStake,
			)
		}
	}

	ourCount := numeric.NewDec(roster.HmySlotCount)
	ourPercentage := numeric.ZeroDec()
	theirPercentage := numeric.ZeroDec()
	var lastStakedVoter *stakedVoter = nil

	for i := range staked {
		member := stakedVoter{
			IsActive:         true,
			IsHarmonyNode:    true,
			EarningAccount:   staked[i].EcdsaAddress,
			Identity:         staked[i].BlsPublicKey,
			EffectivePercent: numeric.ZeroDec(),
			RawStake:         numeric.ZeroDec(),
		}

		// Real Staker
		if staked[i].TotalStake != nil {
			member.IsHarmonyNode = false
			member.RawStake = member.RawStake.Add(*staked[i].TotalStake)
			member.EffectivePercent = staked[i].TotalStake.
				Quo(roster.RawStakedTotal).
				Mul(StakersShare)
			theirPercentage = theirPercentage.Add(member.EffectivePercent)
			lastStakedVoter = &member
		} else { // Our node
			// TODO See the todo on where this called in one-node-staked-vote,
			// need to have these two values of our
			// percentage and hmy percentage sum to 1
			member.EffectivePercent = HarmonysShare.Quo(ourCount)
			ourPercentage = ourPercentage.Add(member.EffectivePercent)
		}

		roster.Voters[staked[i].BlsPublicKey] = member
	}

	// NOTE Enforce voting power sums to one, give diff (expect tiny amt) to last staked voter
	if diff := numeric.OneDec().Sub(
		ourPercentage.Add(theirPercentage),
	); diff.GT(numeric.ZeroDec()) && lastStakedVoter != nil {
		lastStakedVoter.EffectivePercent = lastStakedVoter.EffectivePercent.Add(diff)
		theirPercentage = theirPercentage.Add(diff)
		utils.Logger().Info().
			Str("diff", diff.String()).
			Str("bls-public-key-of-receipent", lastStakedVoter.Identity.Hex()).
			Msg("sum of voting power of hmy & staked slots not equal to 1, gave diff to staked slot")
	}

	roster.OurVotingPowerTotalPercentage = ourPercentage
	roster.TheirVotingPowerTotalPercentage = theirPercentage

	return roster

}

// NewRoster ..
func NewRoster() *Roster {
	return &Roster{
		map[shard.BlsPublicKey]stakedVoter{},
		numeric.ZeroDec(),
		numeric.ZeroDec(),
		numeric.ZeroDec(),
		0,
	}
}
