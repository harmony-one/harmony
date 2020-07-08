package consensus

import (
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	harmony_bls "github.com/harmony-one/harmony/crypto/bls"
	"github.com/stretchr/testify/assert"
)

func TestBasicViewChanging(t *testing.T) {
	_, _, consensus, _, err := GenerateConsensusForTesting()
	assert.NoError(t, err)

	state := State{mode: Normal}

	// Change Mode
	assert.Equal(t, state.mode, consensus.current.mode)
	assert.Equal(t, state.Mode(), consensus.current.Mode())

	consensus.current.SetMode(ViewChanging)
	assert.Equal(t, ViewChanging, consensus.current.mode)
	assert.Equal(t, ViewChanging, consensus.current.Mode())

	// Change ViewID
	assert.Equal(t, state.viewID, consensus.current.viewID)
	assert.Equal(t, state.ViewID(), consensus.current.ViewID())
	assert.Equal(t, state.GetViewID(), consensus.current.GetViewID()) // Why are there two methods to retrieve the ViewID?

	newViewID := consensus.current.ViewID() + 1
	consensus.current.SetViewID(newViewID)
	assert.Equal(t, newViewID, consensus.current.viewID)
	assert.Equal(t, newViewID, consensus.current.ViewID())
	assert.Equal(t, newViewID, consensus.current.GetViewID())
}

func TestPhaseSwitching(t *testing.T) {
	type phaseSwitch struct {
		start FBFTPhase
		end   FBFTPhase
	}

	phases := []FBFTPhase{FBFTAnnounce, FBFTPrepare, FBFTCommit}

	_, _, consensus, _, err := GenerateConsensusForTesting()
	assert.NoError(t, err)

	assert.Equal(t, FBFTAnnounce, consensus.phase) // It's a new consensus, we should be at the FBFTAnnounce phase

	override := false

	switches := []phaseSwitch{
		{start: FBFTAnnounce, end: FBFTPrepare},
		{start: FBFTPrepare, end: FBFTCommit},
		{start: FBFTCommit, end: FBFTAnnounce},
	}

	for _, sw := range switches {
		testPhaseGroupSwitching(t, consensus, phases, sw.start, sw.end, override)
	}

	override = true

	for _, sw := range switches {
		testPhaseGroupSwitching(t, consensus, phases, sw.start, sw.end, override)
	}

	switches = []phaseSwitch{
		{start: FBFTAnnounce, end: FBFTCommit},
		{start: FBFTPrepare, end: FBFTAnnounce},
		{start: FBFTCommit, end: FBFTPrepare},
	}

	for _, sw := range switches {
		testPhaseGroupSwitching(t, consensus, phases, sw.start, sw.end, override)
	}
}

func testPhaseGroupSwitching(t *testing.T, consensus *Consensus, phases []FBFTPhase, startPhase FBFTPhase, desiredPhase FBFTPhase, override bool) {
	phaseMapping := make(map[FBFTPhase]bool)

	if override {
		for range phases {
			consensus.switchPhase(desiredPhase, override)
			assert.Equal(t, desiredPhase, consensus.phase)
		}

		assert.Equal(t, desiredPhase, consensus.phase)

		return
	}

	phaseMapping[FBFTAnnounce] = false
	phaseMapping[FBFTPrepare] = false
	phaseMapping[FBFTCommit] = false
	phaseMapping[startPhase] = false
	phaseMapping[desiredPhase] = true

	assert.Equal(t, startPhase, consensus.phase)

	for _, phase := range phases {
		consensus.switchPhase(desiredPhase, override)
		expected := phaseMapping[phase]
		assert.Equal(t, expected, (phase == consensus.phase))
	}

	assert.Equal(t, desiredPhase, consensus.phase)
}

func TestGetNextLeaderKeyShouldFailForStandardGeneratedConsensus(t *testing.T) {
	_, _, consensus, _, err := GenerateConsensusForTesting()
	assert.NoError(t, err)

	// The below results in: "panic: runtime error: integer divide by zero"
	// This happens because there's no check for if there are any participants or not in https://github.com/harmony-one/harmony/blob/main/consensus/quorum/quorum.go#L188-L197
	assert.Panics(t, func() { consensus.GetNextLeaderKey() })
}

func TestGetNextLeaderKeyShouldSucceed(t *testing.T) {
	_, _, consensus, _, err := GenerateConsensusForTesting()
	assert.NoError(t, err)

	assert.Equal(t, int64(0), consensus.Decider.ParticipantsCount())

	blsKeys := []*bls_core.PublicKey{}
	wrappedBLSKeys := []*bls.PublicKeyWrapper{}

	keyCount := int64(5)
	for i := int64(0); i < keyCount; i++ {
		blsKey := harmony_bls.RandPrivateKey()
		blsPubKey := blsKey.GetPublicKey()
		bytes := bls.SerializedPublicKey{}
		bytes.FromLibBLSPublicKey(blsPubKey)
		wrapped := &bls.PublicKeyWrapper{Object: blsPubKey, Bytes: bytes}

		blsKeys = append(blsKeys, blsPubKey)
		wrappedBLSKeys = append(wrappedBLSKeys, wrapped)
	}

	consensus.Decider.UpdateParticipants(blsKeys)
	assert.Equal(t, keyCount, consensus.Decider.ParticipantsCount())

	consensus.LeaderPubKey = wrappedBLSKeys[0]
	nextKey := consensus.GetNextLeaderKey()

	assert.Equal(t, nextKey, wrappedBLSKeys[1])
}
