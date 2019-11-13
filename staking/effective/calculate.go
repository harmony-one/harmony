package effective

import (
	"fmt"
	"math/big"
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
func Stake(median, actual *big.Int) numeric.Dec {
	medianDec := numeric.NewDecFromBigInt(median)
	actualDec := numeric.NewDecFromBigInt(actual)
	left := numeric.MinDec(onePlusC.Mul(medianDec), actualDec)
	right := oneMinusC.Mul(medianDec)
	return numeric.MaxDec(left, right)
}

// Median find the median stake
func Median(stakes []*big.Int) *big.Int {
	sort.SliceStable(
		stakes,
		func(i, j int) bool { return stakes[i].Cmp(stakes[j]) <= 0 },
	)
	const isEven = 0
	switch l := len(stakes); l % 2 {
	case isEven:
		middle := new(big.Int).Add(stakes[(l/2)-1], stakes[(l/2)+1])
		return new(big.Int).Div(middle, big.NewInt(2))
	default:
		return stakes[l/2]
	}
}

// Apply ..
func Apply(stakes []*big.Int) []numeric.Dec {
	asNumeric := make([]numeric.Dec, len(stakes))
	median := Median(stakes)

	fmt.Println("Median is:", median.Uint64())

	for i := range stakes {
		asNumeric[i] = Stake(median, stakes[i])
	}
	return asNumeric
}
