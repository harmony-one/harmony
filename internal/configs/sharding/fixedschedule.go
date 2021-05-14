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

func (s fixedSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	return big.NewInt(int64(blockNum / s.BlocksPerEpoch()))
}

func (s fixedSchedule) IsLastBlock(blockNum uint64) bool {
	blocks := s.BlocksPerEpoch()
	return blockNum%blocks == blocks-1
}

func (s fixedSchedule) EpochLastBlock(epochNum uint64) uint64 {
	blocks := s.BlocksPerEpoch()
	return blocks*(epochNum+1) - 1
}

func (s fixedSchedule) VdfDifficulty() int {
	return mainnetVdfDifficulty
}

func (s fixedSchedule) GetNetworkID() NetworkID {
	return DevNet
}

// GetShardingStructure is the sharding structure for fixed schedule.
func (s fixedSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, TestNetHTTPPattern, TestNetHTTPPattern)
}

// IsSkippedEpoch returns if an epoch was skipped on shard due to staking epoch
func (s fixedSchedule) IsSkippedEpoch(shardID uint32, epoch *big.Int) bool {
	return false
}

// NewFixedSchedule returns a sharding configuration schedule that uses the
// given config instance for all epochs.  Useful for testing.
func NewFixedSchedule(instance Instance) Schedule {
	return fixedSchedule{instance: instance}
}
