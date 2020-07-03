package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModeStrings(t *testing.T) {
	modes := []Mode{
		Normal,
		ViewChanging,
		Syncing,
		Listening,
	}

	expectations := make(map[Mode]string)
	expectations[Normal] = "Normal"
	expectations[ViewChanging] = "ViewChanging"
	expectations[Syncing] = "Syncing"
	expectations[Listening] = "Listening"

	for _, mode := range modes {
		expected := expectations[mode]
		assert.Equal(t, expected, mode.String())
	}
}

func TestPhaseStrings(t *testing.T) {
	phases := []FBFTPhase{
		FBFTAnnounce,
		FBFTPrepare,
		FBFTCommit,
	}

	expectations := make(map[FBFTPhase]string)
	expectations[FBFTAnnounce] = "Announce"
	expectations[FBFTPrepare] = "Prepare"
	expectations[FBFTCommit] = "Commit"

	for _, phase := range phases {
		expected := expectations[phase]
		assert.Equal(t, expected, phase.String())
	}
}
