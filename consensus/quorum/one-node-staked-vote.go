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
)

var (
	one   = numeric.OneDec()
	two   = numeric.NewDec(2)
	three = numeric.NewDec(3)

	twoThirds = two.Quo(three)
	// ninetyPercent = numeric.NewDecFromStr("0.90")
	hSentinel = numeric.ZeroDec()
)

type stakedVoter struct {
	isActive, isHarmonyNode bool
	earningAccount          common.Address
	effective               numeric.Dec
}

type stakedVoteWeight struct {
	SignatureReader
	DependencyInjectionWriter
	DependencyInjectionReader
	slash.ThresholdDecider
	validatorStakes map[shard.BlsPublicKey]stakedVoter
	stakedTotal     numeric.Dec
	hmySlotCount    int64
}

// Policy ..
func (v *stakedVoteWeight) Policy() Policy {
	return SuperMajorityStake
}

// IsQuorumAchieved ..
func (v *stakedVoteWeight) IsQuorumAchieved(p Phase) bool {
	w := shard.BlsPublicKey{}
	members := v.Participants()
	currentTotalPower := numeric.ZeroDec()
	ourCount := numeric.NewDecFromBigInt(big.NewInt(v.hmySlotCount))

	for i := range members {
		if v.ReadSignature(p, members[i]) != nil {
			w.FromLibBLSPublicKey(members[i])
			if v.validatorStakes[w].isHarmonyNode {
				currentTotalPower = currentTotalPower.Add(
					two.Mul(v.stakedTotal).Add(one).Quo(ourCount),
				)
			} else {
				currentTotalPower = currentTotalPower.Add(
					v.validatorStakes[w].effective,
				)
			}
		}
	}

	t := numeric.NewDecFromBigInt(v.QuorumThreshold())

	utils.Logger().Info().
		Str("policy", v.Policy().String()).
		Str("phase", p.String()).
		Str("threshold", t.String()).
		Str("total-power-of-signers", currentTotalPower.String()).
		Msg("Attempt to reach quorum")

	return currentTotalPower.GT(t)
}

// QuorumThreshold ..
func (v *stakedVoteWeight) QuorumThreshold() *big.Int {
	return three.Mul(v.stakedTotal).Add(one).Mul(twoThirds).RoundInt()
}

// RewardThreshold ..
func (v *stakedVoteWeight) IsRewardThresholdAchieved() bool {
	// TODO Implement
	return true
}

// Award ..
func (v *stakedVoteWeight) Award(
	Pie *big.Int, earners []common.Address, hook func(earner common.Address, due *big.Int),
) *big.Int {
	payout := big.NewInt(0)
	last := big.NewInt(0)
	count := big.NewInt(int64(len(earners)))
	proportional := map[common.Address]numeric.Dec{}

	for _, details := range v.validatorStakes {
		if details.isHarmonyNode == false {
			proportional[details.earningAccount] = details.effective.QuoTruncate(
				v.stakedTotal,
			)
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

func (v *stakedVoteWeight) UpdateVotingPower(staked shard.SlotList) {
	s, _ := v.ShardIDProvider()()

	v.validatorStakes = map[shard.BlsPublicKey]stakedVoter{}
	v.Reset([]Phase{Prepare, Commit, ViewChange})

	for i := range staked {
		// Real Staker
		if staked[i].StakeWithDelegationApplied != nil {
			v.validatorStakes[staked[i].BlsPublicKey] = stakedVoter{
				true, false, staked[i].EcdsaAddress, *staked[i].StakeWithDelegationApplied,
			}
			v.stakedTotal = v.stakedTotal.Add(*staked[i].StakeWithDelegationApplied)
		} else { // Our node
			v.validatorStakes[staked[i].BlsPublicKey] = stakedVoter{
				true, true, staked[i].EcdsaAddress, hSentinel,
			}
			v.hmySlotCount++
		}
	}

	utils.Logger().Info().
		Uint32("on-shard", s).
		Str("Staked", v.stakedTotal.String()).
		Msg("Total staked")
}

func (v *stakedVoteWeight) ToggleActive(k *bls.PublicKey) bool {
	w := shard.BlsPublicKey{}
	w.FromLibBLSPublicKey(k)
	g := v.validatorStakes[w]
	g.isActive = !g.isActive
	v.validatorStakes[w] = g
	return v.validatorStakes[w].isActive
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

	type t struct {
		Policy       string   `json"policy"`
		ShardID      uint32   `json:"shard-id"`
		Count        int      `json:"count"`
		Participants []string `json:"committee-members"`
		TotalStaked  string   `json:"total-staked"`
	}

	members := v.DumpParticipants()
	parts := []string{}
	for i := range members {
		k := bls.PublicKey{}
		k.DeserializeHexStr(members[i])
		w := shard.BlsPublicKey{}
		w.FromLibBLSPublicKey(&k)
		staker := v.validatorStakes[w]
		if staker.isHarmonyNode {
			parts = append(parts, members[i])
		} else {
			parts = append(parts, members[i]+"-"+staker.effective.String())
		}
	}
	b1, _ := json.Marshal(t{
		v.Policy().String(), s, len(members), parts, v.stakedTotal.String(),
	})
	return string(b1)
}
