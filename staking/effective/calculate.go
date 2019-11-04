package effective

import (
	"sort"

	"github.com/harmony-one/harmony/numeric"
)

// medium.com/harmony-one/introducing-harmonys-effective-proof-of-stake-epos-2d39b4b8d58
var (
	c, _      = numeric.NewDecFromStr("0.15")
	onePlusC  = numeric.OneDec().Add(c)
	oneMinusC = numeric.OneDec().Sub(c)
)

// Stake computes the effective proof of stake as descibed in whitepaper
func Stake(median, actual numeric.Dec) numeric.Dec {
	left := numeric.MinDec(onePlusC.Mul(median), actual)
	right := oneMinusC.Mul(median)
	return numeric.MaxDec(left, right)
}

// Median find the median stake
func Median(stakes []numeric.Dec) numeric.Dec {
	sort.SliceStable(
		stakes,
		func(i, j int) bool { return stakes[i].LT(stakes[j]) },
	)
	const isEven = 0
	switch l := len(stakes); l % 2 {
	case isEven:
		return stakes[(l/2)-1].Add(stakes[(l/2)+1]).QuoInt64(2)
	default:
		return stakes[l/2]
	}
}

// NOTE technically should be done here instead of one-node-staked-vote.go but
// have current split of needing some nonstaking based logic makes us leak abstraction
// and need to know if node is harmony node.
// Code in effective should be pure math computation, no business logic.

// VotingPower ..
func VotingPower(keeper StakeKeeper, decidingHook func()) {
	// members := keeper.Inventory()
	// count := len(members.BLSPublicKeys)
	// totalAmount := numeric.ZeroDec()
	// realStakes := []numeric.Dec{}

	// for i := 0; i < count; i++ {
	// 	bPublic := &bls.PublicKey{}
	// 	// TODO handle error
	// 	bPublic.Deserialize(members.BLSPublicKeys[i][:])

	// 	switch stake := members.WithDelegationApplied[i]; stake.Cmp(hSentinel) {
	// 	// Our node
	// 	case 0:
	// 		v.validatorStakes[bPublic] = stakedVoter{
	// 			true, true, hEffectiveSentinel,
	// 		}
	// 	default:
	// 		realStakes = append(realStakes, members.WithDelegationApplied[i])
	// 		v.validatorStakes[bPublic] = stakedVoter{
	// 			true, false, members.WithDelegationApplied[i],
	// 		}
	// 	}
	// }

	// median := effective.Median(realStakes)
	// for _, voter := range v.validatorStakes {
	// 	if !voter.isHarmonyNode {
	// 		voter.effective = effective.Stake(median, voter.effective)
	// 		totalAmount.Add(voter.effective)
	// 	}
	// }

}

// Choose picks the stakers
// func Choose([]*staking.Validator)
