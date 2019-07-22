package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
)

// TestnetSchedule is the long-running public testnet sharding
// configuration schedule.
var TestnetSchedule testnetSchedule

type testnetSchedule struct{}

const (
	testnetV1Epoch = 1
	testnetV2Epoch = 2
)

func (testnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(big.NewInt(testnetV2Epoch)) >= 0:
		return testnetV2
	case epoch.Cmp(big.NewInt(testnetV1Epoch)) >= 0:
		return testnetV1
	default: // genesis
		return testnetV0
	}
}

func (testnetSchedule) BlocksPerEpoch() uint64 {
	// 8 seconds per block, roughly 86400 blocks, around one day
	return 60
}

var testnetReshardingEpoch = []*big.Int{big.NewInt(0), big.NewInt(testnetV1Epoch), big.NewInt(testnetV2Epoch)}

var testnetV0 = MustNewInstance(2, 150, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
var testnetV1 = MustNewInstance(2, 160, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
var testnetV2 = MustNewInstance(2, 170, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
