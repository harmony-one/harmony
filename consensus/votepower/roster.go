package votepower

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

var (
	// HarmonysShare ..
	HarmonysShare = numeric.MustNewDecFromStr("0.68")
	// StakersShare ..
	StakersShare = numeric.MustNewDecFromStr("0.32")
	// ErrVotingPowerNotEqualOne ..
	ErrVotingPowerNotEqualOne = errors.New("voting power not equal to one")
)

type stakedVoter struct {
	IsActive         bool               `json:"is-active"`
	IsHarmonyNode    bool               `json:"is-harmony"`
	EarningAccount   common.Address     `json:"earning-account"`
	Identity         shard.BlsPublicKey `json:"bls-public-key"`
	EffectivePercent numeric.Dec        `json:"voting"`
	RawStake         numeric.Dec        `json:"raw-stake"`
}

// Roster ..
type Roster struct {
	Voters                          map[shard.BlsPublicKey]stakedVoter
	OurVotingPowerTotalPercentage   numeric.Dec
	TheirVotingPowerTotalPercentage numeric.Dec
	RawStakedTotal                  numeric.Dec
	HmySlotCount                    int64
}

// JSON dump
func (r *Roster) JSON() string {
	v := map[string]stakedVoter{}
	for k, value := range r.Voters {
		v[k.Hex()] = value
	}
	c := struct {
		Voters map[string]stakedVoter `json:"voters"`
		Our    string                 `json:"ours"`
		Their  string                 `json:"theirs"`
		Raw    string                 `json:"raw-total"`
	}{
		v,
		r.OurVotingPowerTotalPercentage.String(),
		r.TheirVotingPowerTotalPercentage.String(),
		r.RawStakedTotal.String(),
	}
	b, _ := json.Marshal(&c)
	return string(b)
}

// Compute creates a new roster based off the shard.SlotList
func Compute(staked shard.SlotList) (*Roster, error) {
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
	// TODO Check for duplicate BLS Keys
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
	); !diff.IsZero() && lastStakedVoter != nil {
		utils.Logger().Info().
			Str("theirs", theirPercentage.String()).
			Str("ours", ourPercentage.String()).
			Str("diff", diff.String()).
			Str("combined", theirPercentage.Add(diff).Add(ourPercentage).String()).
			Str("bls-public-key-of-receipent", lastStakedVoter.Identity.Hex()).
			Msg("voting power of hmy & staked slots not sum to 1, giving diff to staked slot")
		lastStakedVoter.EffectivePercent = lastStakedVoter.EffectivePercent.Add(diff)
		theirPercentage = theirPercentage.Add(diff)
	}

	if lastStakedVoter != nil &&
		!ourPercentage.Add(theirPercentage).Equal(numeric.OneDec()) {
		utils.Logger().Error().
			Str("theirs", theirPercentage.String()).
			Str("ours", ourPercentage.String()).
			Msg("Total voting power not equal 100 percent")
		return nil, ErrVotingPowerNotEqualOne
	}

	roster.OurVotingPowerTotalPercentage = ourPercentage
	roster.TheirVotingPowerTotalPercentage = theirPercentage

	return roster, nil

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
