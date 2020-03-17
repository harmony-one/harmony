package votepower

import (
	"math/big"
	"math/rand"
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
		newSlot.EffectiveStake = nil
		slotList = append(slotList, newSlot)
	}

	totalStake = numeric.ZeroDec()
	for j := 0; j < stakedNodes; j++ {
		newSlot := generateRandomSlot()
		totalStake = totalStake.Add(*newSlot.EffectiveStake)
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
	expectedRoster := NewRoster(shard.BeaconChainShardID)
	// Calculated when generated
	expectedRoster.TotalEffectiveStake = totalStake
	expectedRoster.HMYSlotCount = int64(harmonyNodes)

	asDecHMYSlotCount := numeric.NewDec(expectedRoster.HMYSlotCount)
	ourPercentage := numeric.ZeroDec()
	theirPercentage := numeric.ZeroDec()

	staked := slotList
	for i := range staked {
		member := AccommodateHarmonyVote{
			PureStakedVote: PureStakedVote{
				EarningAccount: staked[i].EcdsaAddress,
				Identity:       staked[i].BlsPublicKey,
				GroupPercent:   numeric.ZeroDec(),
				EffectiveStake: numeric.ZeroDec(),
			},
			OverallPercent: numeric.ZeroDec(),
			IsHarmonyNode:  false,
		}

		// Real Staker
		if e := staked[i].EffectiveStake; e != nil {
			member.EffectiveStake = member.EffectiveStake.Add(*e)
			member.GroupPercent = e.Quo(expectedRoster.TotalEffectiveStake)
			member.OverallPercent = member.GroupPercent.Mul(StakersShare)
			theirPercentage = theirPercentage.Add(member.OverallPercent)
		} else { // Our node
			member.IsHarmonyNode = true
			member.OverallPercent = HarmonysShare.Quo(asDecHMYSlotCount)
			member.GroupPercent = member.OverallPercent.Quo(HarmonysShare)
			ourPercentage = ourPercentage.Add(member.OverallPercent)
		}

		expectedRoster.Voters[staked[i].BlsPublicKey] = &member
	}

	expectedRoster.OurVotingPowerTotalPercentage = ourPercentage
	expectedRoster.TheirVotingPowerTotalPercentage = theirPercentage

	computedRoster, err := Compute(&shard.Committee{
		shard.BeaconChainShardID, slotList,
	})
	if err != nil {
		t.Error("Computed Roster failed on vote summation to one")
	}

	if !compareRosters(expectedRoster, computedRoster, t) {
		t.Errorf("Compute Roster mismatch with expected Roster")
	}
	// Check that voting percents sum to 100
	if !computedRoster.OurVotingPowerTotalPercentage.Add(
		computedRoster.TheirVotingPowerTotalPercentage,
	).Equal(numeric.OneDec()) {
		t.Errorf(
			"Total voting power does not equal 1. Harmony voting power: %s, Staked voting power: %s",
			computedRoster.OurVotingPowerTotalPercentage,
			computedRoster.TheirVotingPowerTotalPercentage,
		)
	}
}

func compareRosters(a, b *Roster, t *testing.T) bool {
	voterMatch := true
	for k, voter := range a.Voters {
		if other, exists := b.Voters[k]; exists {
			if !compareStakedVoter(voter, other) {
				t.Error("voter slot not match")
				voterMatch = false
			}
		} else {
			t.Error("computed roster missing")
			voterMatch = false
		}
	}
	return a.OurVotingPowerTotalPercentage.Equal(b.OurVotingPowerTotalPercentage) &&
		a.TheirVotingPowerTotalPercentage.Equal(b.TheirVotingPowerTotalPercentage) &&
		a.TotalEffectiveStake.Equal(b.TotalEffectiveStake) &&
		a.HMYSlotCount == b.HMYSlotCount && voterMatch
}

func compareStakedVoter(a, b *AccommodateHarmonyVote) bool {
	return a.IsHarmonyNode == b.IsHarmonyNode &&
		a.EarningAccount == b.EarningAccount &&
		a.OverallPercent.Equal(b.OverallPercent) &&
		a.EffectiveStake.Equal(b.EffectiveStake)
}
