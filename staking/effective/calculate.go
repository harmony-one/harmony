package effective

import (
	"bytes"
	"encoding/json"
	"math/big"
	"sort"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
)

// medium.com/harmony-one/introducing-harmonys-effective-proof-of-stake-epos-2d39b4b8d58
var (
	two         = numeric.NewDecFromBigInt(big.NewInt(2))
	c, _        = numeric.NewDecFromStr("0.15")
	cV2, _      = numeric.NewDecFromStr("0.35")
	onePlusC    = numeric.OneDec().Add(c)
	oneMinusC   = numeric.OneDec().Sub(c)
	onePlusCV2  = numeric.OneDec().Add(cV2)
	oneMinusCV2 = numeric.OneDec().Sub(cV2)
)

// SlotPurchase ..
type SlotPurchase struct {
	Addr      common.Address
	Key       bls.SerializedPublicKey
	RawStake  numeric.Dec
	EPoSStake numeric.Dec
}

// MarshalJSON ..
func (p SlotPurchase) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Addr      string      `json:"slot-owner"`
		Key       string      `json:"bls-public-key"`
		RawStake  numeric.Dec `json:"raw-stake"`
		EPoSStake numeric.Dec `json:"eposed-stake"`
	}{
		common2.MustAddressToBech32(p.Addr),
		p.Key.Hex(),
		p.RawStake,
		p.EPoSStake,
	})
}

// SlotOrder ..
type SlotOrder struct {
	Stake       *big.Int                  `json:"stake"`
	SpreadAmong []bls.SerializedPublicKey `json:"keys-at-auction"`
	Percentage  numeric.Dec               `json:"percentage-of-total-auction-stake"`
}

// Median ..
func Median(stakes []SlotPurchase) numeric.Dec {
	if len(stakes) == 0 {
		return numeric.ZeroDec()
	}

	sort.SliceStable(
		stakes,
		func(i, j int) bool {
			return stakes[i].RawStake.GT(stakes[j].RawStake)
		},
	)
	const isEven = 0
	switch l := len(stakes); l % 2 {
	case isEven:
		left := (l / 2) - 1
		right := l / 2
		return stakes[left].RawStake.Add(stakes[right].RawStake).Quo(two)
	default:
		return stakes[l/2].RawStake
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
		if slotsCount == 0 {
			continue
		}
		spread := numeric.NewDecFromBigInt(staker.slot.Stake).
			QuoInt64(int64(slotsCount))
		for i := 0; i < slotsCount; i++ {
			eposedSlots = append(eposedSlots, SlotPurchase{
				Addr: staker.addr,
				Key:  staker.slot.SpreadAmong[i],
				// NOTE these are same because later the .EPoSStake mutated
				RawStake:  spread,
				EPoSStake: spread,
			})
		}
	}

	sort.SliceStable(
		eposedSlots,
		func(i, j int) bool {
			return eposedSlots[i].RawStake.GT(eposedSlots[j].RawStake)
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
func Apply(shortHand map[common.Address]*SlotOrder, pull int, isExtendedBound bool) (
	numeric.Dec, []SlotPurchase,
) {
	median, picks := Compute(shortHand, pull)
	max := onePlusC.Mul(median)
	min := oneMinusC.Mul(median)
	if isExtendedBound {
		max = onePlusCV2.Mul(median)
		min = oneMinusCV2.Mul(median)
	}
	for i := range picks {
		picks[i].EPoSStake = effectiveStake(min, max, picks[i].RawStake)
	}

	return median, picks
}

func effectiveStake(min, max, actual numeric.Dec) numeric.Dec {
	newMax := numeric.MinDec(max, actual)
	return numeric.MaxDec(newMax, min)
}
