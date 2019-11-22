package quorum

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/pkg/errors"
)

var (
	twoThird      = numeric.NewDec(2).Quo(numeric.NewDec(3))
	ninetyPercent = numeric.MustNewDecFromStr("0.90")
	harmonysShare = numeric.MustNewDecFromStr("0.68")
	stakersShare  = numeric.MustNewDecFromStr("0.32")
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

type stakedVoter struct {
	isActive, isHarmonyNode bool
	earningAccount          common.Address
	effectivePercent        numeric.Dec
	rawStake                numeric.Dec
}

type stakedVoteWeight struct {
	SignatureReader
	DependencyInjectionWriter
	DependencyInjectionReader
	slash.ThresholdDecider
	voters                map[shard.BlsPublicKey]stakedVoter
	ourVotingPowerTotal   numeric.Dec
	theirVotingPowerTotal numeric.Dec
	stakedTotal           numeric.Dec
	hmySlotCount          int64
}

// Policy ..
func (v *stakedVoteWeight) Policy() Policy {
	return SuperMajorityStake
}

// IsQuorumAchieved ..
func (v *stakedVoteWeight) IsQuorumAchieved(p Phase) bool {
	t := v.QuorumThreshold()
	currentTotalPower := v.computeCurrentTotalPower(p)

	utils.Logger().Info().
		Str("policy", v.Policy().String()).
		Str("phase", p.String()).
		Str("threshold", t.String()).
		Str("total-power-of-signers", currentTotalPower.String()).
		Msg("Attempt to reach quorum")
	return currentTotalPower.GT(t)
}

func (v *stakedVoteWeight) computeCurrentTotalPower(p Phase) numeric.Dec {
	w := shard.BlsPublicKey{}
	members := v.Participants()
	currentTotalPower := numeric.ZeroDec()

	for i := range members {
		if v.ReadSignature(p, members[i]) != nil {
			w.FromLibBLSPublicKey(members[i])
			currentTotalPower = currentTotalPower.Add(
				v.voters[w].effectivePercent,
			)
		}
	}

	return currentTotalPower
}

// QuorumThreshold ..
func (v *stakedVoteWeight) QuorumThreshold() numeric.Dec {
	return twoThird
}

// RewardThreshold ..
func (v *stakedVoteWeight) IsRewardThresholdAchieved() bool {
	return v.computeCurrentTotalPower(Commit).GTE(ninetyPercent)
}

// Award ..
func (v *stakedVoteWeight) Award(
	Pie *big.Int, earners []common.Address, hook func(earner common.Address, due *big.Int),
) *big.Int {
	payout := big.NewInt(0)
	last := big.NewInt(0)
	count := big.NewInt(int64(len(earners)))
	// proportional := map[common.Address]numeric.Dec{}

	for _, voter := range v.voters {
		if voter.isHarmonyNode == false {
			// proportional[details.earningAccount] = details.effective.QuoTruncate(
			// 	v.stakedTotal,
			// )
		}
	}
	// TODO Finish implementing this logic w/Chao

	for i := range earners {
		cur := big.NewInt(0)

		cur.Mul(Pie, big.NewInt(int64(i+1))).Div(cur, count)

		diff := big.NewInt(0).Sub(cur, last)

		// hook(common.Address(account), diff)

		payout = big.NewInt(0).Add(payout, diff)

		last = cur
	}

	return payout
}

var (
	errSumOfVotingPowerNotOne   = errors.New("sum of total votes do not sum to 100%")
	errSumOfOursAndTheirsNotOne = errors.New(
		"sum of hmy nodes and stakers do not sum to 100%",
	)
)

func (v *stakedVoteWeight) SetVoters(
	staked shard.SlotList,
) (*TallyResult, error) {
	s, _ := v.ShardIDProvider()()
	v.voters = map[shard.BlsPublicKey]stakedVoter{}
	v.Reset([]Phase{Prepare, Commit, ViewChange})
	v.hmySlotCount = 0
	v.stakedTotal = numeric.ZeroDec()

	for i := range staked {
		if staked[i].StakeWithDelegationApplied == nil {
			v.hmySlotCount++
		} else {
			v.stakedTotal = v.stakedTotal.Add(*staked[i].StakeWithDelegationApplied)
		}
	}

	ourCount := numeric.NewDec(v.hmySlotCount)
	ourPercentage := numeric.ZeroDec()
	theirPercentage := numeric.ZeroDec()
	totalStakedPercent := numeric.ZeroDec()

	for i := range staked {
		member := stakedVoter{
			isActive:         true,
			isHarmonyNode:    true,
			earningAccount:   staked[i].EcdsaAddress,
			effectivePercent: numeric.ZeroDec(),
		}

		// Real Staker
		if staked[i].StakeWithDelegationApplied != nil {
			member.isHarmonyNode = false
			member.effectivePercent = staked[i].StakeWithDelegationApplied.
				Quo(v.stakedTotal).
				Mul(stakersShare)
			theirPercentage = theirPercentage.Add(member.effectivePercent)
		} else { // Our node
			member.effectivePercent = harmonysShare.Quo(ourCount)
			ourPercentage = ourPercentage.Add(member.effectivePercent)
		}

		totalStakedPercent = totalStakedPercent.Add(member.effectivePercent)
		v.voters[staked[i].BlsPublicKey] = member
	}

	utils.Logger().Info().
		Str("our-percentage", ourPercentage.String()).
		Str("their-percentage", theirPercentage.String()).
		Uint32("on-shard", s).
		Str("Raw-Staked", v.stakedTotal.String()).
		Msg("Total staked")

	switch {
	case totalStakedPercent.Equal(totalShare) == false:
		return nil, errSumOfVotingPowerNotOne
	case ourPercentage.Add(theirPercentage).Equal(totalShare) == false:
		return nil, errSumOfOursAndTheirsNotOne
	}

	// Hold onto this calculation
	v.ourVotingPowerTotal = ourPercentage
	v.theirVotingPowerTotal = theirPercentage

	return &TallyResult{ourPercentage, theirPercentage}, nil
}

func (v *stakedVoteWeight) ToggleActive(k *bls.PublicKey) bool {
	w := shard.BlsPublicKey{}
	w.FromLibBLSPublicKey(k)
	g := v.voters[w]
	g.isActive = !g.isActive
	v.voters[w] = g
	return v.voters[w].isActive
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

	parts := make([]u, len(v.voters))
	i := 0

	for identity, voter := range v.voters {
		member := u{
			voter.isHarmonyNode,
			identity.Hex(),
			voter.effectivePercent.String(),
			"",
		}
		if !voter.isHarmonyNode {
			member.RawStake = voter.rawStake.String()
		}
		parts[i] = member
		i++
	}

	b1, _ := json.Marshal(t{
		v.Policy().String(),
		s,
		len(v.voters),
		parts,
		v.ourVotingPowerTotal.String(),
		v.theirVotingPowerTotal.String(),
		v.stakedTotal.String(),
	})
	return string(b1)
}
