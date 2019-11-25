package votepower

import (
  "fmt"
  "math/big"
  "math/rand"
  "testing"

  "github.com/ethereum/go-ethereum/common"
  "github.com/harmony-one/bls/ffi/go/bls"
  "github.com/harmony-one/harmony/numeric"
  "github.com/harmony-one/harmony/shard"
)

var (
  slotList         shard.SlotList
  totalStake       numeric.Dec
  harmonyNodes     = 10
  stakedNodes      = 10
  maxAccountGen    = int64(98765654323123134)
	accountGen       = rand.New(rand.NewSource(1337))
	maxKeyGen        = int64(98765654323123134)
	keyGen           = rand.New(rand.NewSource(42))
	maxStakeGen      = int64(200)
	stakeGen         = rand.New(rand.NewSource(541))
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

  fmt.Println(slotList)
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
  for i, slot := range slotList {
    newMember = stakedVoter{
      IsActive:         true,
      IsHarmonyNode:    false,
      EarningAccount:   slot.EcdsaAddress,
      EffectivePercent: numeric.ZeroDec(),
      RawStake:         numeric.ZeroDec(),
    }
    // Not Harmony node
    if newMember.TotalStake != nil {
      member.RawStake = *slot.TotalStake
      member.EffectivePercent = slot.TotalStake.Quo(roster.RawStakedTotal).Mul(StakersShare)
      expectedRoster.TheirVotingPowerTotalPercentage = expectedRoster.TheirVotingPowerTotalPercentage.Add(member.EffectivePercent)
    } else {
      // Harmony node
      member.IsHarmonyNode = true
      member.RawStake = numeric.ZeroDec()
      member.EffectivePercent = HarmonysShare.Quo(numeric.NewDec(expectedRoster.HmySlotCount))
      expectedRoster.OurVotingPowerTotalPercentage = expectedRoster.OurVotingPowerTotalPercentage.Add(member.EffectivePercent)
    }
  }

  computedRoster := Compute(slotList)
  // Check that voting percents sum to 100
  if !computedRoster.OurVotingPowerTotalPercentage.Add(computedRoster.TheirVotingPowerTotalPercentage).Equal(numeric.OneDec()) {
    t.Errorf("Total voting power does not equal 1. Harmony voting power: %s, Staked voting power: %s",
      computedRoster.OurVotingPowerTotalPercentage, computedRoster.TheirVotingPowerTotalPercentage)
  }
}

func compareRosters(a, b Roster) bool {

}
