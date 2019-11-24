package shardingconfig

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/genesis"
)

const (
	// PangaeaHTTPPattern is the http pattern for pangaea.
	PangaeaHTTPPattern = "https://api.s%d.p.hmny.io"
	// PangaeaWSPattern is the websocket pattern for pangaea.
	PangaeaWSPattern = "wss://ws.s%d.p.hmny.io"
)

// PangaeaSchedule is the Pangaea sharding configuration schedule.
var PangaeaSchedule pangaeaSchedule

type pangaeaSchedule struct{}

func (ps pangaeaSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	return pangaeaV0
}

func (ps pangaeaSchedule) BlocksPerEpoch() uint64 {
	return 2700 // 6 hours with 8 seconds/block
}

func (ps pangaeaSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	return big.NewInt(int64(blockNum / ps.BlocksPerEpoch()))
}

func (ps pangaeaSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ps.BlocksPerEpoch() == 0
}

func (ps pangaeaSchedule) EpochLastBlock(epochNum uint64) uint64 {
	blocks := ps.BlocksPerEpoch()
	return blocks*(epochNum+1) - 1
}

func (ps pangaeaSchedule) VdfDifficulty() int {
	return testnetVdfDifficulty
}

func (ps pangaeaSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

var pangaeaReshardingEpoch = []*big.Int{common.Big0}

var pangaeaV0 = MustNewInstance(
	2, 50, 40, genesis.PangaeaAccounts, genesis.FoundationalPangaeaAccounts, pangaeaReshardingEpoch)

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (ps pangaeaSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}

func (pangaeaSchedule) GetNetworkID() NetworkID {
	return Pangaea
}

// GetShardingStructure is the sharding structure for mainnet.
func (pangaeaSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, PangaeaHTTPPattern, PangaeaWSPattern)
}
