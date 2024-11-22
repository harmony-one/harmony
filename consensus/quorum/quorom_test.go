package quorum

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	harmony_bls "github.com/harmony-one/harmony/crypto/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/shard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func createTestCIdentities(numAddresses int, keysPerAddress int) (*cIdentities, shard.SlotList, []harmony_bls.PublicKeyWrapper) {
	testAddresses := make([]common.Address, 0, numAddresses*numAddresses)
	for i := int(0); i < numAddresses; i++ {
		h := fmt.Sprintf("0x%040x", i)
		addr := common.HexToAddress(h)
		testAddresses = append(testAddresses, addr)
	}
	slots := shard.SlotList{}
	list := []harmony_bls.PublicKeyWrapper{}
	// generate slots and public keys
	for i := 0; i < numAddresses; i++ {
		for j := 0; j < keysPerAddress; j++ { // keys per address
			blsKey := harmony_bls.RandPrivateKey()
			wrapper := harmony_bls.PublicKeyWrapper{Object: blsKey.GetPublicKey()}
			wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)

			slots = append(slots, shard.Slot{
				EcdsaAddress:   testAddresses[i],
				BLSPublicKey:   wrapper.Bytes,
				EffectiveStake: nil,
			})
			list = append(list, wrapper)
		}
	}
	// initialize and update cIdentities
	c := newCIdentities()
	c.UpdateParticipants(list, []bls.PublicKeyWrapper{})
	return c, slots, list
}

func TestCIdentities_NthNextValidatorHmy(t *testing.T) {
	c, slots, list := createTestCIdentities(3, 3)

	found, key := c.NthNextValidator(slots, &list[0], 1)
	require.Equal(t, true, found)
	// because we skip 3 keys of current validator
	require.Equal(t, 3, c.IndexOf(key.Bytes))
}

func TestCIdentities_NthNextValidatorFailedEdgeCase1(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from panic as expected: %v", r)
		} else {
			t.Errorf("Expected a panic when next is 0, but no panic occurred")
		}
	}()

	// create test identities and slots
	c, slots, _ := createTestCIdentities(3, 3)

	// create a public key wrapper that doesn't exist in the identities
	t.Log("creating a random public key wrapper not present in test identities")
	blsKey := harmony_bls.RandPrivateKey()
	wrapper := harmony_bls.PublicKeyWrapper{Object: blsKey.GetPublicKey()}

	// Edge Case: Trigger NthNextValidator with next=0, which should cause a panic
	t.Log("Calling NthNextValidator with next=0 to test panic handling")
	c.NthNextValidator(slots, &wrapper, 0)
}

func TestCIdentities_NthNextValidatorFailedEdgeCase2(t *testing.T) {
	// create test identities and slots
	c, slots, list := createTestCIdentities(1, 3)

	done := make(chan bool)

	go func() {
		// possible infinite loop, it will time out
		c.NthNextValidator(slots, &list[1], 1)

		done <- true
	}()

	select {
	case <-done:
		t.Error("Expected a timeout, but successfully calculated next leader")

	case <-time.After(5 * time.Second):
		t.Log("Test timed out, possible infinite loop")
	}
}

func TestCIdentities_NthNextValidatorFailedEdgeCase3(t *testing.T) {
	// create 3 test addresses
	testAddresses := make([]common.Address, 0, 3)
	for i := int(0); i < 3; i++ {
		h := fmt.Sprintf("0x%040x", i)
		addr := common.HexToAddress(h)
		testAddresses = append(testAddresses, addr)
	}
	slots := shard.SlotList{}
	list := []harmony_bls.PublicKeyWrapper{}

	// First add 4 keys for first address
	for i := 0; i < 4; i++ {
		blsKey := harmony_bls.RandPrivateKey()
		wrapper := harmony_bls.PublicKeyWrapper{Object: blsKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)

		slots = append(slots, shard.Slot{
			EcdsaAddress:   testAddresses[0],
			BLSPublicKey:   wrapper.Bytes,
			EffectiveStake: nil,
		})
		list = append(list, wrapper)
	}

	// Then add 1 key for next two addresses
	for i := 1; i < 3; i++ {
		blsKey := harmony_bls.RandPrivateKey()
		wrapper := harmony_bls.PublicKeyWrapper{Object: blsKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)

		slots = append(slots, shard.Slot{
			EcdsaAddress:   testAddresses[i],
			BLSPublicKey:   wrapper.Bytes,
			EffectiveStake: nil,
		})
		list = append(list, wrapper)
	}

	// initialize and update cIdentities
	c := newCIdentities()
	c.UpdateParticipants(list, []bls.PublicKeyWrapper{})

	// current key is the first one.
	found, key := c.NthNextValidator(slots, &list[0], 1)
	require.Equal(t, true, found)
	// because we skip 4 keys of first validator, the next validator key index is 4 (starts from 0)
	// but it returns 5 and skips second validator (key index: 4)
	require.Equal(t, 5, c.IndexOf(key.Bytes))
	t.Log("second validator were skipped")
}

func TestCIdentities_NthNextValidatorV2Hmy(t *testing.T) {
	c, slots, list := createTestCIdentities(3, 3)

	found, key := c.NthNextValidatorV2(slots, &list[0], 1)
	require.Equal(t, true, found)
	// because we skip 3 keys of current validator
	require.Equal(t, 3, c.IndexOf(key.Bytes))
}

func TestCIdentities_NthNextValidatorV2EdgeCase1(t *testing.T) {
	// create test identities and slots
	c, slots, _ := createTestCIdentities(3, 3)

	// create a public key wrapper that doesn't exist in the identities
	t.Log("creating a random public key wrapper not present in test identities")
	blsKey := harmony_bls.RandPrivateKey()
	wrapper := harmony_bls.PublicKeyWrapper{Object: blsKey.GetPublicKey()}

	// Edge Case: Trigger NthNextValidator with next=0, which should cause a panic
	t.Log("Calling NthNextValidatorV2 with next=0 to test panic handling")
	found, key := c.NthNextValidatorV2(slots, &wrapper, 0)

	require.Equal(t, true, found)
	require.Equal(t, 0, c.IndexOf(key.Bytes))
}

func TestCIdentities_NthNextValidatorV2EdgeCase2(t *testing.T) {
	// create test identities and slots
	c, slots, list := createTestCIdentities(1, 3)

	done := make(chan bool)

	go func() {
		c.NthNextValidatorV2(slots, &list[1], 1)

		done <- true
	}()

	select {
	case <-done:
		t.Log("Test completed successfully ")
	case <-time.After(5 * time.Second):
		t.Error("timeout, possible infinite loop")
	}
}

func TestCIdentities_NthNextValidatorV2EdgeCase3(t *testing.T) {
	// create 3 test addresses
	testAddresses := make([]common.Address, 0, 3)
	for i := int(0); i < 3; i++ {
		h := fmt.Sprintf("0x%040x", i)
		addr := common.HexToAddress(h)
		testAddresses = append(testAddresses, addr)
	}
	slots := shard.SlotList{}
	list := []harmony_bls.PublicKeyWrapper{}

	// First add 4 keys for first address
	for i := 0; i < 4; i++ {
		blsKey := harmony_bls.RandPrivateKey()
		wrapper := harmony_bls.PublicKeyWrapper{Object: blsKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)

		slots = append(slots, shard.Slot{
			EcdsaAddress:   testAddresses[0],
			BLSPublicKey:   wrapper.Bytes,
			EffectiveStake: nil,
		})
		list = append(list, wrapper)
	}

	// Then add 1 key for next two addresses
	for i := 1; i < 3; i++ {
		blsKey := harmony_bls.RandPrivateKey()
		wrapper := harmony_bls.PublicKeyWrapper{Object: blsKey.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)

		slots = append(slots, shard.Slot{
			EcdsaAddress:   testAddresses[i],
			BLSPublicKey:   wrapper.Bytes,
			EffectiveStake: nil,
		})
		list = append(list, wrapper)
	}

	// initialize and update cIdentities
	c := newCIdentities()
	c.UpdateParticipants(list, []bls.PublicKeyWrapper{})

	// current key is the first one.
	found, key := c.NthNextValidatorV2(slots, &list[0], 1)
	require.Equal(t, true, found)
	// because we skip 4 keys of first validator, the next validator key index is 4 (starts from 0)
	require.Equal(t, 4, c.IndexOf(key.Bytes))
}
