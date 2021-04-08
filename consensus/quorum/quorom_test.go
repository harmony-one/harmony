package quorum

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
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

	blsKeys := []bls.PublicKey{}
	keyCount := int64(5)
	for i := int64(0); i < keyCount; i++ {
		secretKey := bls.RandSecretKey()
		blsKeys = append(blsKeys, secretKey.PublicKey())
	}

	decider.UpdateParticipants(blsKeys)
	assert.Equal(t, keyCount, decider.ParticipantsCount())
}

func TestSubmitVote(test *testing.T) {
	blockHash := [32]byte{}
	copy(blockHash[:], []byte("random"))
	blockNum := uint64(1000)
	viewID := uint64(2)

	decider := NewDecider(
		SuperMajorityStake, shard.BeaconChainShardID,
	)

	message := []byte("test string")
	blsPriKey1 := bls.RandSecretKey()
	pubKeyWrapper1 := blsPriKey1.PublicKey()

	blsPriKey2 := bls.RandSecretKey()
	pubKeyWrapper2 := blsPriKey2.PublicKey()

	decider.UpdateParticipants([]bls.PublicKey{pubKeyWrapper1, pubKeyWrapper2})

	if _, err := decider.submitVote(
		Prepare,
		[]bls.SerializedPublicKey{pubKeyWrapper1.Serialized()},
		blsPriKey1.Sign(message),
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID,
	); err != nil {
		test.Log(err)
	}

	if _, err := decider.submitVote(
		Prepare,
		[]bls.SerializedPublicKey{pubKeyWrapper2.Serialized()},
		blsPriKey2.Sign(message),
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID,
	); err != nil {
		test.Log(err)
	}
	if decider.SignersCount(Prepare) != 2 {
		test.Fatal("submitVote failed")
	}

	aggSig := bls.AggreagateSignatures(
		[]bls.Signature{
			blsPriKey1.Sign(message),
			blsPriKey2.Sign(message)},
	)
	if decider.AggregateVotes(Prepare).ToHex() != aggSig.ToHex() {
		test.Fatal("AggregateVotes failed")
	}
}

func TestSubmitVoteAggregateSig(test *testing.T) {
	blockHash := [32]byte{}
	copy(blockHash[:], []byte("random"))
	blockNum := uint64(1000)
	viewID := uint64(2)

	decider := NewDecider(
		SuperMajorityStake, shard.BeaconChainShardID,
	)

	blsPriKey1 := bls.RandSecretKey()
	pubKeyWrapper1 := blsPriKey1.PublicKey()
	blsPriKey2 := bls.RandSecretKey()
	pubKeyWrapper2 := blsPriKey2.PublicKey()
	blsPriKey3 := bls.RandSecretKey()
	pubKeyWrapper3 := blsPriKey3.PublicKey()
	decider.UpdateParticipants([]bls.PublicKey{pubKeyWrapper1, pubKeyWrapper2})

	decider.submitVote(
		Prepare,
		[]bls.SerializedPublicKey{pubKeyWrapper1.Serialized()},
		blsPriKey1.Sign(blockHash[:]),
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID,
	)

	signatures := []bls.Signature{}
	for _, priKey := range []bls.SecretKey{blsPriKey2, blsPriKey3} {
		if s := priKey.Sign(blockHash[:]); s != nil {
			signatures = append(signatures, s)
		}
	}
	aggSig := bls.AggreagateSignatures(signatures)
	if _, err := decider.submitVote(
		Prepare,
		[]bls.SerializedPublicKey{pubKeyWrapper2.Serialized(), pubKeyWrapper3.Serialized()},
		aggSig,
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID,
	); err != nil {
		test.Log(err)
	}

	if decider.SignersCount(Prepare) != 3 {
		test.Fatal("submitVote failed")
	}

	signatures = append(signatures, blsPriKey1.Sign(blockHash[:]))
	aggSig = bls.AggreagateSignatures(signatures)
	if decider.AggregateVotes(Prepare).ToHex() != aggSig.ToHex() {
		test.Fatal("AggregateVotes failed")
	}

	if _, err := decider.submitVote(
		Prepare,
		[]bls.SerializedPublicKey{pubKeyWrapper2.Serialized()},
		aggSig,
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID,
	); err == nil {
		test.Fatal("Expect error for duplicate votes from the same key")
	}
}

func TestAddNewVote(test *testing.T) {
	shard.Schedule = shardingconfig.LocalnetSchedule
	blockHash := [32]byte{}
	copy(blockHash[:], []byte("random"))
	blockNum := uint64(1000)
	viewID := uint64(2)

	decider := NewDecider(
		SuperMajorityStake, shard.BeaconChainShardID,
	)

	slotList := shard.SlotList{}
	sKeys := []bls.SecretKey{}
	pubKeys := []bls.PublicKey{}

	quorumNodes := 10

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 3 {
			newSlot.EffectiveStake = nil
		}
		sKeys = append(sKeys, sKey)
		slotList = append(slotList, newSlot)
		wrapper := sKey.PublicKey()
		pubKeys = append(pubKeys, wrapper)
	}

	decider.UpdateParticipants(pubKeys)
	decider.SetVoters(&shard.Committee{
		shard.BeaconChainShardID, slotList,
	}, big.NewInt(3))

	signatures := []bls.Signature{}
	for _, priKey := range []bls.SecretKey{sKeys[0], sKeys[1], sKeys[2]} {
		if s := priKey.Sign(blockHash[:]); s != nil {
			signatures = append(signatures, s)
		}
	}
	aggSig := bls.AggreagateSignatures(signatures)

	// aggregate sig from all of 3 harmony nodes
	decider.AddNewVote(Prepare,
		[]bls.PublicKey{pubKeys[0], pubKeys[1], pubKeys[2]},
		aggSig,
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID)

	if !decider.IsQuorumAchieved(Prepare) {
		test.Error("quorum should have been achieved with harmony nodes")
	}
	if decider.SignersCount(Prepare) != 3 {
		test.Errorf("signers are incorrect for harmony nodes signing with aggregate sig: have %d, expect %d", decider.SignersCount(Prepare), 3)
	}

	decider.ResetPrepareAndCommitVotes()

	// aggregate sig from 3 external nodes, expect error
	signatures = []bls.Signature{}
	for _, priKey := range []bls.SecretKey{sKeys[3], sKeys[4], sKeys[5]} {
		if s := priKey.Sign(blockHash[:]); s != nil {
			signatures = append(signatures, s)

		}
	}
	aggSig = bls.AggreagateSignatures(signatures)

	_, err := decider.AddNewVote(Prepare,
		[]bls.PublicKey{pubKeys[3], pubKeys[4], pubKeys[5]},
		aggSig,
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID)

	if err == nil {
		test.Error("Should have error due to aggregate sig from multiple accounts")
	}
	if decider.IsQuorumAchieved(Prepare) {
		test.Fatal("quorum shouldn't have been achieved with external nodes")
	}
	if decider.SignersCount(Prepare) != 0 {
		test.Errorf("signers are incorrect for harmony nodes signing with aggregate sig: have %d, expect %d", decider.SignersCount(Prepare), 0)
	}

	decider.ResetPrepareAndCommitVotes()

	// one sig from external node
	_, err = decider.AddNewVote(Prepare,
		[]bls.PublicKey{pubKeys[3]},
		sKeys[3].Sign(blockHash[:]),
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID)
	if err != nil {
		test.Error(err)
	}
	if decider.IsQuorumAchieved(Prepare) {
		test.Fatal("quorum shouldn't have been achieved with only one key signing")
	}
	if decider.SignersCount(Prepare) != 1 {
		test.Errorf("signers are incorrect for harmony nodes signing with aggregate sig: have %d, expect %d", decider.SignersCount(Prepare), 1)
	}
}

func TestAddNewVoteAggregateSig(test *testing.T) {
	shard.Schedule = shardingconfig.LocalnetSchedule
	blockHash := [32]byte{}
	copy(blockHash[:], []byte("random"))
	blockNum := uint64(1000)
	viewID := uint64(2)

	decider := NewDecider(
		SuperMajorityStake, shard.BeaconChainShardID,
	)

	slotList := shard.SlotList{}
	sKeys := []bls.SecretKey{}
	pubKeys := []bls.PublicKey{}

	quorumNodes := 5

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 3 {
			newSlot.EffectiveStake = nil
		}
		sKeys = append(sKeys, sKey)
		slotList = append(slotList, newSlot)
		wrapper := sKey.PublicKey()
		pubKeys = append(pubKeys, wrapper)
	}

	// make all external keys belong to same account
	slotList[3].EcdsaAddress = slotList[4].EcdsaAddress

	decider.UpdateParticipants(pubKeys)
	decider.SetVoters(&shard.Committee{
		shard.BeaconChainShardID, slotList,
	}, big.NewInt(3))

	signatures := []bls.Signature{}
	for _, priKey := range []bls.SecretKey{sKeys[0], sKeys[1]} {
		if s := priKey.Sign(blockHash[:]); s != nil {
			signatures = append(signatures, s)
		}
	}
	aggSig := bls.AggreagateSignatures(signatures)

	// aggregate sig from all of 2 harmony nodes
	decider.AddNewVote(Prepare,
		[]bls.PublicKey{pubKeys[0], pubKeys[1]},
		aggSig,
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID)

	if decider.IsQuorumAchieved(Prepare) {
		test.Error("quorum should not have been achieved with 2 harmony nodes")
	}
	if decider.SignersCount(Prepare) != 2 {
		test.Errorf("signers are incorrect for harmony nodes signing with aggregate sig: have %d, expect %d", decider.SignersCount(Prepare), 2)
	}
	// aggregate sig from all of 2 external nodes

	signatures = []bls.Signature{}
	for _, priKey := range []bls.SecretKey{sKeys[3], sKeys[4]} {
		if s := priKey.Sign(blockHash[:]); s != nil {
			signatures = append(signatures, s)
		}
	}
	aggSig = bls.AggreagateSignatures(signatures)

	decider.AddNewVote(Prepare,
		[]bls.PublicKey{pubKeys[3], pubKeys[4]},
		aggSig,
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID)

	if !decider.IsQuorumAchieved(Prepare) {
		test.Error("quorum should have been achieved with 2 harmony nodes")
	}
	if decider.SignersCount(Prepare) != 4 {
		test.Errorf("signers are incorrect for harmony nodes signing with aggregate sig: have %d, expect %d", decider.SignersCount(Prepare), 4)
	}
}

func TestAddNewVoteInvalidAggregateSig(test *testing.T) {
	shard.Schedule = shardingconfig.LocalnetSchedule
	blockHash := [32]byte{}
	copy(blockHash[:], []byte("random"))
	blockNum := uint64(1000)
	viewID := uint64(2)

	decider := NewDecider(
		SuperMajorityStake, shard.BeaconChainShardID,
	)

	slotList := shard.SlotList{}
	sKeys := []bls.SecretKey{}
	pubKeys := []bls.PublicKey{}

	quorumNodes := 8

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 3 {
			newSlot.EffectiveStake = nil
		}
		sKeys = append(sKeys, sKey)
		slotList = append(slotList, newSlot)
		wrapper := sKey.PublicKey()
		pubKeys = append(pubKeys, wrapper)
	}

	// make all external keys belong to same account
	slotList[3].EcdsaAddress = slotList[7].EcdsaAddress
	slotList[4].EcdsaAddress = slotList[7].EcdsaAddress
	slotList[5].EcdsaAddress = slotList[7].EcdsaAddress
	slotList[6].EcdsaAddress = slotList[7].EcdsaAddress

	decider.UpdateParticipants(pubKeys)
	decider.SetVoters(&shard.Committee{
		shard.BeaconChainShardID, slotList,
	}, big.NewInt(3))

	signatures := []bls.Signature{}
	for _, priKey := range []bls.SecretKey{sKeys[0], sKeys[1]} {
		if s := priKey.Sign(blockHash[:]); s != nil {
			signatures = append(signatures, s)
		}
	}
	aggSig := bls.AggreagateSignatures(signatures)

	// aggregate sig from all of 2 harmony nodes
	decider.AddNewVote(Prepare,
		[]bls.PublicKey{pubKeys[0], pubKeys[1]},
		aggSig,
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID)

	if decider.IsQuorumAchieved(Prepare) {
		test.Error("quorum should not have been achieved with 2 harmony nodes")
	}
	if decider.SignersCount(Prepare) != 2 {
		test.Errorf("signers are incorrect for harmony nodes signing with aggregate sig: have %d, expect %d", decider.SignersCount(Prepare), 2)
	}

	signatures = []bls.Signature{}
	for _, priKey := range []bls.SecretKey{sKeys[3], sKeys[4]} {
		if s := priKey.Sign(blockHash[:]); s != nil {
			signatures = append(signatures, s)
		}
	}
	aggSig = bls.AggreagateSignatures(signatures)

	// aggregate sig from all of 2 external nodes
	_, err := decider.AddNewVote(Prepare,
		[]bls.PublicKey{pubKeys[3], pubKeys[4]},
		aggSig,
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID)

	if err != nil {
		test.Error(err, "expect no error")
	}
	if decider.SignersCount(Prepare) != 4 {
		test.Errorf("signers are incorrect for harmony nodes signing with aggregate sig: have %d, expect %d", decider.SignersCount(Prepare), 4)
	}

	// Aggregate Vote should only contain sig from 0, 1, 3, 4
	fourSigs := decider.AggregateVotes(Prepare)
	aggPubKey := bls.AggreagatePublicKeys([]bls.PublicKey{pubKeys[0], pubKeys[1], pubKeys[3], pubKeys[4]})

	if !fourSigs.Verify(aggPubKey, blockHash[:]) {
		test.Error("Failed to aggregate votes for 4 keys from 2 aggregate sigs")
	}

	_, err = decider.AddNewVote(Prepare,
		[]bls.PublicKey{pubKeys[3], pubKeys[7]},
		aggSig,
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID)

	if !strings.Contains(err.Error(), "vote is already submitted") {
		test.Error(err, "expect error due to already submitted votes")
	}
	if decider.SignersCount(Prepare) != 4 {
		test.Errorf("signers are incorrect for harmony nodes signing with aggregate sig: have %d, expect %d", decider.SignersCount(Prepare), 4)
	}

	_, err = decider.AddNewVote(Prepare,
		[]bls.PublicKey{pubKeys[6], pubKeys[5], pubKeys[6]},
		aggSig,
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID)

	if !strings.Contains(err.Error(), "duplicate key found in votes") {
		test.Error(err, "expect error due to duplicate keys in aggregated votes")
	}
	if decider.SignersCount(Prepare) != 4 {
		test.Errorf("signers are incorrect for harmony nodes signing with aggregate sig: have %d, expect %d", decider.SignersCount(Prepare), 4)
	}
}

func TestInvalidAggregateSig(test *testing.T) {
	shard.Schedule = shardingconfig.LocalnetSchedule
	blockHash := [32]byte{}
	copy(blockHash[:], []byte("random"))

	slotList := shard.SlotList{}
	sKeys := []bls.SecretKey{}
	pubKeys := []bls.PublicKey{}

	quorumNodes := 8

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 3 {
			newSlot.EffectiveStake = nil
		}
		sKeys = append(sKeys, sKey)
		slotList = append(slotList, newSlot)
		wrapper := sKey.PublicKey()
		pubKeys = append(pubKeys, wrapper)
	}

	signatures := []bls.Signature{}
	for _, priKey := range []bls.SecretKey{sKeys[0], sKeys[1], sKeys[2], sKeys[2]} {
		if s := priKey.Sign(blockHash[:]); s != nil {
			signatures = append(signatures, s)
		}
	}
	aggSig := bls.AggreagateSignatures(signatures)

	aggPubKey := bls.AggreagatePublicKeys([]bls.PublicKey{pubKeys[0], pubKeys[1], pubKeys[2]})

	if aggSig.Verify(aggPubKey, blockHash[:]) {
		test.Error("Expect aggregate signature verification to fail due to duplicate signing from one key")
	}

	signatures = []bls.Signature{}
	for _, priKey := range []bls.SecretKey{sKeys[0], sKeys[1], sKeys[2]} {
		if s := priKey.Sign(blockHash[:]); s != nil {
			signatures = append(signatures, s)
		}
	}
	aggSig = bls.AggreagateSignatures(signatures)
	if !aggSig.Verify(aggPubKey, blockHash[:]) {
		test.Error("Expect aggregate signature verification to succeed with correctly matched keys and sigs")
	}
}
