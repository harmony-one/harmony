package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
)

// MainnetSchedule is the mainnet sharding configuration schedule.
var MainnetSchedule mainnetSchedule

type mainnetSchedule struct{}

func (mainnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	//case epoch.Cmp(big.NewInt(1000)) >= 0:
	//	return mainnet6400
	//case epoch.Cmp(big.NewInt(100)) >= 0:
	//	return mainnetV2
	default: // genesis
		return mainnetV0
	}
}

var mainnetReshardingEpoch = make([]*big.Int, 0)
var mainnetV0 = MustNewInstance(4, 150, 112, genesis.HarmonyAccounts, genesis.FoundationalNodeAccounts, mainnetReshardingEpoch)

//var mainnetV2 = MustNewInstance(8, 200, 100)
//var mainnet6400 = MustNewInstance(16, 400, 50)
