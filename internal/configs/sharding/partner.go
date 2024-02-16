package shardingconfig

import (
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/params"
)

// PartnerSchedule is the long-running public partner sharding
// configuration schedule.
var PartnerSchedule partnerSchedule

var feeCollectorsDevnet = []FeeCollectors{
	FeeCollectors{
		mustAddress("0xb728AEaBF60fD01816ee9e756c18bc01dC91ba5D"): numeric.OneDec(),
	},
	FeeCollectors{
		mustAddress("0xb728AEaBF60fD01816ee9e756c18bc01dC91ba5D"): numeric.MustNewDecFromStr("0.5"),
		mustAddress("0xb41B6B8d9e68fD44caC8342BC2EEf4D59531d7d7"): numeric.MustNewDecFromStr("0.5"),
	},
}

type partnerSchedule struct{}

const (
	// 30 min per epoch (at 2s/block)
	partnerBlocksPerEpoch = 900

	partnerVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	// PartnerHTTPPattern is the http pattern for partner.
	PartnerHTTPPattern = "https://api.s%d.ps.hmny.io"
	// PartnerWSPattern is the websocket pattern for partner.
	PartnerWSPattern = "wss://ws.s%d.ps.hmny.io"
)

func (ps partnerSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case params.PartnerChainConfig.IsDevnetExternalEpoch(epoch):
		return partnerV3
	case params.PartnerChainConfig.IsHIP30(epoch):
		return partnerV2
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
	params.PartnerChainConfig.StakingEpoch,
}

var partnerV0 = MustNewInstance(
	2, 5, 5, 0,
	numeric.OneDec(), genesis.TNHarmonyAccounts,
	genesis.TNFoundationalAccounts, emptyAllowlist, nil,
	numeric.ZeroDec(), ethCommon.Address{},
	partnerReshardingEpoch, PartnerSchedule.BlocksPerEpoch(),
)
var partnerV1 = MustNewInstance(
	2, 15, 4, 0,
	numeric.MustNewDecFromStr("0.9"), genesis.TNHarmonyAccounts,
	genesis.TNFoundationalAccounts, emptyAllowlist, nil,
	numeric.ZeroDec(), ethCommon.Address{},
	partnerReshardingEpoch, PartnerSchedule.BlocksPerEpoch(),
)
var partnerV2 = MustNewInstance(
	2, 20, 4, 0,
	numeric.MustNewDecFromStr("0.9"), genesis.TNHarmonyAccounts,
	genesis.TNFoundationalAccounts, emptyAllowlist,
	feeCollectorsDevnet[1], numeric.MustNewDecFromStr("0.25"),
	hip30CollectionAddressTestnet, partnerReshardingEpoch,
	PartnerSchedule.BlocksPerEpoch(),
)
var partnerV3 = MustNewInstance(
	2, 20, 0, 0,
	numeric.MustNewDecFromStr("0.0"), genesis.TNHarmonyAccounts,
	genesis.TNFoundationalAccounts, emptyAllowlist,
	feeCollectorsDevnet[1], numeric.MustNewDecFromStr("0.25"),
	hip30CollectionAddressTestnet, partnerReshardingEpoch,
	PartnerSchedule.BlocksPerEpoch(),
)
