package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/numeric"
)

const (
	testBlockPerEpoch = 2
	testVDFDifficulty = 1

	testHTTPPattern = "test-http-%d"
	testWSPattern   = "test-ws-%d"
)

// TestSchedule is the schedule used for unit testing
var TestSchedule testSchedule

type testSchedule struct{}

// InstanceForEpoch return the instance configuration for the given epoch
func (testSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	return testV0
}

// BlocksPerEpoch return the number of blocks for each epoch
func (testSchedule) BlocksPerEpoch() uint64 {
	return testBlockPerEpoch
}

// CalcEpochNumber calculate the epoch number with the given block number
func (testSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	epoch := blockNum / testBlockPerEpoch
	return big.NewInt(int64(epoch))
}

// IsLastBlock calculates whether the given block number is the last block of an epoch
func (testSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%testBlockPerEpoch == 0
}

// VdfDifficulty return the VDF difficulty
func (testSchedule) VdfDifficulty() int {
	return testVDFDifficulty
}

// ConsensusRatio return the threshold of EPOS voting power to reach consensus
func (testSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

// GetNetworkID return the network ID
func (testSchedule) GetNetworkID() NetworkID {
	return Test
}

func (testSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, testHTTPPattern, testWSPattern)
}

var testV0 = MustNewInstance(4, 5, 0, numeric.ZeroDec(), genesis.LocalHarmonyAccounts, genesis.LocalFnAccounts, nil, testBlockPerEpoch)
