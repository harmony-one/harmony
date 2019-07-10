package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
)

// LocalnetSchedule is the local testnet sharding
// configuration schedule.
var LocalnetSchedule localnetSchedule

type localnetSchedule struct{}

func (localnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(big.NewInt(20)) >= 0:
		return localnetV2
	case epoch.Cmp(big.NewInt(10)) >= 0:
		return localnetV1
	default: // genesis
		return localnetV0
	}
}

var localnetV0 = MustNewInstance(2, 7, 5, genesis.LocalHarmonyAccounts, genesis.LocalFnAccounts)
var localnetV1 = MustNewInstance(2, 7, 5, genesis.LocalHarmonyAccountsV1, genesis.LocalFnAccountsV1)
var localnetV2 = MustNewInstance(2, 7, 4, genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2)
