package quorum

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
)

var (
	twoThirds = numeric.NewDec(2).QuoInt64(3)
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
	validatorStakes map[[shard.PublicKeySizeInBytes]byte]stakedVoter
	total           numeric.Dec
}

// Policy ..
func (v *stakedVoteWeight) Policy() Policy {
	return SuperMajorityStake
}

// IsQuorumAchieved ..
func (v *stakedVoteWeight) IsQuorumAchieved(p Phase) bool {
	// TODO Implement this logic
	// fmt.Println("is quorum achieved")
	// soFar := numeric.ZeroDec()
	w := shard.BlsPublicKey{}
	members := v.Participants()

	for i := range members {
		w.FromLibBLSPublicKey(members[i])
		// isHMY := v.validatorStakes[w].isHarmonyNode
		if v.ReadSignature(p, members[i]) == nil {
			//
		}
	}

	return true
}

// QuorumThreshold ..
func (v *stakedVoteWeight) QuorumThreshold() *big.Int {
	// fmt.Println("check quorum threshold")
	return v.total.Mul(twoThirds).Int
}

// RewardThreshold ..
func (v *stakedVoteWeight) IsRewardThresholdAchieved() bool {
	// TODO Implement
	// fmt.Println("check threshold")
	return true
}

// Award ..
func (v *stakedVoteWeight) Award(
	Pie *big.Int, earners []common.Address, hook func(earner common.Address, due *big.Int),
) *big.Int {
	payout := big.NewInt(0)
	last := big.NewInt(0)
	count := big.NewInt(int64(len(earners)))

	s, _ := v.ShardIDProvider()()
	fmt.Println("Award called on shard as staked vote", s)
	proportional := map[common.Address]numeric.Dec{}

	for _, details := range v.validatorStakes {
		if details.isHarmonyNode == false {
			proportional[details.earningAccount] = details.effective.QuoTruncate(
				v.total,
			)
		}
	}

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

	v.validatorStakes = map[[shard.PublicKeySizeInBytes]byte]stakedVoter{}
	v.Reset([]Phase{Prepare, Commit, ViewChange})

	for i := range staked {
		if staked[i].StakeWithDelegationApplied != nil {
			v.validatorStakes[staked[i].BlsPublicKey] = stakedVoter{
				true, false, staked[i].EcdsaAddress, *staked[i].StakeWithDelegationApplied,
			}
			v.total = v.total.Add(*staked[i].StakeWithDelegationApplied)
		} else {
			v.validatorStakes[staked[i].BlsPublicKey] = stakedVoter{
				true, true, staked[i].EcdsaAddress, hSentinel,
			}
		}
	}

	utils.Logger().Info().
		Uint32("on-shard", s).
		Str("Staked", v.total.String()).
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
		v.Policy().String(), s, len(members), parts, v.total.String(),
	})
	return string(b1)
}
