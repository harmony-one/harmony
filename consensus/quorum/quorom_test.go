package quorum

import (
	"math/big"
	"strings"
	"testing"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	harmony_bls "github.com/harmony-one/harmony/crypto/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/stretchr/testify/assert"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"
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

	decider.UpdateParticipants(blsKeys, []bls.PublicKeyWrapper{})
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

	message := "test string"
	blsPriKey1 := bls.RandPrivateKey()
	pubKeyWrapper1 := bls.PublicKeyWrapper{Object: blsPriKey1.GetPublicKey()}
	pubKeyWrapper1.Bytes.FromLibBLSPublicKey(pubKeyWrapper1.Object)

	blsPriKey2 := bls.RandPrivateKey()
	pubKeyWrapper2 := bls.PublicKeyWrapper{Object: blsPriKey2.GetPublicKey()}
	pubKeyWrapper2.Bytes.FromLibBLSPublicKey(pubKeyWrapper2.Object)

	decider.UpdateParticipants([]bls.PublicKeyWrapper{pubKeyWrapper1, pubKeyWrapper2}, []bls.PublicKeyWrapper{})

	if _, err := decider.submitVote(
		Prepare,
		[]bls.SerializedPublicKey{pubKeyWrapper1.Bytes},
		blsPriKey1.Sign(message),
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID,
	); err != nil {
		test.Log(err)
	}

	if _, err := decider.submitVote(
		Prepare,
		[]bls.SerializedPublicKey{pubKeyWrapper2.Bytes},
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

	aggSig := &bls_core.Sign{}
	aggSig.Add(blsPriKey1.Sign(message))
	aggSig.Add(blsPriKey2.Sign(message))
	if decider.AggregateVotes(Prepare).SerializeToHexStr() != aggSig.SerializeToHexStr() {
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

	blsPriKey1 := bls.RandPrivateKey()
	pubKeyWrapper1 := bls.PublicKeyWrapper{Object: blsPriKey1.GetPublicKey()}
	pubKeyWrapper1.Bytes.FromLibBLSPublicKey(pubKeyWrapper1.Object)

	blsPriKey2 := bls.RandPrivateKey()
	pubKeyWrapper2 := bls.PublicKeyWrapper{Object: blsPriKey2.GetPublicKey()}
	pubKeyWrapper2.Bytes.FromLibBLSPublicKey(pubKeyWrapper2.Object)

	blsPriKey3 := bls.RandPrivateKey()
	pubKeyWrapper3 := bls.PublicKeyWrapper{Object: blsPriKey3.GetPublicKey()}
	pubKeyWrapper3.Bytes.FromLibBLSPublicKey(pubKeyWrapper3.Object)

	decider.UpdateParticipants([]bls.PublicKeyWrapper{pubKeyWrapper1, pubKeyWrapper2}, []bls.PublicKeyWrapper{})

	decider.submitVote(
		Prepare,
		[]bls.SerializedPublicKey{pubKeyWrapper1.Bytes},
		blsPriKey1.SignHash(blockHash[:]),
		common.BytesToHash(blockHash[:]),
		blockNum,
		viewID,
	)

	aggSig := &bls_core.Sign{}
	for _, priKey := range []*bls_core.SecretKey{blsPriKey2, blsPriKey3} {
		if s := priKey.SignHash(blockHash[:]); s != nil {
			aggSig.Add(s)
		}
	}
	if _, err := decider.submitVote(
		Prepare,
		[]bls.SerializedPublicKey{pubKeyWrapper2.Bytes, pubKeyWrapper3.Bytes},
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

	aggSig.Add(blsPriKey1.SignHash(blockHash[:]))
	if decider.AggregateVotes(Prepare).SerializeToHexStr() != aggSig.SerializeToHexStr() {
		test.Fatal("AggregateVotes failed")
	}

	if _, err := decider.submitVote(
		Prepare,
		[]bls.SerializedPublicKey{pubKeyWrapper2.Bytes},
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
	sKeys := []bls_core.SecretKey{}
	pubKeys := []bls.PublicKeyWrapper{}

	quorumNodes := 10

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 3 {
			newSlot.EffectiveStake = nil
		}
		sKeys = append(sKeys, sKey)
		slotList = append(slotList, newSlot)
		wrapper := bls.PublicKeyWrapper{Object: sKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)
		pubKeys = append(pubKeys, wrapper)
	}

	decider.UpdateParticipants(pubKeys, []bls.PublicKeyWrapper{})
	decider.SetVoters(&shard.Committee{
		ShardID: shard.BeaconChainShardID, Slots: slotList,
	}, big.NewInt(3))

	aggSig := &bls_core.Sign{}
	for _, priKey := range []*bls_core.SecretKey{&sKeys[0], &sKeys[1], &sKeys[2]} {
		if s := priKey.SignHash(blockHash[:]); s != nil {
			aggSig.Add(s)
		}
	}

	// aggregate sig from all of 3 harmony nodes
	decider.AddNewVote(Prepare,
		[]*bls.PublicKeyWrapper{&pubKeys[0], &pubKeys[1], &pubKeys[2]},
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
	aggSig = &bls_core.Sign{}
	for _, priKey := range []*bls_core.SecretKey{&sKeys[3], &sKeys[4], &sKeys[5]} {
		if s := priKey.SignHash(blockHash[:]); s != nil {
			aggSig.Add(s)
		}
	}
	_, err := decider.AddNewVote(Prepare,
		[]*bls.PublicKeyWrapper{&pubKeys[3], &pubKeys[4], &pubKeys[5]},
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
		[]*bls.PublicKeyWrapper{&pubKeys[3]},
		sKeys[3].SignHash(blockHash[:]),
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
	sKeys := []bls_core.SecretKey{}
	pubKeys := []bls.PublicKeyWrapper{}

	quorumNodes := 5

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 3 {
			newSlot.EffectiveStake = nil
		}
		sKeys = append(sKeys, sKey)
		slotList = append(slotList, newSlot)
		wrapper := bls.PublicKeyWrapper{Object: sKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)
		pubKeys = append(pubKeys, wrapper)
	}

	// make all external keys belong to same account
	slotList[3].EcdsaAddress = slotList[4].EcdsaAddress

	decider.UpdateParticipants(pubKeys, []bls.PublicKeyWrapper{})
	decider.SetVoters(&shard.Committee{
		ShardID: shard.BeaconChainShardID, Slots: slotList,
	}, big.NewInt(3))

	aggSig := &bls_core.Sign{}
	for _, priKey := range []*bls_core.SecretKey{&sKeys[0], &sKeys[1]} {
		if s := priKey.SignHash(blockHash[:]); s != nil {
			aggSig.Add(s)
		}
	}

	// aggregate sig from all of 2 harmony nodes
	decider.AddNewVote(Prepare,
		[]*bls.PublicKeyWrapper{&pubKeys[0], &pubKeys[1]},
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

	aggSig = &bls_core.Sign{}
	for _, priKey := range []*bls_core.SecretKey{&sKeys[3], &sKeys[4]} {
		if s := priKey.SignHash(blockHash[:]); s != nil {
			aggSig.Add(s)
		}
	}
	decider.AddNewVote(Prepare,
		[]*bls.PublicKeyWrapper{&pubKeys[3], &pubKeys[4]},
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
	sKeys := []bls_core.SecretKey{}
	pubKeys := []bls.PublicKeyWrapper{}

	quorumNodes := 8

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 3 {
			newSlot.EffectiveStake = nil
		}
		sKeys = append(sKeys, sKey)
		slotList = append(slotList, newSlot)
		wrapper := bls.PublicKeyWrapper{Object: sKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)
		pubKeys = append(pubKeys, wrapper)
	}

	// make all external keys belong to same account
	slotList[3].EcdsaAddress = slotList[7].EcdsaAddress
	slotList[4].EcdsaAddress = slotList[7].EcdsaAddress
	slotList[5].EcdsaAddress = slotList[7].EcdsaAddress
	slotList[6].EcdsaAddress = slotList[7].EcdsaAddress

	decider.UpdateParticipants(pubKeys, []bls.PublicKeyWrapper{})
	decider.SetVoters(&shard.Committee{
		ShardID: shard.BeaconChainShardID, Slots: slotList,
	}, big.NewInt(3))

	aggSig := &bls_core.Sign{}
	for _, priKey := range []*bls_core.SecretKey{&sKeys[0], &sKeys[1]} {
		if s := priKey.SignHash(blockHash[:]); s != nil {
			aggSig.Add(s)
		}
	}

	// aggregate sig from all of 2 harmony nodes
	decider.AddNewVote(Prepare,
		[]*bls.PublicKeyWrapper{&pubKeys[0], &pubKeys[1]},
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

	aggSig = &bls_core.Sign{}
	for _, priKey := range []*bls_core.SecretKey{&sKeys[3], &sKeys[4]} {
		if s := priKey.SignHash(blockHash[:]); s != nil {
			aggSig.Add(s)
		}
	}
	// aggregate sig from all of 2 external nodes
	_, err := decider.AddNewVote(Prepare,
		[]*bls.PublicKeyWrapper{&pubKeys[3], &pubKeys[4]},
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
	aggPubKey := &bls_core.PublicKey{}
	for _, priKey := range []*bls_core.PublicKey{pubKeys[0].Object, pubKeys[1].Object, pubKeys[3].Object, pubKeys[4].Object} {
		aggPubKey.Add(priKey)
	}
	if !fourSigs.VerifyHash(aggPubKey, blockHash[:]) {
		test.Error("Failed to aggregate votes for 4 keys from 2 aggregate sigs")
	}

	_, err = decider.AddNewVote(Prepare,
		[]*bls.PublicKeyWrapper{&pubKeys[3], &pubKeys[7]},
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
		[]*bls.PublicKeyWrapper{&pubKeys[6], &pubKeys[5], &pubKeys[6]},
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
	sKeys := []bls_core.SecretKey{}
	pubKeys := []bls.PublicKeyWrapper{}

	quorumNodes := 8

	for i := 0; i < quorumNodes; i++ {
		newSlot, sKey := generateRandomSlot()
		if i < 3 {
			newSlot.EffectiveStake = nil
		}
		sKeys = append(sKeys, sKey)
		slotList = append(slotList, newSlot)
		wrapper := bls.PublicKeyWrapper{Object: sKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)
		pubKeys = append(pubKeys, wrapper)
	}

	aggSig := &bls_core.Sign{}
	for _, priKey := range []*bls_core.SecretKey{&sKeys[0], &sKeys[1], &sKeys[2], &sKeys[2]} {
		if s := priKey.SignHash(blockHash[:]); s != nil {
			aggSig.Add(s)
		}
	}

	aggPubKey := &bls_core.PublicKey{}

	for _, priKey := range []*bls_core.PublicKey{pubKeys[0].Object, pubKeys[1].Object, pubKeys[2].Object} {
		aggPubKey.Add(priKey)
	}

	if aggSig.VerifyHash(aggPubKey, blockHash[:]) {
		test.Error("Expect aggregate signature verification to fail due to duplicate signing from one key")
	}

	aggSig = &bls_core.Sign{}
	for _, priKey := range []*bls_core.SecretKey{&sKeys[0], &sKeys[1], &sKeys[2]} {
		if s := priKey.SignHash(blockHash[:]); s != nil {
			aggSig.Add(s)
		}
	}
	if !aggSig.VerifyHash(aggPubKey, blockHash[:]) {
		test.Error("Expect aggregate signature verification to succeed with correctly matched keys and sigs")
	}
}

func TestNthNextHmyExt(test *testing.T) {
	numHmyNodes := 10
	numAllExtNodes := 10
	numAllowlistExtNodes := numAllExtNodes / 2
	allowlist := shardingconfig.Allowlist{MaxLimitPerShard: numAllowlistExtNodes - 1}
	blsKeys := []harmony_bls.PublicKeyWrapper{}
	for i := 0; i < numHmyNodes+numAllExtNodes; i++ {
		blsKey := harmony_bls.RandPrivateKey()
		wrapper := harmony_bls.PublicKeyWrapper{Object: blsKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)
		blsKeys = append(blsKeys, wrapper)
	}
	allowlistLeaders := blsKeys[len(blsKeys)-allowlist.MaxLimitPerShard:]
	allLeaders := append(blsKeys[:numHmyNodes], allowlistLeaders...)

	decider := NewDecider(SuperMajorityVote, shard.BeaconChainShardID)
	fakeInstance := shardingconfig.MustNewInstance(2, 20, numHmyNodes, 0, numeric.OneDec(), nil, nil, allowlist, nil, nil, 0)

	decider.UpdateParticipants(blsKeys, allowlistLeaders)
	for i := 0; i < len(allLeaders); i++ {
		leader := allLeaders[i]
		for j := 0; j < len(allLeaders)*2; j++ {
			expectNextLeader := allLeaders[(i+j)%len(allLeaders)]
			found, nextLeader := decider.NthNextHmyExt(fakeInstance, &leader, j)
			if !found {
				test.Fatal("next leader not found")
			}
			if expectNextLeader.Bytes != nextLeader.Bytes {
				test.Fatal("next leader is not expected")
			}

			preJ := -j
			preIndex := (i + len(allLeaders) + preJ%len(allLeaders)) % len(allLeaders)
			expectPreLeader := allLeaders[preIndex]
			found, preLeader := decider.NthNextHmyExt(fakeInstance, &leader, preJ)
			if !found {
				test.Fatal("previous leader not found")
			}
			if expectPreLeader.Bytes != preLeader.Bytes {
				test.Fatal("previous leader is not expected")
			}
		}
	}
}
