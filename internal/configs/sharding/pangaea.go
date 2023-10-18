package shardingconfig

import (
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/params"
)

// PangaeaSchedule is the Pangaea sharding configuration schedule.
var PangaeaSchedule pangaeaSchedule

type pangaeaSchedule struct{}

const (
	// 8*450=3600 sec epochs for P3 of open staking
	pangaeaBlocksPerEpoch = 450

	pangaeaVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	// PangaeaHTTPPattern is the http pattern for pangaea.
	PangaeaHTTPPattern = "https://api.s%d.os.hmny.io"
	// PangaeaWSPattern is the websocket pattern for pangaea.
	PangaeaWSPattern = "wss://ws.s%d.os.hmny.io"
)

func (ps pangaeaSchedule) InstanceForEpoch(epoch *big.Int) Instance {
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

func (ps pangaeaSchedule) GetNetworkID() NetworkID {
	return Pangaea
}

// GetShardingStructure is the sharding structure for mainnet.
func (ps pangaeaSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, PangaeaHTTPPattern, PangaeaWSPattern)
}

// IsSkippedEpoch returns if an epoch was skipped on shard due to staking epoch
func (ps pangaeaSchedule) IsSkippedEpoch(shardID uint32, epoch *big.Int) bool {
	return false
}

var pangaeaReshardingEpoch = []*big.Int{
	big.NewInt(0),
	params.PangaeaChainConfig.StakingEpoch,
}

var pangaeaV0 = MustNewInstance(
	4, 30, 30, 0, numeric.OneDec(), genesis.TNHarmonyAccounts,
	genesis.TNFoundationalAccounts, emptyAllowlist, nil,
	numeric.ZeroDec(), ethCommon.Address{},
	pangaeaReshardingEpoch, PangaeaSchedule.BlocksPerEpoch(),
)
var pangaeaV1 = MustNewInstance(
	4, 110, 30, 0, numeric.MustNewDecFromStr("0.68"),
	genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts,
	emptyAllowlist, nil, numeric.ZeroDec(), ethCommon.Address{},
	pangaeaReshardingEpoch, PangaeaSchedule.BlocksPerEpoch(),
)
