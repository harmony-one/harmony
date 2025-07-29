package vm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEpoch_NotWriteProtected(t *testing.T) {
	require.False(t, (&epoch{}).IsWrite(), "epoch should not be a write operation")
}

func TestVrf_NotWriteProtected(t *testing.T) {
	require.False(t, (&vrf{}).IsWrite(), "vrf should not be a write operation")
}

func TestCrossShardXferPrecompile_IsWrite(t *testing.T) {
	require.True(t, (&crossShardXferPrecompile{}).IsWrite(), "crossShardXferPrecompile should be a write operation")
}

func TestStakingPrecompile_IsWrite(t *testing.T) {
	require.True(t, (&stakingPrecompile{}).IsWrite(), "stakingPrecompile should be a write operation")
}
