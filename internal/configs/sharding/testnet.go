package shardingconfig

import "math/big"

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

var testnetV0 = MustNewInstance(2, 150, 150)
