package stagedstreamsync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrIfNotNextBlock(t *testing.T) {
	require.NoError(t, errIfNotNextBlock(0, 1))
	require.NoError(t, errIfNotNextBlock(10, 11))
	require.NoError(t, errIfNotNextBlock(^uint64(0)-1, ^uint64(0)))

	require.ErrorIs(t, errIfNotNextBlock(0, 0), ErrUnexpectedBlockNumber)
	require.ErrorIs(t, errIfNotNextBlock(10, 10), ErrUnexpectedBlockNumber)
	require.ErrorIs(t, errIfNotNextBlock(10, 12), ErrUnexpectedBlockNumber)
	require.ErrorIs(t, errIfNotNextBlock(10, 9), ErrUnexpectedBlockNumber)
}
