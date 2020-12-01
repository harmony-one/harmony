package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/params"
)

// PartnerSchedule is the long-running public partner sharding
// configuration schedule.
var PartnerSchedule partnerSchedule

type partnerSchedule struct{}

const (
	// 10 minutes per epoch (at 8s/block)
	partnerBlocksPerEpoch = 75

	partnerVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	// PartnerHTTPPattern is the http pattern for partner.
	PartnerHTTPPattern = "https://api.s%d.ps.hmny.io"
	// PartnerWSPattern is the websocket pattern for partner.
	PartnerWSPattern = "wss://ws.s%d.ps.hmny.io"
)

func (ps partnerSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(params.PartnerChainConfig.StakingEpoch) >= 0:
		return partnerV1
	default: // genesis
		return partnerV0
	}
}

func (ps partnerSchedule) BlocksPerEpoch() uint64 {
	return partnerBlocksPerEpoch
}

func (ps partnerSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	epoch := blockNum / ps.BlocksPerEpoch()
	return big.NewInt(int64(epoch))
}

func (ps partnerSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ps.BlocksPerEpoch() == 0
}

func (ps partnerSchedule) EpochLastBlock(epochNum uint64) uint64 {
	return ps.BlocksPerEpoch()*(epochNum+1) - 1
}

func (ps partnerSchedule) VdfDifficulty() int {
	return partnerVdfDifficulty
}

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (ps partnerSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}

func (ps partnerSchedule) GetNetworkID() NetworkID {
	return Partner
}

// GetShardingStructure is the sharding structure for partner.
func (ps partnerSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, PartnerHTTPPattern, PartnerWSPattern)
}

// IsSkippedEpoch returns if an epoch was skipped on shard due to staking epoch
func (ps partnerSchedule) IsSkippedEpoch(shardID uint32, epoch *big.Int) bool {
	return false
}

var partnerReshardingEpoch = []*big.Int{
	big.NewInt(0),
	params.TestnetChainConfig.StakingEpoch,
}

var partnerV0 = MustNewInstance(2, 15, 15, numeric.OneDec(), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, partnerReshardingEpoch, PartnerSchedule.BlocksPerEpoch())
var partnerV1 = MustNewInstance(2, 30, 15, numeric.MustNewDecFromStr("0.68"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, partnerReshardingEpoch, PartnerSchedule.BlocksPerEpoch())
