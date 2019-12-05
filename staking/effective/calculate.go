package effective

import (
	"encoding/json"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
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

// Slots ..
type Slots []SlotPurchase

// JSON is a plain JSON dump
func (s Slots) JSON() string {
	type t struct {
		Address string `json:"slot-owner"`
		Key     string `json:"bls-public-key"`
		Stake   string `json:"actual-stake"`
	}
	type v struct {
		Slots []t `json:"slots"`
	}
	data := v{}
	for i := range s {
		newData := t{
			common2.MustAddressToBech32(s[i].Address),
			s[i].BlsPublicKey.Hex(),
			s[i].Dec.String(),
		}
		data.Slots = append(data.Slots, newData)
	}
	b, _ := json.Marshal(data)
	return string(b)
}

func median(stakes []SlotPurchase) numeric.Dec {

	if len(stakes) == 0 {
		utils.Logger().Error().Int("non-zero", len(stakes)).
			Msg("Input to median has len 0, check caller")
	}

	sort.SliceStable(
		stakes,
		func(i, j int) bool { return stakes[i].Dec.LT(stakes[j].Dec) },
	)
	const isEven = 0
	switch l := len(stakes); l % 2 {
	case isEven:
		left := (l / 2) - 1
		right := (l / 2)
		utils.Logger().Info().Int("left", left).Int("right", right)
		return stakes[left].Dec.Add(stakes[right].Dec).Quo(two)
	default:
		utils.Logger().Info().Int("median index", l/2)
		return stakes[l/2].Dec
	}
}

// Apply ..
func Apply(shortHand map[common.Address]SlotOrder, pull int) Slots {
	eposedSlots := Slots{}
	if len(shortHand) == 0 {
		return eposedSlots
	}
	// Expand
	for staker := range shortHand {
		slotsCount := len(shortHand[staker].SpreadAmong)
		spread := numeric.NewDecFromBigInt(shortHand[staker].Stake).
			QuoInt64(int64(slotsCount))
		for i := 0; i < slotsCount; i++ {
			eposedSlots = append(eposedSlots, SlotPurchase{
				staker,
				shortHand[staker].SpreadAmong[i],
				spread,
			})
		}
	}
	if len(eposedSlots) < len(shortHand) {
		// WARN Should never happen
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
		return Slots{}
	}

	median := median(picks)

	for i := range picks {
		picks[i].Dec = effectiveStake(median, picks[i].Dec)
	}

	return picks
}
