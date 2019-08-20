package shardingconfig

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/internal/genesis"
)

// PangaeaSchedule is the Pangaea sharding configuration schedule.
var PangaeaSchedule pangaeaSchedule

type pangaeaSchedule struct{}

func (pangaeaSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	return pangaeaV0
}

func (pangaeaSchedule) BlocksPerEpoch() uint64 {
	return 10800 // 1 day with 8 seconds/block
}

func (ps pangaeaSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	return big.NewInt(int64(blockNum / ps.BlocksPerEpoch()))
}

func (ps pangaeaSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ps.BlocksPerEpoch() == 0
}

func (pangaeaSchedule) VdfDifficulty() int {
	return testnetVdfDifficulty
}

func (pangaeaSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

var pangaeaReshardingEpoch = []*big.Int{common.Big0}

var pangaeaV0 = MustNewInstance(
	4, 250, 230, genesis.PangaeaAccounts, genesis.FoundationalPangaeaAccounts, pangaeaReshardingEpoch)

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (pangaeaSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}
