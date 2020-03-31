package effective

import (
	"bytes"
	"encoding/json"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	common2 "github.com/harmony-one/harmony/internal/common"
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
	Addr  common.Address     `json:"slot-owner"`
	Key   shard.BLSPublicKey `json:"bls-public-key"`
	Stake numeric.Dec        `json:"eposed-stake"`
}

// MarshalJSON ..
func (p SlotPurchase) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Addr  string      `json:"slot-owner"`
		Key   string      `json:"bls-public-key"`
		Stake numeric.Dec `json:"eposed-stake"`
	}{
		common2.MustAddressToBech32(p.Addr),
		p.Key.Hex(),
		p.Stake,
	})
}

// SlotOrder ..
type SlotOrder struct {
	Stake       *big.Int             `json:"stake"`
	SpreadAmong []shard.BLSPublicKey `json:"keys-at-auction"`
	Percentage  numeric.Dec          `json:"percentage-of-total-auction-stake"`
}

// Median ..
func Median(stakes []SlotPurchase) numeric.Dec {
	if len(stakes) == 0 {
		return numeric.ZeroDec()
	}

	sort.SliceStable(
		stakes,
		func(i, j int) bool {
			return stakes[i].Stake.GT(stakes[j].Stake)
		},
	)
	const isEven = 0
	switch l := len(stakes); l % 2 {
	case isEven:
		left := (l / 2) - 1
		right := l / 2
		return stakes[left].Stake.Add(stakes[right].Stake).Quo(two)
	default:
		return stakes[l/2].Stake
	}
}

// Compute ..
func Compute(
	shortHand map[common.Address]*SlotOrder, pull int,
) (numeric.Dec, []SlotPurchase) {
	eposedSlots := []SlotPurchase{}
	if len(shortHand) == 0 {
		return numeric.ZeroDec(), eposedSlots
	}

	type t struct {
		addr common.Address
		slot *SlotOrder
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
		func(i, j int) bool {
			return eposedSlots[i].Stake.GT(eposedSlots[j].Stake)
		},
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
func Apply(shortHand map[common.Address]*SlotOrder, pull int) (
	numeric.Dec, []SlotPurchase,
) {
	median, picks := Compute(shortHand, pull)
	for i := range picks {
		picks[i].Stake = effectiveStake(median, picks[i].Stake)
	}

	return median, picks
}
