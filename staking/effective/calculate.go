package effective

import (
	"bytes"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

// medium.com/harmony-one/introducing-harmonys-effective-proof-of-stake-epos-2d39b4b8d58
var (
	two       = numeric.NewDecFromBigInt(big.NewInt(2))
	c, _      = numeric.NewDecFromStr("0.15")
	onePlusC  = numeric.OneDec().Add(c)
	oneMinusC = numeric.OneDec().Sub(c)
)

func effectiveStake(median, actual numeric.Dec) numeric.Dec {
	left := numeric.MinDec(onePlusC.Mul(median), actual)
	right := oneMinusC.Mul(median)
	return numeric.MaxDec(left, right)
}

// SlotPurchase ..
type SlotPurchase struct {
	common.Address     `json:"slot-owner"`
	shard.BlsPublicKey `json:"bls-public-key"`
	numeric.Dec        `json:"eposed-stake"`
}

// SlotOrder ..
type SlotOrder struct {
	Stake       *big.Int
	SpreadAmong []shard.BlsPublicKey
}

// Median ..
func Median(stakes []SlotPurchase) numeric.Dec {
	if len(stakes) == 0 {
		return numeric.ZeroDec()
	}

	sort.SliceStable(
		stakes,
		func(i, j int) bool { return stakes[i].Dec.GT(stakes[j].Dec) },
	)
	const isEven = 0
	switch l := len(stakes); l % 2 {
	case isEven:
		left := (l / 2) - 1
		right := l / 2
		return stakes[left].Dec.Add(stakes[right].Dec).Quo(two)
	default:
		return stakes[l/2].Dec
	}
}

// Compute ..
func Compute(
	shortHand map[common.Address]SlotOrder, pull int,
) (numeric.Dec, []SlotPurchase) {
	eposedSlots := []SlotPurchase{}
	if len(shortHand) == 0 {
		return numeric.ZeroDec(), eposedSlots
	}

	type t struct {
		addr common.Address
		slot SlotOrder
	}

	shorter := []t{}
	for key, value := range shortHand {
		shorter = append(shorter, t{key, value})
	}

	sort.SliceStable(
		shorter,
		func(i, j int) bool {
			return bytes.Compare(
				shorter[i].addr.Bytes(), shorter[j].addr.Bytes(),
			) == -1
		},
	)

	// Expand
	for _, staker := range shorter {
		slotsCount := len(staker.slot.SpreadAmong)
		spread := numeric.NewDecFromBigInt(staker.slot.Stake).
			QuoInt64(int64(slotsCount))
		for i := 0; i < slotsCount; i++ {
			eposedSlots = append(eposedSlots, SlotPurchase{
				staker.addr,
				staker.slot.SpreadAmong[i],
				spread,
			})
		}
	}

	sort.SliceStable(
		eposedSlots,
		func(i, j int) bool { return eposedSlots[i].Dec.GT(eposedSlots[j].Dec) },
	)

	if l := len(eposedSlots); l < pull {
		pull = l
	}
	picks := eposedSlots[:pull]

	if len(picks) == 0 {
		return numeric.ZeroDec(), []SlotPurchase{}
	}

	return Median(picks), picks

}

// Apply ..
func Apply(shortHand map[common.Address]SlotOrder, pull int) []SlotPurchase {
	median, picks := Compute(shortHand, pull)
	for i := range picks {
		picks[i].Dec = effectiveStake(median, picks[i].Dec)
	}

	return picks
}
