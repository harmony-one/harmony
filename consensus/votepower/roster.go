package votepower

import (
	"github.com/ethereum/go-ethereum/common"
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

// Compute ..
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

	for i := range staked {
		member := stakedVoter{
			IsActive:         true,
			IsHarmonyNode:    true,
			EarningAccount:   staked[i].EcdsaAddress,
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
		} else { // Our node
			// TODO See the todo on where this called in one-node-staked-vote,
			// need to have these two values of our
			// percentage and hmy percentage sum to 1
			member.EffectivePercent = HarmonysShare.Quo(ourCount)
			ourPercentage = ourPercentage.Add(member.EffectivePercent)
		}

		roster.Voters[staked[i].BlsPublicKey] = member
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
