package params

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsOneEpochBeforeHIP30(t *testing.T) {
	c := ChainConfig{
		HIP30Epoch: big.NewInt(3),
	}

	require.True(t, c.IsOneEpochBeforeHIP30(big.NewInt(2)))
	require.False(t, c.IsOneEpochBeforeHIP30(big.NewInt(3)))
}
