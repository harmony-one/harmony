package crosslinks_test

import (
	"testing"
	"unsafe"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils/crosslinks"
	"github.com/stretchr/testify/require"
)

func TestCrosslink(t *testing.T) {
	t.Parallel()

	s := crosslinks.New()
	require.EqualValues(t, 0, s.LatestSentCrosslinkBlockNumber())

	s.SetLatestSentCrosslinkBlockNumber(5)
	require.EqualValues(t, 5, s.LatestSentCrosslinkBlockNumber())
}

func TestSignal(t *testing.T) {
	t.Parallel()

	s := crosslinks.New()
	require.Nil(t, s.LastKnownCrosslinkHeartbeatSignal())

	signal := &types.CrosslinkHeartbeat{LatestContinuousBlockNum: 10}
	s.SetLastKnownCrosslinkHeartbeatSignal(signal)

	// They should have same value.
	require.Equal(t, signal, s.LastKnownCrosslinkHeartbeatSignal())
	// They should have even same pointer.
	require.Equal(t, unsafe.Pointer(signal), unsafe.Pointer(s.LastKnownCrosslinkHeartbeatSignal()))
}
