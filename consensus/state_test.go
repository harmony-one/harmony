package consensus_test

import (
	"testing"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/quorum"
)

func TestState_SetBlockNum(t *testing.T) {
	state := consensus.NewState(consensus.Normal, 0)
	if state.GetBlockNum() == 1 {
		t.Errorf("GetBlockNum expected not to be 1")
	}
	state.SetBlockNum(1)
	if state.GetBlockNum() != 1 {
		t.Errorf("SetBlockNum failed")
	}
}

func TestState_LastQuorumAchievedBlock(t *testing.T) {
	state := consensus.NewState(consensus.Normal, 0)

	if got := state.GetLastQuorumAchievedBlock(quorum.Prepare); got != 0 {
		t.Fatalf("Prepare last quorum: got %d, want 0", got)
	}
	if got := state.GetLastQuorumAchievedBlock(quorum.Commit); got != 0 {
		t.Fatalf("Commit last quorum: got %d, want 0", got)
	}

	state.SetLastQuorumAchievedBlock(quorum.Prepare, 10)
	state.SetLastQuorumAchievedBlock(quorum.Commit, 11)

	if got := state.GetLastQuorumAchievedBlock(quorum.Prepare); got != 10 {
		t.Fatalf("Prepare last quorum: got %d, want 10", got)
	}
	if got := state.GetLastQuorumAchievedBlock(quorum.Commit); got != 11 {
		t.Fatalf("Commit last quorum: got %d, want 11", got)
	}

	// Phases are independent; setting one must not clobber the other.
	state.SetLastQuorumAchievedBlock(quorum.Prepare, 12)
	if got := state.GetLastQuorumAchievedBlock(quorum.Commit); got != 11 {
		t.Fatalf("Commit last quorum changed unexpectedly: got %d, want 11", got)
	}

	// Unknown phases are ignored.
	state.SetLastQuorumAchievedBlock(quorum.ViewChange, 99)
	if got := state.GetLastQuorumAchievedBlock(quorum.ViewChange); got != 0 {
		t.Fatalf("ViewChange last quorum: got %d, want 0", got)
	}
}
