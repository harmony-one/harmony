package shardingconfig

import (
	"math/big"

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

func (partnerSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(params.PartnerChainConfig.StakingEpoch) >= 0:
		return partnerV1
	default: // genesis
		return partnerV0
	}
}

func (partnerSchedule) BlocksPerEpoch() uint64 {
	return partnerBlocksPerEpoch
}

func (ts partnerSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	epoch := blockNum / ts.BlocksPerEpoch()
	return big.NewInt(int64(epoch))
}

func (ts partnerSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ts.BlocksPerEpoch() == 0
}

func (ts partnerSchedule) EpochLastBlock(epochNum uint64) uint64 {
	return ts.BlocksPerEpoch()*(epochNum+1) - 1
}

func (ts partnerSchedule) VdfDifficulty() int {
	return partnerVdfDifficulty
}

// ConsensusRatio ratio of new nodes vs consensus total nodes
func (ts partnerSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (ts partnerSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}

func (ts partnerSchedule) GetNetworkID() NetworkID {
	return Partner
}

// GetShardingStructure is the sharding structure for partner.
func (ts partnerSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, PartnerHTTPPattern, PartnerWSPattern)
}

var partnerReshardingEpoch = []*big.Int{
	big.NewInt(0),
	params.TestnetChainConfig.StakingEpoch,
}

var partnerV0 = MustNewInstance(2, 10, 10, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, partnerReshardingEpoch, PartnerSchedule.BlocksPerEpoch())
var partnerV1 = MustNewInstance(2, 20, 10, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, partnerReshardingEpoch, PartnerSchedule.BlocksPerEpoch())
