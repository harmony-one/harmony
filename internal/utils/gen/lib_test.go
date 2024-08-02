package gen

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestSeed(t *testing.T) {
	t.Parallel()
	g := New(0)
	addr1 := g.Address()
	if addr1 == (common.Address{}) {
		t.Error("address should not be empty")
	}
	g2 := New(0)
	require.Equal(t, addr1, g2.Address())
	require.NotEqual(t, addr1, g2.Address())

	g3 := New(1)
	require.NotEqual(t, addr1, g3.Address())
}
