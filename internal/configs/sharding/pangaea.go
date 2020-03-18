package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/params"
)

// PangaeaSchedule is the Pangaea sharding configuration schedule.
var PangaeaSchedule pangaeaSchedule

type pangaeaSchedule struct{}

const (
	// ~64 sec epochs for P1 of open staking
	pangaeaBlocksPerEpoch = 8

	pangaeaVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	// PangaeaHTTPPattern is the http pattern for pangaea.
	PangaeaHTTPPattern = "https://api.s%d.os.hmny.io"
	// PangaeaWSPattern is the websocket pattern for pangaea.
	PangaeaWSPattern = "wss://ws.s%d.os.hmny.io"
)

func (pangaeaSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(params.PangaeaChainConfig.StakingEpoch) >= 0:
		return pangaeaV1
	default: // genesis
		return pangaeaV0
	}
}

func (ps pangaeaSchedule) BlocksPerEpoch() uint64 {
	return pangaeaBlocksPerEpoch
}

func (ps pangaeaSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	epoch := blockNum / ps.BlocksPerEpoch()
	return big.NewInt(int64(epoch))
}

func (ps pangaeaSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ps.BlocksPerEpoch() == 0
}

func (ps pangaeaSchedule) EpochLastBlock(epochNum uint64) uint64 {
	return ps.BlocksPerEpoch()*(epochNum+1) - 1
}

func (ps pangaeaSchedule) VdfDifficulty() int {
	return pangaeaVdfDifficulty
}

func (ps pangaeaSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

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

var pangaeaReshardingEpoch = []*big.Int{
	big.NewInt(0),
	params.PangaeaChainConfig.StakingEpoch,
}

var pangaeaV0 = MustNewInstance(4, 60, 60, numeric.OneDec(), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, pangaeaReshardingEpoch, PangaeaSchedule.BlocksPerEpoch())
var pangaeaV1 = MustNewInstance(4, 110, 60, numeric.MustNewDecFromStr("0.68"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, pangaeaReshardingEpoch, PangaeaSchedule.BlocksPerEpoch())
