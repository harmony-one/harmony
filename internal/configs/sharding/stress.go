package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/params"
)

// StressNetSchedule is the long-running public stressNet sharding
// configuration schedule.
var StressNetSchedule stressnetSchedule

type stressnetSchedule struct{}

const (
	// ~304 sec epochs for P2 of open staking
	stressnetBlocksPerEpoch = 38

	stressnetVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	// StressNetHTTPPattern is the http pattern for stressnet.
	StressNetHTTPPattern = "https://api.s%d.stn.hmny.io"
	// StressNetWSPattern is the websocket pattern for stressnet.
	StressNetWSPattern = "wss://ws.s%d.stn.hmny.io"
)

func (stressnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(params.StressnetChainConfig.StakingEpoch) >= 0:
		return stressnetV1
	default: // genesis
		return stressnetV0
	}
}

func (stressnetSchedule) BlocksPerEpoch() uint64 {
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

// ConsensusRatio ratio of new nodes vs consensus total nodes
func (ss stressnetSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (ss stressnetSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}

func (ss stressnetSchedule) GetNetworkID() NetworkID {
	return StressNet
}

// GetShardingStructure is the sharding structure for stressnet.
func (ss stressnetSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, StressNetHTTPPattern, StressNetWSPattern)
}

var stressnetReshardingEpoch = []*big.Int{
	big.NewInt(0),
	params.StressnetChainConfig.StakingEpoch,
}

var stressnetV0 = MustNewInstance(2, 30, 30, numeric.OneDec(), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, stressnetReshardingEpoch, StressNetSchedule.BlocksPerEpoch())
var stressnetV1 = MustNewInstance(2, 50, 30, numeric.MustNewDecFromStr("0.9"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, stressnetReshardingEpoch, StressNetSchedule.BlocksPerEpoch())
