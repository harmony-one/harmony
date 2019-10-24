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
// func VotingPower(keeper StakeKeeper) map[]{

// }
// Choose picks the stakers
// func Choose([]*staking.Validator)
