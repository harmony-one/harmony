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
	testnetVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	// TestNetHTTPPattern is the http pattern for testnet.
	TestNetHTTPPattern = "https://api.s%d.b.hmny.io"
	// TestNetWSPattern is the websocket pattern for testnet.
	TestNetWSPattern = "wss://ws.s%d.b.hmny.io"
)

func (testnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	return testnetV0
}

func (testnetSchedule) BlocksPerEpoch() uint64 {
	return 450 // 1 hour with 8 seconds/block
}

func (ts testnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	return big.NewInt(int64(blockNum / ts.BlocksPerEpoch()))
}

func (ts testnetSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ts.BlocksPerEpoch() == 0
}

func (ts testnetSchedule) EpochLastBlock(epochNum uint64) uint64 {
	blocks := ts.BlocksPerEpoch()
	return blocks*(epochNum+1) - 1
}

func (ts testnetSchedule) VdfDifficulty() int {
	return testnetVdfDifficulty
}

// ConsensusRatio ratio of new nodes vs consensus total nodes
func (ts testnetSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (ts testnetSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}

func (ts testnetSchedule) GetNetworkID() NetworkID {
	return TestNet
}

// GetShardingStructure is the sharding structure for testnet.
func (ts testnetSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, TestNetHTTPPattern, TestNetWSPattern)
}

var testnetReshardingEpoch = []*big.Int{big.NewInt(0)}

var testnetV0 = MustNewInstance(3, 10, 10, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
