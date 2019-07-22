package shardingconfig

import (
	"math/big"
)

const (
	// VLBPE is a Very Large Block Per Epoch
	VLBPE = 1000000000000
)

type fixedSchedule struct {
	instance Instance
}

// InstanceForEpoch returns the fixed sharding configuration instance regardless
// the given epoch.
func (s fixedSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	return s.instance
}

func (s fixedSchedule) BlocksPerEpoch() uint64 {
	return VLBPE
}

// NewFixedSchedule returns a sharding configuration schedule that uses the
// given config instance for all epochs.  Useful for testing.
func NewFixedSchedule(instance Instance) Schedule {
	return fixedSchedule{instance: instance}
}
