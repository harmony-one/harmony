package quorum

import (
	"math/big"
	"math/rand"
	"strconv"
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"

	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
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

type secretKeyMap map[bls.SerializedPublicKey]bls_core.SecretKey

func init() {
	basicDecider = NewDecider(SuperMajorityStake, shard.BeaconChainShardID)
	shard.Schedule = shardingconfig.LocalnetSchedule
}

func generateRandomSlot() (shard.Slot, bls_core.SecretKey) {
	addr := common.Address{}
	addr.SetBytes(big.NewInt(int64(accountGen.Int63n(maxAccountGen))).Bytes())
	secretKey := bls_core.SecretKey{}
	secretKey.Deserialize(big.NewInt(int64(keyGen.Int63n(maxKeyGen))).Bytes())
	key := bls.SerializedPublicKey{}
	key.FromLibBLSPublicKey(secretKey.GetPublicKey())
	stake := numeric.NewDecFromBigInt(big.NewInt(int64(stakeGen.Int63n(maxStakeGen))))
	return shard.Slot{EcdsaAddress: addr, BLSPublicKey: key, EffectiveStake: &stake}, secretKey
}

// 50 Harmony Nodes, 50 Staked Nodes
func setupBaseCase() (Decider, *TallyResult, shard.SlotList, map[string]secretKeyMap) {
	slotList := shard.SlotList{}
	sKeys := map[string]secretKeyMap{}
	sKeys[hmy] = secretKeyMap{}
	sKeys[reg] = secretKeyMap{}
	pubKeys := []bls.PublicKeyWrapper{}

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 50 {
			newSlot.EffectiveStake = nil
			sKeys[hmy][newSlot.BLSPublicKey] = sKey
		} else {
			sKeys[reg][newSlot.BLSPublicKey] = sKey
		}
		slotList = append(slotList, newSlot)
		wrapper := bls.PublicKeyWrapper{Object: sKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)
		pubKeys = append(pubKeys, wrapper)
	}

	decider := NewDecider(SuperMajorityStake, shard.BeaconChainShardID)
	decider.UpdateParticipants(pubKeys, []bls.PublicKeyWrapper{})
	tally, err := decider.SetVoters(&shard.Committee{
		ShardID: shard.BeaconChainShardID, Slots: slotList,
	}, big.NewInt(3))
	if err != nil {
		panic("Unable to SetVoters for Base Case")
	}
	return decider, tally, slotList, sKeys
}

// 33 Harmony Nodes, 67 Staked Nodes
func setupEdgeCase() (Decider, *TallyResult, shard.SlotList, secretKeyMap) {
	slotList := shard.SlotList{}
	sKeys := secretKeyMap{}
	pubKeys := []bls.PublicKeyWrapper{}

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 33 {
			newSlot.EffectiveStake = nil
			sKeys[newSlot.BLSPublicKey] = sKey
		}
		slotList = append(slotList, newSlot)
		wrapper := bls.PublicKeyWrapper{Object: sKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)
		pubKeys = append(pubKeys, wrapper)
	}

	decider := NewDecider(SuperMajorityStake, shard.BeaconChainShardID)
	decider.UpdateParticipants(pubKeys, []bls.PublicKeyWrapper{})
	tally, err := decider.SetVoters(&shard.Committee{
		ShardID: shard.BeaconChainShardID, Slots: slotList,
	}, big.NewInt(3))
	if err != nil {
		panic("Unable to SetVoters for Edge Case")
	}
	return decider, tally, slotList, sKeys
}

func sign(d Decider, k secretKeyMap, p Phase) {
	for k, v := range k {
		sig := v.Sign(msg)
		// TODO Make upstream test provide meaningful test values
		d.AddNewVote(p, []*bls.PublicKeyWrapper{{Bytes: k}}, sig, common.Hash{}, 0, 0)
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
	rewarded := stakedVote.IsAllSigsCollected()
	if rewarded {
		t.Errorf("[IsAllSigsCollected] Got: %s, Expected: false (All Staker nodes = 32%%)",
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
	rewarded = stakedVote.IsAllSigsCollected()
	if !rewarded {
		t.Errorf("[IsAllSigsCollected] Got: %s, Expected: true (All nodes = 100%%)",
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
	rewarded := stakedVote.IsAllSigsCollected()
	if rewarded {
		t.Errorf("[IsAllSigsCollected] Got: %s, Expected: false (All Harmony nodes = 68%%)",
			strconv.FormatBool(rewarded))
	}
}
