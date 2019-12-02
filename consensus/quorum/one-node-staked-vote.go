package quorum

import (
	"encoding/json"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/votepower"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/pkg/errors"
)

var (
	twoThird      = numeric.NewDec(2).Quo(numeric.NewDec(3))
	ninetyPercent = numeric.MustNewDecFromStr("0.90")
	totalShare    = numeric.MustNewDecFromStr("1.00")
)

// TODO Test the case where we have 33 nodes, 68/33 will give precision hell and it should trigger
// the 100% mismatch err.

// TallyResult is the result of when we calculate voting power,
// recall that it happens to us at epoch change
type TallyResult struct {
	ourPercent   numeric.Dec
	theirPercent numeric.Dec
}

type stakedVoteWeight struct {
	SignatureReader
	DependencyInjectionWriter
	DependencyInjectionReader
	slash.ThresholdDecider
	roster votepower.Roster
}

// Policy ..
func (v *stakedVoteWeight) Policy() Policy {
	return SuperMajorityStake
}

// IsQuorumAchieved ..
func (v *stakedVoteWeight) IsQuorumAchieved(p Phase) bool {
	t := v.QuorumThreshold()
	currentTotalPower, err := v.computeCurrentTotalPower(p)

	if err != nil {
		utils.Logger().Error().
			AnErr("bls error", err).
			Msg("Failure in attempt bls-key reading")
		return false
	}

	utils.Logger().Info().
		Str("policy", v.Policy().String()).
		Str("phase", p.String()).
		Str("threshold", t.String()).
		Str("total-power-of-signers", currentTotalPower.String()).
		Msg("Attempt to reach quorum")
	return currentTotalPower.GT(t)
}

// IsQuorumAchivedByMask ..
func (v *stakedVoteWeight) IsQuorumAchievedByMask(mask *bls_cosi.Mask) bool {
	threshold := v.QuorumThreshold()
	currentTotalPower := v.computeTotalPowerByMask(mask)
	if currentTotalPower == nil {
		utils.Logger().Warn().
			Msgf("[IsQuorumAchievedByMask] currentTotalPower is nil")
		return false
	}
	utils.Logger().Info().
		Str("policy", v.Policy().String()).
		Str("threshold", threshold.String()).
		Str("total-power-of-signers", currentTotalPower.String()).
		Msg("[IsQuorumAchievedByMask] Checking quorum")
	return (*currentTotalPower).GT(threshold)
}

func (v *stakedVoteWeight) computeCurrentTotalPower(p Phase) (*numeric.Dec, error) {
	w := shard.BlsPublicKey{}
	members := v.Participants()
	currentTotalPower := numeric.ZeroDec()

	for i := range members {
		if v.ReadSignature(p, members[i]) != nil {
			err := w.FromLibBLSPublicKey(members[i])
			if err != nil {
				return nil, err
			}
			currentTotalPower = currentTotalPower.Add(
				v.roster.Voters[w].EffectivePercent,
			)
		}
	}
	return &currentTotalPower, nil
}

// ComputeTotalPowerByMask computes the total power indicated by bitmap mask
func (v *stakedVoteWeight) computeTotalPowerByMask(mask *bls_cosi.Mask) *numeric.Dec {
	currentTotalPower := numeric.ZeroDec()
	pubKeys := mask.Publics
	for _, key := range pubKeys {
		w := shard.BlsPublicKey{}
		err := w.FromLibBLSPublicKey(key)
		if err != nil {
			return nil
		}
		if enabled, err := mask.KeyEnabled(key); err == nil && enabled {
			currentTotalPower = currentTotalPower.Add(
				v.roster.Voters[w].EffectivePercent,
			)
		}
	}
	return &currentTotalPower
}

// QuorumThreshold ..
func (v *stakedVoteWeight) QuorumThreshold() numeric.Dec {
	return twoThird
}

// RewardThreshold ..
func (v *stakedVoteWeight) IsRewardThresholdAchieved() bool {
	reached, err := v.computeCurrentTotalPower(Commit)
	if err != nil {
		utils.Logger().Error().
			AnErr("bls error", err).
			Msg("Failure in attempt bls-key reading")
		return false
	}
	return reached.GTE(ninetyPercent)
}

// Award ..
// func (v *stakedVoteWeight) Award(
// 	Pie numeric.Dec,
// 	earners []common.Address,
// 	hook func(earner common.Address, due *big.Int),
// ) numeric.Dec {
// 	payout := big.NewInt(0)
// 	last := big.NewInt(0)
// 	count := big.NewInt(int64(len(earners)))
// 	// proportional := map[common.Address]numeric.Dec{}

// 	for _, voter := range v.voters {
// 		if voter.isHarmonyNode == false {
// 			// proportional[details.earningAccount] = details.effective.QuoTruncate(
// 			// 	v.stakedTotal,
// 			// )
// 		}
// 	}
// 	// TODO Finish implementing this logic w/Chao

// 	for i := range earners {
// 		cur := big.NewInt(0)

// 		cur.Mul(Pie, big.NewInt(int64(i+1))).Div(cur, count)

// 		diff := big.NewInt(0).Sub(cur, last)

// 		// hook(common.Address(account), diff)

// 		payout = big.NewInt(0).Add(payout, diff)

// 		last = cur
// 	}

// 	return payout
// }

var (
	errSumOfVotingPowerNotOne   = errors.New("sum of total votes do not sum to 100 percent")
	errSumOfOursAndTheirsNotOne = errors.New(
		"sum of hmy nodes and stakers do not sum to 100 percent",
	)
)

func (v *stakedVoteWeight) SetVoters(
	staked shard.SlotList,
) (*TallyResult, error) {
	s, _ := v.ShardIDProvider()()
	v.Reset([]Phase{Prepare, Commit, ViewChange})

	roster, err := votepower.Compute(staked)
	if err != nil {
		return nil, err
	}
	utils.Logger().Info().
		Str("our-percentage", roster.OurVotingPowerTotalPercentage.String()).
		Str("their-percentage", roster.TheirVotingPowerTotalPercentage.String()).
		Uint32("on-shard", s).
		Str("Raw-Staked", roster.RawStakedTotal.String()).
		Msg("Total staked")

	// Hold onto this calculation
	v.roster = *roster
	return &TallyResult{
		roster.OurVotingPowerTotalPercentage, roster.TheirVotingPowerTotalPercentage,
	}, nil
}

func (v *stakedVoteWeight) ToggleActive(k *bls.PublicKey) bool {
	w := shard.BlsPublicKey{}
	w.FromLibBLSPublicKey(k)
	g := v.roster.Voters[w]
	g.IsActive = !g.IsActive
	v.roster.Voters[w] = g
	return v.roster.Voters[w].IsActive
}

func (v *stakedVoteWeight) ShouldSlash(key shard.BlsPublicKey) bool {
	s, _ := v.ShardIDProvider()()
	switch s {
	case shard.BeaconChainShardID:
		return v.SlashThresholdMet(key)
	default:
		return false
	}
}

func (v *stakedVoteWeight) JSON() string {
	s, _ := v.ShardIDProvider()()
	voterCount := len(v.roster.Voters)

	type u struct {
		IsHarmony   bool   `json:"is-harmony-slot"`
		Identity    string `json:"bls-public-key"`
		VotingPower string `json:"voting-power-%"`
		RawStake    string `json:"raw-stake,omitempty"`
	}

	type t struct {
		Policy            string `json"policy"`
		ShardID           uint32 `json:"shard-id"`
		Count             int    `json:"count"`
		Participants      []u    `json:"committee-members"`
		HmyVotingPower    string `json:"hmy-voting-power"`
		StakedVotingPower string `json:"staked-voting-power"`
		TotalStaked       string `json:"total-raw-staked"`
	}

	parts := make([]u, voterCount)
	i := 0

	for identity, voter := range v.roster.Voters {
		member := u{
			voter.IsHarmonyNode,
			identity.Hex(),
			voter.EffectivePercent.String(),
			"",
		}
		if !voter.IsHarmonyNode {
			member.RawStake = voter.RawStake.String()
		}
		parts[i] = member
		i++
	}

	b1, _ := json.Marshal(t{
		v.Policy().String(),
		s,
		voterCount,
		parts,
		v.roster.OurVotingPowerTotalPercentage.String(),
		v.roster.TheirVotingPowerTotalPercentage.String(),
		v.roster.RawStakedTotal.String(),
	})
	return string(b1)
}

func (v *stakedVoteWeight) AmIMemberOfCommitee() bool {
	pubKeyFunc := v.MyPublicKey()
	if pubKeyFunc == nil {
		return false
	}
	identity, _ := pubKeyFunc()
	w := shard.BlsPublicKey{}
	w.FromLibBLSPublicKey(identity)
	_, ok := v.roster.Voters[w]
	return ok
}
