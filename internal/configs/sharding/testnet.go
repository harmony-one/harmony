package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
)

// TestnetSchedule is the long-running public testnet sharding
// configuration schedule.
var TestnetSchedule testnetSchedule

type testnetSchedule struct{}

func (testnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	default: // genesis
		return testnetV0
	}
}

func (testnetSchedule) BlocksPerEpoch() uint64 {
	// 8 seconds per block, roughly 86400 blocks, around one day
	return 10800
}

var testnetReshardingEpoch = make([]*big.Int, 0)
var testnetV0 = MustNewInstance(2, 150, 150, genesis.TNHarmonyAccounts, genesis.FoundationalNodeAccounts, testnetReshardingEpoch)
