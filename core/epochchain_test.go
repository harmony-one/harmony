package core_test

import (
	"testing"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/vm"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/stretchr/testify/require"
)

func TestGenesisBlock(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	err := (&core.GenesisInitializer{NetworkType: nodeconfig.Mainnet}).InitChainDB(db, 0)
	require.NoError(t, err)

	chain, err := core.NewEpochChain(db, nil, nil, vm.Config{})
	require.NoError(t, err)

	header := chain.GetHeaderByNumber(0)
	require.NotEmpty(t, header)
}
