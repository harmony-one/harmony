package types

import (
	"math/big"
	"testing"

	"github.com/harmony-one/harmony/internal/utils/gen"
	"github.com/stretchr/testify/require"
)

func TestCrosslinks_Sorting(t *testing.T) {
	var bb CrossLinks = []CrossLink{
		&CrossLinkV1{
			BlockNumberF: big.NewInt(1),
			ViewIDF:      big.NewInt(0),
		},
		&CrossLinkV1{
			BlockNumberF: big.NewInt(1),
			ViewIDF:      big.NewInt(1),
		},
		&CrossLinkV1{
			BlockNumberF: big.NewInt(1),
			ViewIDF:      big.NewInt(4),
		},
		&CrossLinkV1{
			BlockNumberF: big.NewInt(1),
			ViewIDF:      big.NewInt(3),
		},
		&CrossLinkV1{
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

func FuzzSerializationV2(f *testing.F) {
	f.Fuzz(func(t *testing.T, seed int64) {
		r := gen.New(seed)
		x := CrossLinkV2{
			CrossLinkV1: CrossLinkV1{
				HashF:        r.Hash(),
				BlockNumberF: r.BigInt(),
				ViewIDF:      r.BigInt(),
				SignatureF:   r.Bytes96(),
				BitmapF:      r.BytesN(32),
				ShardIDF:     r.Uint32(),
				EpochF:       r.BigInt(),
			},
			Proposer: r.Address(),
		}
		bytes := x.Serialize()
		x2, err := DeserializeCrossLinkV2(bytes)
		require.NoError(t, err)
		require.Equal(t, x, *x2)
	})
}
