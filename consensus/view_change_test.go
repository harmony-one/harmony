package consensus

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	harmony_bls "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/shard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicViewChanging(t *testing.T) {
	_, _, consensus, _, err := GenerateConsensusForTesting()
	assert.NoError(t, err)

	state := NewState(Normal)

	// Change Mode
	assert.Equal(t, state.mode, consensus.current.mode)
	assert.Equal(t, state.Mode(), consensus.current.Mode())

	consensus.current.SetMode(ViewChanging)
	assert.Equal(t, ViewChanging, consensus.current.Mode())

	// Change ViewID
	assert.Equal(t, state.GetViewChangingID(), consensus.current.GetViewChangingID())

	newViewID := consensus.current.GetViewChangingID() + 1
	consensus.SetViewIDs(newViewID)
	assert.Equal(t, newViewID, consensus.current.GetViewChangingID())
}

func TestPhaseSwitching(t *testing.T) {
	type phaseSwitch struct {
		start FBFTPhase
		end   FBFTPhase
	}

	phases := []FBFTPhase{FBFTAnnounce, FBFTPrepare, FBFTCommit}

	_, _, consensus, _, err := GenerateConsensusForTesting()
	assert.NoError(t, err)

	assert.Equal(t, FBFTAnnounce, consensus.phase) // It's a new consensus, we should be at the FBFTAnnounce phase.

	switches := []phaseSwitch{
		{start: FBFTAnnounce, end: FBFTPrepare},
		{start: FBFTPrepare, end: FBFTCommit},
		{start: FBFTCommit, end: FBFTAnnounce},
	}

	for _, sw := range switches {
		testPhaseGroupSwitching(t, consensus, phases, sw.start, sw.end)
	}

	for _, sw := range switches {
		testPhaseGroupSwitching(t, consensus, phases, sw.start, sw.end)
	}

	switches = []phaseSwitch{
		{start: FBFTAnnounce, end: FBFTCommit},
		{start: FBFTPrepare, end: FBFTAnnounce},
		{start: FBFTCommit, end: FBFTPrepare},
	}

	for _, sw := range switches {
		testPhaseGroupSwitching(t, consensus, phases, sw.start, sw.end)
	}
}

func testPhaseGroupSwitching(t *testing.T, consensus *Consensus, phases []FBFTPhase, startPhase FBFTPhase, desiredPhase FBFTPhase) {
	for range phases {
		consensus.switchPhase("test", desiredPhase)
		assert.Equal(t, desiredPhase, consensus.phase)
	}

	assert.Equal(t, desiredPhase, consensus.phase)

	return
}

func TestGetNextLeaderKeyShouldFailForStandardGeneratedConsensus(t *testing.T) {
	_, _, consensus, _, err := GenerateConsensusForTesting()
	assert.NoError(t, err)

	// The below results in: "panic: runtime error: integer divide by zero"
	// This happens because there's no check for if there are any participants or not in https://github.com/harmony-one/harmony/blob/main/consensus/quorum/quorum.go#L188-L197
	assert.Panics(t, func() { consensus.getNextLeaderKey(uint64(1), nil) })
}

func TestGetNextLeaderKeyShouldSucceed(t *testing.T) {
	_, _, consensus, _, err := GenerateConsensusForTesting()
	assert.NoError(t, err)

	assert.Equal(t, int64(0), consensus.Decider().ParticipantsCount())

	blsKeys := []*bls_core.PublicKey{}
	wrappedBLSKeys := []bls.PublicKeyWrapper{}

	keyCount := int64(5)
	for i := int64(0); i < keyCount; i++ {
		blsKey := harmony_bls.RandPrivateKey()
		blsPubKey := blsKey.GetPublicKey()
		bytes := bls.SerializedPublicKey{}
		bytes.FromLibBLSPublicKey(blsPubKey)
		wrapped := bls.PublicKeyWrapper{Object: blsPubKey, Bytes: bytes}

		blsKeys = append(blsKeys, blsPubKey)
		wrappedBLSKeys = append(wrappedBLSKeys, wrapped)
	}

	consensus.Decider().UpdateParticipants(wrappedBLSKeys, []bls.PublicKeyWrapper{})
	assert.Equal(t, keyCount, consensus.Decider().ParticipantsCount())

	consensus.setLeaderPubKey(&wrappedBLSKeys[0])
	nextKey := consensus.getNextLeaderKey(uint64(1), nil)

	assert.Equal(t, nextKey, &wrappedBLSKeys[1])
}

func TestViewChangeNextValidator(t *testing.T) {
	decider := quorum.NewDecider(quorum.SuperMajorityVote, shard.BeaconChainShardID)
	assert.Equal(t, int64(0), decider.ParticipantsCount())
	wrappedBLSKeys := []bls.PublicKeyWrapper{}

	const keyCount = 5
	for i := 0; i < keyCount; i++ {
		blsKey := harmony_bls.RandPrivateKey()
		blsPubKey := harmony_bls.WrapperFromPrivateKey(blsKey)
		wrappedBLSKeys = append(wrappedBLSKeys, *blsPubKey.Pub)
	}

	decider.UpdateParticipants(wrappedBLSKeys, []bls.PublicKeyWrapper{})
	assert.EqualValues(t, keyCount, decider.ParticipantsCount())

	t.Run("check_different_address_for_validators", func(t *testing.T) {
		var (
			rs    *bls.PublicKeyWrapper
			err   error
			slots []shard.Slot
		)
		for i := 0; i < keyCount; i++ {
			slot := shard.Slot{
				EcdsaAddress: common.BigToAddress(big.NewInt(int64(i))),
				BLSPublicKey: wrappedBLSKeys[i].Bytes,
			}
			slots = append(slots, slot)
		}

		rs, err = viewChangeNextValidator(decider, 0, slots, &wrappedBLSKeys[0])
		require.NoError(t, err)
		require.Equal(t, &wrappedBLSKeys[0], rs)

		rs, err = viewChangeNextValidator(decider, 1, slots, &wrappedBLSKeys[0])
		require.NoError(t, err)
		require.Equal(t, &wrappedBLSKeys[1], rs)

		rs, err = viewChangeNextValidator(decider, 2, slots, &wrappedBLSKeys[0])
		require.NoError(t, err)
		require.Equal(t, &wrappedBLSKeys[2], rs)

		// and no panic or error for future 1k gaps
		for i := 0; i < 1000; i++ {
			_, err = viewChangeNextValidator(decider, i, slots, &wrappedBLSKeys[0])
			require.NoError(t, err)
		}
	})

	// we can't find next validator, because all validators have the same address
	t.Run("same_address_for_all_validators", func(t *testing.T) {
		var (
			rs    *bls.PublicKeyWrapper
			err   error
			slots []shard.Slot
		)
		for i := 0; i < keyCount; i++ {
			slot := shard.Slot{
				EcdsaAddress: common.BytesToAddress([]byte("one1ay37rp2pc3kjarg7a322vu3sa8j9puahg679z3")),
				BLSPublicKey: wrappedBLSKeys[i].Bytes,
			}
			slots = append(slots, slot)
		}

		rs, err = viewChangeNextValidator(decider, 0, slots, &wrappedBLSKeys[0])
		require.NoError(t, err)
		require.Equal(t, &wrappedBLSKeys[0], rs)

		rs, err = viewChangeNextValidator(decider, 1, slots, &wrappedBLSKeys[0])
		require.NoError(t, err)
		require.Equal(t, &wrappedBLSKeys[1], rs)

		// error because all validators belong same address
		_, err = viewChangeNextValidator(decider, 2, slots, &wrappedBLSKeys[0])
		require.Error(t, err)

		// all of them return error, no way to recover
		for i := 2; i < 1000; i++ {
			_, err = viewChangeNextValidator(decider, i, slots, &wrappedBLSKeys[0])
			require.Errorf(t, err, "error because all validators belong same address %d", i)
		}
	})

	// we can't find next validator, because all validators have the same address
	t.Run("check_5_validators_2_addrs", func(t *testing.T) {
		// Slot represents node id (BLS address)
		var (
			addr1 = common.BytesToAddress([]byte("one1ay37rp2pc3kjarg7a322vu3sa8j9puahg679z3"))
			addr2 = common.BytesToAddress([]byte("one1ay37rp2pc3kjarg7a322vu3sa8j9puahg679z4"))
			rs    *bls.PublicKeyWrapper
			err   error
		)
		slots := []shard.Slot{
			{
				EcdsaAddress: addr1,
				BLSPublicKey: wrappedBLSKeys[0].Bytes,
			},
			{
				EcdsaAddress: addr1,
				BLSPublicKey: wrappedBLSKeys[1].Bytes,
			},
			{
				EcdsaAddress: addr2,
				BLSPublicKey: wrappedBLSKeys[2].Bytes,
			},
			{
				EcdsaAddress: addr2,
				BLSPublicKey: wrappedBLSKeys[3].Bytes,
			},
			{
				EcdsaAddress: addr2,
				BLSPublicKey: wrappedBLSKeys[4].Bytes,
			},
		}

		rs, err = viewChangeNextValidator(decider, 0, slots, &wrappedBLSKeys[0])
		require.NoError(t, err)
		require.Equal(t, &wrappedBLSKeys[0], rs)

		rs, err = viewChangeNextValidator(decider, 1, slots, &wrappedBLSKeys[0])
		require.NoError(t, err)
		require.Equal(t, &wrappedBLSKeys[1], rs)

		rs, err = viewChangeNextValidator(decider, 2, slots, &wrappedBLSKeys[0])
		require.NoError(t, err)
		require.Equal(t, &wrappedBLSKeys[2], rs)

		rs, err = viewChangeNextValidator(decider, 3, slots, &wrappedBLSKeys[0])
		require.NoError(t, err)
		require.Equal(t, &wrappedBLSKeys[1], rs)
	})
}
