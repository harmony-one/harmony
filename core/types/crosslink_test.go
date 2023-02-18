package types

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCrosslinks_Sorting(t *testing.T) {
	bb := CrossLinks{
		{
			BlockNumberF: big.NewInt(1),
			ViewIDF:      big.NewInt(0),
		},
		{
			BlockNumberF: big.NewInt(1),
			ViewIDF:      big.NewInt(1),
		},
		{
			BlockNumberF: big.NewInt(1),
			ViewIDF:      big.NewInt(4),
		},
		{
			BlockNumberF: big.NewInt(1),
			ViewIDF:      big.NewInt(3),
		},
		{
			BlockNumberF: big.NewInt(1),
			ViewIDF:      big.NewInt(2),
		},
	}
	bb.Sort()

	for i, v := range bb {
		require.EqualValues(t, i, v.ViewID().Uint64())
	}

	require.True(t, bb.IsSorted())
}

func TestBigNumberInequality(t *testing.T) {
	type A struct {
		X int
	}
	require.False(t, big.NewInt(1) == big.NewInt(1))
	require.False(t, &A{} == &A{})
}
