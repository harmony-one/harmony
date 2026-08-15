package types

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTotalRedelegatableUndelegations(t *testing.T) {
	currEpoch := big.NewInt(2965)
	delegations := Undelegations{
		{Amount: big.NewInt(100), Epoch: big.NewInt(2964)},
		{Amount: big.NewInt(200), Epoch: big.NewInt(2965)},
		{Amount: big.NewInt(300), Epoch: big.NewInt(2963)},
	}

	total := TotalRedelegatableUndelegations(delegations, currEpoch)
	require.Equal(t, int64(400), total.Int64())
}
