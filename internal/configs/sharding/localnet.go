package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
)

// LocalnetSchedule is the local testnet sharding
// configuration schedule.
var LocalnetSchedule localnetSchedule

type localnetSchedule struct{}

const (
	localnetV1Epoch = 10
	localnetV2Epoch = 20
)

func (localnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(big.NewInt(localnetV2Epoch)) >= 0:
		return localnetV2
	case epoch.Cmp(big.NewInt(localnetV1Epoch)) >= 0:
		return localnetV1
	default: // genesis
		return localnetV0
	}
}

var localnetReshardingEpoch = []*big.Int{big.NewInt(0), big.NewInt(localnetV1Epoch), big.NewInt(localnetV2Epoch)}

var localnetV0 = MustNewInstance(2, 7, 5, genesis.LocalHarmonyAccounts, genesis.LocalFnAccounts, localnetReshardingEpoch)
var localnetV1 = MustNewInstance(2, 7, 5, genesis.LocalHarmonyAccountsV1, genesis.LocalFnAccountsV1, localnetReshardingEpoch)
var localnetV2 = MustNewInstance(2, 10, 4, genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2, localnetReshardingEpoch)
