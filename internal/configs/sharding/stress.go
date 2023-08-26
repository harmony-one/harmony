package shardingconfig

import (
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/params"
)

// StressNetSchedule is the long-running public stressNet sharding
// configuration schedule.
var StressNetSchedule stressnetSchedule

type stressnetSchedule struct{}

const (
	// ~0.5 hour per epoch
	stressnetBlocksPerEpoch = 1024

	stressnetVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	// StressNetHTTPPattern is the http pattern for stressnet.
	StressNetHTTPPattern = "https://api.s%d.stn.hmny.io"
	// StressNetWSPattern is the websocket pattern for stressnet.
	StressNetWSPattern = "wss://ws.s%d.stn.hmny.io"
)

func (ss stressnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case params.StressnetChainConfig.IsSixtyPercent(epoch):
		return stressnetV2
	case epoch.Cmp(params.StressnetChainConfig.StakingEpoch) >= 0:
		return stressnetV1
	default: // genesis
		return stressnetV0
	}
}

func (ss stressnetSchedule) BlocksPerEpoch() uint64 {
	return stressnetBlocksPerEpoch
}

func (ss stressnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	epoch := blockNum / ss.BlocksPerEpoch()
	return big.NewInt(int64(epoch))
}

func (ss stressnetSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ss.BlocksPerEpoch() == 0
}

func (ss stressnetSchedule) EpochLastBlock(epochNum uint64) uint64 {
	return ss.BlocksPerEpoch()*(epochNum+1) - 1
}

func (ss stressnetSchedule) VdfDifficulty() int {
	return stressnetVdfDifficulty
}

func (ss stressnetSchedule) GetNetworkID() NetworkID {
	return StressNet
}

// GetShardingStructure is the sharding structure for stressnet.
func (ss stressnetSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, StressNetHTTPPattern, StressNetWSPattern)
}

// IsSkippedEpoch returns if an epoch was skipped on shard due to staking epoch
func (ss stressnetSchedule) IsSkippedEpoch(shardID uint32, epoch *big.Int) bool {
	return false
}

var stressnetReshardingEpoch = []*big.Int{
	big.NewInt(0),
	params.StressnetChainConfig.StakingEpoch,
}

var stressnetV0 = MustNewInstance(
	2, 10, 10, 0,
	numeric.OneDec(), genesis.TNHarmonyAccounts,
	genesis.TNFoundationalAccounts, emptyAllowlist, nil,
	numeric.ZeroDec(), ethCommon.Address{},
	stressnetReshardingEpoch, StressNetSchedule.BlocksPerEpoch(),
)
var stressnetV1 = MustNewInstance(
	2, 30, 10, 0,
	numeric.MustNewDecFromStr("0.9"), genesis.TNHarmonyAccounts,
	genesis.TNFoundationalAccounts, emptyAllowlist, nil,
	numeric.ZeroDec(), ethCommon.Address{},
	stressnetReshardingEpoch, StressNetSchedule.BlocksPerEpoch(),
)
var stressnetV2 = MustNewInstance(
	2, 30, 10, 0,
	numeric.MustNewDecFromStr("0.6"), genesis.TNHarmonyAccounts,
	genesis.TNFoundationalAccounts, emptyAllowlist, nil,
	numeric.ZeroDec(), ethCommon.Address{},
	stressnetReshardingEpoch, StressNetSchedule.BlocksPerEpoch(),
)
