package consensus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLockedFBFTPhase(t *testing.T) {
	s := NewLockedFBFTPhase(FBFTAnnounce)
	require.Equal(t, FBFTAnnounce, s.Get())

	s.Set(FBFTCommit)
	require.Equal(t, FBFTCommit, s.Get())
}
