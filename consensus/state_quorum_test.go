package consensus

import "testing"

func TestState_LastQuorumAchievedBlock(t *testing.T) {
	state := NewState(Normal, 0)

	if got := state.getLastPrepareQuorumBlock(); got != 0 {
		t.Fatalf("Prepare last quorum: got %d, want 0", got)
	}
	if got := state.getLastCommitQuorumBlock(); got != 0 {
		t.Fatalf("Commit last quorum: got %d, want 0", got)
	}

	state.setLastPrepareQuorumBlock(10)
	state.setLastCommitQuorumBlock(11)

	if got := state.getLastPrepareQuorumBlock(); got != 10 {
		t.Fatalf("Prepare last quorum: got %d, want 10", got)
	}
	if got := state.getLastCommitQuorumBlock(); got != 11 {
		t.Fatalf("Commit last quorum: got %d, want 11", got)
	}

	// Phases are independent; setting one must not clobber the other.
	state.setLastPrepareQuorumBlock(12)
	if got := state.getLastCommitQuorumBlock(); got != 11 {
		t.Fatalf("Commit last quorum changed unexpectedly: got %d, want 11", got)
	}

	state.clearLastQuorumAchievedBlocks()
	if got := state.getLastPrepareQuorumBlock(); got != 0 {
		t.Fatalf("Prepare last quorum after clear: got %d, want 0", got)
	}
	if got := state.getLastCommitQuorumBlock(); got != 0 {
		t.Fatalf("Commit last quorum after clear: got %d, want 0", got)
	}
}
