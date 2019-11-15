package effective

import (
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

func stake(median, actual numeric.Dec) numeric.Dec {
	left := numeric.MinDec(onePlusC.Mul(median), actual)
	right := oneMinusC.Mul(median)
	return numeric.MaxDec(left, right)
}

// SlotPurchase ..
type SlotPurchase struct {
	common.Address `json:"slot-owner"`
	numeric.Dec    `json:"eposed-stake"`
}

// SlotOrder ..
type SlotOrder struct {
	Stake       *big.Int
	SpreadAmong []shard.BlsPublicKey
}

// Slots ..
type Slots []SlotPurchase

func median(stakes []SlotPurchase) numeric.Dec {
	sort.SliceStable(
		stakes,
		func(i, j int) bool { return stakes[i].Dec.LTE(stakes[j].Dec) },
	)
	const isEven = 0
	switch l := len(stakes); l % 2 {
	case isEven:
		return stakes[(l/2)-1].Dec.Add(stakes[(l/2)+1].Dec).Quo(two)
	default:
		return stakes[l/2].Dec
	}
}

// Apply ..
func Apply(shortHand map[common.Address]SlotOrder) Slots {
	eposedSlots := Slots{}
	if len(shortHand) == 0 {
		return eposedSlots
	}
	// Expand
	for staker := range shortHand {
		slotsCount := int64(len(shortHand[staker].SpreadAmong))
		spread := numeric.NewDecFromBigInt(shortHand[staker].Stake).QuoInt64(slotsCount)
		var i int64
		for ; i < slotsCount; i++ {
			eposedSlots = append(eposedSlots, SlotPurchase{staker, spread})
		}
	}
	if len(eposedSlots) < len(shortHand) {
		// WARN Should never happen
	}
	median := median(eposedSlots)

	for i := range eposedSlots {
		eposedSlots[i].Dec = stake(median, eposedSlots[i].Dec)
	}

	return eposedSlots
}
