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
	case epoch.Cmp(big.NewInt(5)) >= 0:
		return testnetV2
	case epoch.Cmp(big.NewInt(2)) >= 0:
		return testnetV1
	default: // genesis
		return testnetV0
	}
}

var testnetV0 = MustNewInstance(2, 5, 5, genesis.TNHarmonyAccounts, genesis.FoundationalNodeAccounts)
var testnetV1 = MustNewInstance(2, 5, 5, genesis.TNHarmonyAccountsV1, genesis.FoundationalNodeAccounts)
var testnetV2 = MustNewInstance(2, 6, 6, genesis.TNHarmonyAccountsV2, genesis.FoundationalNodeAccounts)
