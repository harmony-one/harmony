package quorum

import (
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
	quorumNodes   = 100
	msg           = "Testing"
	hmy           = "Harmony"
	reg           = "Stakers"
	basicDecider  Decider
	maxAccountGen = int64(98765654323123134)
	accountGen    = rand.New(rand.NewSource(1337))
	maxKeyGen     = int64(98765654323123134)
	keyGen        = rand.New(rand.NewSource(42))
	maxStakeGen   = int64(200)
	stakeGen      = rand.New(rand.NewSource(541))
)

type secretKeyMap map[shard.BlsPublicKey]bls.SecretKey

func init() {
	basicDecider = NewDecider(SuperMajorityStake)
}

func generateRandomSlot() (shard.Slot, bls.SecretKey) {
	addr := common.Address{}
	addr.SetBytes(big.NewInt(int64(accountGen.Int63n(maxAccountGen))).Bytes())
	secretKey := bls.SecretKey{}
	secretKey.Deserialize(big.NewInt(int64(keyGen.Int63n(maxKeyGen))).Bytes())
	key := shard.BlsPublicKey{}
	key.FromLibBLSPublicKey(secretKey.GetPublicKey())
	stake := numeric.NewDecFromBigInt(big.NewInt(int64(stakeGen.Int63n(maxStakeGen))))
	return shard.Slot{addr, key, &stake}, secretKey
}

// 50 Harmony Nodes, 50 Staked Nodes
func setupBaseCase() (Decider, *TallyResult, shard.SlotList, map[string]secretKeyMap) {
	slotList := shard.SlotList{}
	sKeys := make(map[string]secretKeyMap)
	sKeys[hmy] = make(secretKeyMap)
	sKeys[reg] = make(secretKeyMap)
	pubKeys := []*bls.PublicKey{}

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 50 {
			newSlot.EffectiveStake = nil
			sKeys[hmy][newSlot.BlsPublicKey] = sKey
		} else {
			sKeys[reg][newSlot.BlsPublicKey] = sKey
		}
		slotList = append(slotList, newSlot)
		pubKeys = append(pubKeys, sKey.GetPublicKey())
	}

	decider := NewDecider(SuperMajorityStake)
	decider.SetShardIDProvider(func() (uint32, error) { return 0, nil })
	decider.UpdateParticipants(pubKeys)
	tally, err := decider.SetVoters(slotList)
	if err != nil {
		panic("Unable to SetVoters for Base Case")
	}
	return decider, tally, slotList, sKeys
}

// 33 Harmony Nodes, 67 Staked Nodes
func setupEdgeCase() (Decider, *TallyResult, shard.SlotList, secretKeyMap) {
	slotList := shard.SlotList{}
	sKeys := make(secretKeyMap)
	pubKeys := []*bls.PublicKey{}

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 33 {
			newSlot.EffectiveStake = nil
			sKeys[newSlot.BlsPublicKey] = sKey
		}
		slotList = append(slotList, newSlot)
		pubKeys = append(pubKeys, sKey.GetPublicKey())
	}

	decider := NewDecider(SuperMajorityStake)
	decider.SetShardIDProvider(func() (uint32, error) { return 0, nil })
	decider.UpdateParticipants(pubKeys)
	tally, err := decider.SetVoters(slotList)
	if err != nil {
		panic("Unable to SetVoters for Edge Case")
	}
	return decider, tally, slotList, sKeys
}

func sign(d Decider, k secretKeyMap, p Phase) {
	for _, v := range k {
		pubKey := v.GetPublicKey()
		sig := v.Sign(msg)
		// TODO Make upstream test provide meaningful test values
		d.SubmitVote(p, pubKey, sig, nil)
	}
}

func TestPolicy(t *testing.T) {
	expectedPolicy := SuperMajorityStake
	policy := basicDecider.Policy()
	if expectedPolicy != policy {
		t.Errorf("Expected: %s, Got: %s", expectedPolicy.String(), policy.String())
	}
}

func TestQuorumThreshold(t *testing.T) {
	expectedThreshold := numeric.NewDec(2).Quo(numeric.NewDec(3))
	quorumThreshold := basicDecider.QuorumThreshold()
	if !expectedThreshold.Equal(quorumThreshold) {
		t.Errorf("Expected: %s, Got: %s", expectedThreshold.String(), quorumThreshold.String())
	}
}

func TestEvenNodes(t *testing.T) {
	stakedVote, result, _, sKeys := setupBaseCase()
	// Check HarmonyPercent + StakePercent == 1
	sum := result.ourPercent.Add(result.theirPercent)
	if !sum.Equal(numeric.OneDec()) {
		t.Errorf("Total voting power does not equal 1. Harmony voting power: %s, Staked voting power: %s, Sum: %s",
			result.ourPercent, result.theirPercent, sum)
		t.FailNow()
	}
	// Sign all Staker nodes
	// Prepare
	sign(stakedVote, sKeys[reg], Prepare)
	achieved := stakedVote.IsQuorumAchieved(Prepare)
	if achieved {
		t.Errorf("[IsQuorumAchieved] Phase: %s, QuorumAchieved: %s, Expected: false (All Staker nodes = 32%%)",
			Prepare, strconv.FormatBool(achieved))
	}
	// Commit
	sign(stakedVote, sKeys[reg], Commit)
	achieved = stakedVote.IsQuorumAchieved(Commit)
	if achieved {
		t.Errorf("[IsQuorumAchieved] Phase: %s, QuorumAchieved: %s, Expected: false (All Staker nodes = 32%%)",
			Commit, strconv.FormatBool(achieved))
	}
	// ViewChange
	sign(stakedVote, sKeys[reg], ViewChange)
	achieved = stakedVote.IsQuorumAchieved(ViewChange)
	if achieved {
		t.Errorf("[IsQuorumAchieved] Phase: %s, Got: %s, Expected: false (All Staker nodes = 32%%)",
			ViewChange, strconv.FormatBool(achieved))
	}
	// RewardThreshold
	rewarded := stakedVote.IsRewardThresholdAchieved()
	if rewarded {
		t.Errorf("[IsRewardThresholdAchieved] Got: %s, Expected: false (All Staker nodes = 32%%)",
			strconv.FormatBool(rewarded))
	}

	// Sign all Harmony Nodes
	sign(stakedVote, sKeys[hmy], Prepare)
	achieved = stakedVote.IsQuorumAchieved(Prepare)
	if !achieved {
		t.Errorf("[IsQuorumAchieved] Phase: %s, QuorumAchieved: %s, Expected: true (All nodes = 100%%)",
			Prepare, strconv.FormatBool(achieved))
	}
	// Commit
	sign(stakedVote, sKeys[hmy], Commit)
	achieved = stakedVote.IsQuorumAchieved(Commit)
	if !achieved {
		t.Errorf("[IsQuorumAchieved] Phase: %s, QuorumAchieved: %s, Expected: true (All nodes = 100%%)",
			Commit, strconv.FormatBool(achieved))
	}
	// ViewChange
	sign(stakedVote, sKeys[hmy], ViewChange)
	achieved = stakedVote.IsQuorumAchieved(ViewChange)
	if !achieved {
		t.Errorf("[IsQuorumAchieved] Phase: %s, Got: %s, Expected: true (All nodes = 100%%)",
			ViewChange, strconv.FormatBool(achieved))
	}
	// RewardThreshold
	rewarded = stakedVote.IsRewardThresholdAchieved()
	if !rewarded {
		t.Errorf("[IsRewardThresholdAchieved] Got: %s, Expected: true (All nodes = 100%%)",
			strconv.FormatBool(rewarded))
	}
}

func Test33HarmonyNodes(t *testing.T) {
	stakedVote, result, _, sKeys := setupEdgeCase()
	// Check HarmonyPercent + StakePercent == 1
	sum := result.ourPercent.Add(result.theirPercent)
	if !sum.Equal(numeric.OneDec()) {
		t.Errorf("Total voting power does not equal 1. Harmony voting power: %s, Staked voting power: %s, Sum: %s",
			result.ourPercent, result.theirPercent, sum)
		t.FailNow()
	}
	// Sign all Harmony Nodes, 0 Staker Nodes
	// Prepare
	sign(stakedVote, sKeys, Prepare)
	achieved := stakedVote.IsQuorumAchieved(Prepare)
	if !achieved {
		t.Errorf("[IsQuorumAchieved] Phase: %s, QuorumAchieved: %s, Expected: true (All Harmony nodes = 68%%)",
			Prepare, strconv.FormatBool(achieved))
	}
	// Commit
	sign(stakedVote, sKeys, Commit)
	achieved = stakedVote.IsQuorumAchieved(Commit)
	if !achieved {
		t.Errorf("[IsQuorumAchieved] Phase: %s, QuorumAchieved: %s, Expected: true (All Harmony nodes = 68%%)",
			Commit, strconv.FormatBool(achieved))
	}
	// ViewChange
	sign(stakedVote, sKeys, ViewChange)
	achieved = stakedVote.IsQuorumAchieved(ViewChange)
	if !achieved {
		t.Errorf("[IsQuorumAchieved] Phase: %s, Got: %s, Expected: true (All Harmony nodes = 68%%)",
			ViewChange, strconv.FormatBool(achieved))
	}
	// RewardThreshold
	rewarded := stakedVote.IsRewardThresholdAchieved()
	if rewarded {
		t.Errorf("[IsRewardThresholdAchieved] Got: %s, Expected: false (All Harmony nodes = 68%%)",
			strconv.FormatBool(rewarded))
	}
}
