package quorum

import (
	"testing"

	harmony_bls "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/shard"
	"github.com/stretchr/testify/assert"
)

func TestPhaseStrings(t *testing.T) {
	phases := []Phase{
		Prepare,
		Commit,
		ViewChange,
	}

	expectations := make(map[Phase]string)
	expectations[Prepare] = "Prepare"
	expectations[Commit] = "Commit"
	expectations[ViewChange] = "viewChange"

	for _, phase := range phases {
		expected := expectations[phase]
		assert.Equal(t, expected, phase.String())
	}
}

func TestPolicyStrings(t *testing.T) {
	policies := []Policy{
		SuperMajorityVote,
		SuperMajorityStake,
	}

	expectations := make(map[Policy]string)
	expectations[SuperMajorityVote] = "SuperMajorityVote"
	expectations[SuperMajorityStake] = "SuperMajorityStake"

	for _, policy := range policies {
		expected := expectations[policy]
		assert.Equal(t, expected, policy.String())
	}
}

func TestAddingQuoromParticipants(t *testing.T) {
	decider := NewDecider(SuperMajorityVote, shard.BeaconChainShardID)

	assert.Equal(t, int64(0), decider.ParticipantsCount())

	blsKeys := []harmony_bls.PublicKeyWrapper{}
	keyCount := int64(5)
	for i := int64(0); i < keyCount; i++ {
		blsKey := harmony_bls.RandPrivateKey()
		wrapper := harmony_bls.PublicKeyWrapper{Object: blsKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)
		blsKeys = append(blsKeys, wrapper)
	}

	decider.UpdateParticipants(blsKeys)
	assert.Equal(t, keyCount, decider.ParticipantsCount())
}
