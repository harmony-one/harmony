package quorum

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

var (
	secretKeyMap  map[shard.BlsPublicKey]bls.SecretKey
	slotList      shard.SlotList
	roster        *votepower.Roster
	stakedVote    Decider
	result        *TallyResult
	harmonyNodes  = 33
	stakedNodes   = 33
	maxAccountGen = int64(98765654323123134)
	accountGen    = rand.New(rand.NewSource(1337))
	maxKeyGen     = int64(98765654323123134)
	keyGen        = rand.New(rand.NewSource(42))
	maxStakeGen   = int64(200)
	stakeGen      = rand.New(rand.NewSource(541))
)

func init() {
	slotList = shard.SlotList{}
	secretKeyMap = make(map[shard.BlsPublicKey]bls.SecretKey)
	for i := 0; i < harmonyNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		newSlot.TotalStake = nil
		slotList = append(slotList, newSlot)
		secretKeyMap[newSlot.BlsPublicKey] = sKey
	}

	for j := 0; j < stakedNodes; j++ {
		newSlot, sKey := generateRandomSlot()
		slotList = append(slotList, newSlot)
		secretKeyMap[newSlot.BlsPublicKey] = sKey
	}

	stakedVote = NewDecider(SuperMajorityStake)
	stakedVote.SetShardIDProvider(func() (uint32, error) { return 0, nil })
	r, err := stakedVote.SetVoters(slotList)
	if err != nil {
		panic("Unable to SetVoters")
	}
	result = r
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

func Test33(t *testing.T) {
	sum := result.ourPercent.Add(result.theirPercent)
	if !sum.Equal(numeric.OneDec()) {
		t.Errorf("Total voting power does not equal 1. Harmony voting power: %s, Staked voting power: %s, Sum: %s",
			result.ourPercent, result.theirPercent, sum)
	}
}

func TestPolicy(t *testing.T) {
	expectedPolicy := SuperMajorityStake
	policy := stakedVote.Policy()
	if expectedPolicy != policy {
		t.Errorf("Expected: %s, Got: %s", expectedPolicy.String(), policy.String())
	}
}

func TestIsQuorumAchieved(t *testing.T) {
	//
}

func TestQuorumThreshold(t *testing.T) {
	expectedThreshold := numeric.NewDec(2).Quo(numeric.NewDec(3))
	quorumThreshold := stakedVote.QuorumThreshold()
	if !expectedThreshold.Equal(quorumThreshold) {
		t.Errorf("Expected: %s, Got: %s", expectedThreshold.String(), quorumThreshold.String())
	}
}

func TestIsRewardThresholdAchieved(t *testing.T) {
	//
}

// ????
func TestShouldSlash(t *testing.T) {
	//
}
