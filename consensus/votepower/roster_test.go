package votepower

import (
	"encoding/json"
	"math/big"
	"math/rand"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

var (
	slotList      shard.SlotList
	totalStake    numeric.Dec
	harmonyNodes  = 10
	stakedNodes   = 10
	maxAccountGen = int64(98765654323123134)
	accountGen    = rand.New(rand.NewSource(1337))
	maxKeyGen     = int64(98765654323123134)
	keyGen        = rand.New(rand.NewSource(42))
	maxStakeGen   = int64(200)
	stakeGen      = rand.New(rand.NewSource(541))
)

func init() {
	for i := 0; i < harmonyNodes; i++ {
		newSlot := generateRandomSlot()
		newSlot.TotalStake = nil
		slotList = append(slotList, newSlot)
	}

	totalStake = numeric.ZeroDec()
	for j := 0; j < stakedNodes; j++ {
		newSlot := generateRandomSlot()
		totalStake = totalStake.Add(*newSlot.TotalStake)
		slotList = append(slotList, newSlot)
	}
}

func generateRandomSlot() shard.Slot {
	addr := common.Address{}
	addr.SetBytes(big.NewInt(int64(accountGen.Int63n(maxAccountGen))).Bytes())
	secretKey := bls.SecretKey{}
	secretKey.Deserialize(big.NewInt(int64(keyGen.Int63n(maxKeyGen))).Bytes())
	key := shard.BlsPublicKey{}
	key.FromLibBLSPublicKey(secretKey.GetPublicKey())
	stake := numeric.NewDecFromBigInt(big.NewInt(int64(stakeGen.Int63n(maxStakeGen))))
	return shard.Slot{addr, key, &stake}
}

func TestCompute(t *testing.T) {
	expectedRoster := NewRoster()
	// Parameterized
	expectedRoster.HmySlotCount = int64(harmonyNodes)
	// Calculated when generated
	expectedRoster.RawStakedTotal = totalStake
	for _, slot := range slotList {
		newMember := stakedVoter{
			IsActive:         true,
			IsHarmonyNode:    false,
			EarningAccount:   slot.EcdsaAddress,
			EffectivePercent: numeric.ZeroDec(),
			RawStake:         numeric.ZeroDec(),
		}
		// Not Harmony node
		if slot.TotalStake != nil {
			newMember.RawStake = *slot.TotalStake
			newMember.EffectivePercent = slot.TotalStake.Quo(expectedRoster.RawStakedTotal).Mul(StakersShare)
			expectedRoster.TheirVotingPowerTotalPercentage = expectedRoster.TheirVotingPowerTotalPercentage.Add(newMember.EffectivePercent)
		} else {
			// Harmony node
			newMember.IsHarmonyNode = true
			newMember.RawStake = numeric.ZeroDec()
			newMember.EffectivePercent = HarmonysShare.Quo(numeric.NewDec(expectedRoster.HmySlotCount))
			expectedRoster.OurVotingPowerTotalPercentage = expectedRoster.OurVotingPowerTotalPercentage.Add(newMember.EffectivePercent)
		}
		expectedRoster.Voters[slot.BlsPublicKey] = newMember
	}

	computedRoster, err := Compute(slotList)
	if err != nil {
		t.Error("Computed Roster failed on vote summation to one")
	}

	if !compareRosters(expectedRoster, computedRoster, t) {
		t.Errorf("Compute Roster mismatch with expected Roster")
	}
	// Check that voting percents sum to 100
	if !computedRoster.OurVotingPowerTotalPercentage.Add(computedRoster.TheirVotingPowerTotalPercentage).Equal(numeric.OneDec()) {
		t.Errorf("Total voting power does not equal 1. Harmony voting power: %s, Staked voting power: %s",
			computedRoster.OurVotingPowerTotalPercentage, computedRoster.TheirVotingPowerTotalPercentage)
	}
}

func compareRosters(a, b *Roster, t *testing.T) bool {
	// Compare stakedVoter maps
	voterMatch := true
	for k, voter := range a.Voters {
		if other, exists := b.Voters[k]; exists {
			if !compareStakedVoter(voter, other) {
				t.Errorf("Expecting %s\n Got %s", voter.formatString(), other.formatString())
				voterMatch = false
			}
		} else {
			t.Errorf("Computed roster missing %s", voter.formatString())
			voterMatch = false
		}
	}
	return a.OurVotingPowerTotalPercentage.Equal(b.OurVotingPowerTotalPercentage) &&
		a.TheirVotingPowerTotalPercentage.Equal(b.TheirVotingPowerTotalPercentage) &&
		a.RawStakedTotal.Equal(b.RawStakedTotal) &&
		a.HmySlotCount == b.HmySlotCount && voterMatch
}

func compareStakedVoter(a, b stakedVoter) bool {
	return a.IsActive == b.IsActive &&
		a.IsHarmonyNode == b.IsHarmonyNode &&
		a.EarningAccount == b.EarningAccount &&
		a.EffectivePercent.Equal(b.EffectivePercent) &&
		a.RawStake.Equal(b.RawStake)
}

func (s *stakedVoter) formatString() string {
	type t struct {
		IsActive         string `json:"active"`
		IsHarmony        string `json:"harmony-node"`
		EarningAccount   string `json:"one-address"`
		EffectivePercent string `json:"effective-percent"`
		RawStake         string `json:"raw-stake"`
	}
	data := t{
		strconv.FormatBool(s.IsActive),
		strconv.FormatBool(s.IsHarmonyNode),
		s.EarningAccount.String(),
		s.EffectivePercent.String(),
		s.RawStake.String(),
	}
	output, _ := json.Marshal(data)
	return string(output)
}
