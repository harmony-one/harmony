package shardingconfig

import (
	"fmt"
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
)

var (
	localnetReshardingEpoch = []*big.Int{
		big.NewInt(0), big.NewInt(localnetV1Epoch), params.LocalnetChainConfig.StakingEpoch, params.LocalnetChainConfig.TwoSecondsEpoch,
	}
	// Number of shards, how many slots on each , how many slots owned by Harmony
	localnetV0   Instance
	localnetV1   Instance
	localnetV2   Instance
	localnetV3   Instance
	localnetV3_1 Instance
	localnetV3_2 Instance
	localnetV4   Instance
)

// LocalnetSchedule is the local testnet sharding
// configuration schedule.
var LocalnetSchedule localnetSchedule

var feeCollectorsLocalnet = FeeCollectors{
	// pk: 0x1111111111111111111111111111111111111111111111111111111111111111
	mustAddress("0x19E7E376E7C213B7E7e7e46cc70A5dD086DAff2A"): numeric.MustNewDecFromStr("0.5"),
	// pk: 0x2222222222222222222222222222222222222222222222222222222222222222
	mustAddress("0x1563915e194D8CfBA1943570603F7606A3115508"): numeric.MustNewDecFromStr("0.5"),
}

// pk: 0x3333333333333333333333333333333333333333333333333333333333333333
var hip30CollectionAddressLocalnet = mustAddress("0x5CbDd86a2FA8Dc4bDdd8a8f69dBa48572EeC07FB")

type localnetSchedule struct{}

const (
	localnetV1Epoch = 1

	localnetEpochBlock1 = 5

	localnetBootstrapEpochBlocks = 4
	localnetBootstrapEpochs      = 3

	localnetVdfDifficulty = 5000 // This takes about 10s to finish the vdf
)

func (ls localnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case params.LocalnetChainConfig.IsOneSecond(epoch):
		return localnetV4
	case params.LocalnetChainConfig.IsHIP30(epoch):
		return localnetV4
	case params.LocalnetChainConfig.IsFeeCollectEpoch(epoch):
		return localnetV3_2
	case params.LocalnetChainConfig.IsSixtyPercent(epoch):
		return localnetV3_1
	case params.LocalnetChainConfig.IsTwoSeconds(epoch):
		return localnetV3
	case params.LocalnetChainConfig.IsStaking(epoch):
		return localnetV2
	case epoch.Cmp(big.NewInt(localnetV1Epoch)) >= 0:
		return localnetV1
	default: // genesis
		return localnetV0
	}
}

func (ls localnetSchedule) BlocksPerEpochOld() uint64 {
	localnetConfig := GetLocalnetConfig()
	return localnetConfig.BlocksPerEpoch
}

func (ls localnetSchedule) BlocksPerEpoch() uint64 {
	localnetConfig := GetLocalnetConfig()
	return localnetConfig.BlocksPerEpochV2
}

func (ls localnetSchedule) blocksInEpoch(epochNum uint64) uint64 {
	if epochNum > 0 &&
		localnetBootstrapEpochBlocks > 0 &&
		localnetBootstrapEpochs > 0 &&
		epochNum <= localnetBootstrapEpochs {
		return localnetBootstrapEpochBlocks
	}
	if params.LocalnetChainConfig.IsTwoSeconds(big.NewInt(int64(epochNum))) {
		return ls.BlocksPerEpoch()
	}
	return ls.BlocksPerEpochOld()
}

func (ls localnetSchedule) twoSecondsFirstBlock() uint64 {
	if params.LocalnetChainConfig.TwoSecondsEpoch.Uint64() == 0 {
		return 0
	}
	return ls.EpochLastBlock(params.LocalnetChainConfig.TwoSecondsEpoch.Uint64()-1) + 1
}

func (ls localnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	if blockNum < localnetEpochBlock1 {
		return big.NewInt(0)
	}
	low, high := uint64(1), uint64(1)
	for ls.EpochLastBlock(high) < blockNum {
		low = high + 1
		high *= 2
	}
	for low < high {
		mid := low + (high-low)/2
		if ls.EpochLastBlock(mid) < blockNum {
			low = mid + 1
		} else {
			high = mid
		}
	}
	return big.NewInt(int64(low))
}

func (ls localnetSchedule) IsLastBlock(blockNum uint64) bool {
	epoch := ls.CalcEpochNumber(blockNum).Uint64()
	return blockNum == ls.EpochLastBlock(epoch)
}

func (ls localnetSchedule) EpochLastBlock(epochNum uint64) uint64 {
	if epochNum == 0 {
		return localnetEpochBlock1 - 1
	}

	bootstrapEpochs := uint64(0)
	if localnetBootstrapEpochBlocks > 0 && localnetBootstrapEpochs > 0 {
		bootstrapEpochs = localnetBootstrapEpochs
		if bootstrapEpochs > epochNum {
			bootstrapEpochs = epochNum
		}
	}

	blocks := bootstrapEpochs * localnetBootstrapEpochBlocks
	regularEpochs := epochNum - bootstrapEpochs
	twoSecondsEpoch := params.LocalnetChainConfig.TwoSecondsEpoch.Uint64()
	oldEpochs := uint64(0)
	if regularEpochs > 0 && twoSecondsEpoch > bootstrapEpochs+1 {
		oldEpochs = twoSecondsEpoch - bootstrapEpochs - 1
		if oldEpochs > regularEpochs {
			oldEpochs = regularEpochs
		}
	}
	v2Epochs := regularEpochs - oldEpochs

	blocks += oldEpochs * ls.BlocksPerEpochOld()
	blocks += v2Epochs * ls.BlocksPerEpoch()
	return localnetEpochBlock1 + blocks - 1
}

func (ls localnetSchedule) VdfDifficulty() int {
	return localnetVdfDifficulty
}

func (ls localnetSchedule) GetNetworkID() NetworkID {
	return LocalNet
}

// GetShardingStructure is the sharding structure for localnet.
func (ls localnetSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	res := []map[string]interface{}{}
	for i := 0; i < numShard; i++ {
		res = append(res, map[string]interface{}{
			"current": int(shardID) == i,
			"shardID": i,
			"http":    fmt.Sprintf("http://127.0.0.1:%d", 9500+2*i),
			"ws":      fmt.Sprintf("ws://127.0.0.1:%d", 9800+2*i),
		})
	}
	return res
}

// IsSkippedEpoch returns if an epoch was skipped on shard due to staking epoch
func (ls localnetSchedule) IsSkippedEpoch(shardID uint32, epoch *big.Int) bool {
	return false
}

// RewardFrequency returns the frequency of block reward
func (ls localnetSchedule) RewardFrequency() uint64 {
	return 16
}

func InitLocalnetInstances() {
	localnetV0 = MustNewInstance(
		2, 7, 5, 0,
		numeric.OneDec(), genesis.LocalHarmonyAccounts,
		genesis.LocalFnAccounts, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpochOld(),
	)
	localnetV1 = MustNewInstance(
		2, 8, 5, 0,
		numeric.OneDec(), genesis.LocalHarmonyAccountsV1,
		genesis.LocalFnAccountsV1, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpochOld(),
	)
	localnetV2 = MustNewInstance(
		2, 9, 6, 0,
		numeric.MustNewDecFromStr("0.68"),
		genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2,
		emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpochOld(),
	)
	localnetV3 = MustNewInstance(
		2, 9, 6, 0,
		numeric.MustNewDecFromStr("0.68"),
		genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2,
		emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpoch(),
	)
	localnetV3_1 = MustNewInstance(
		2, 9, 6, 0,
		numeric.MustNewDecFromStr("0.68"),
		genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2,
		emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpoch(),
	)
	localnetV3_2 = MustNewInstance(
		2, 9, 6, 0,
		numeric.MustNewDecFromStr("0.68"),
		genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2,
		emptyAllowlist, feeCollectorsLocalnet,
		numeric.ZeroDec(), ethCommon.Address{},
		localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpoch(),
	)
	localnetV4 = MustNewInstance(
		2, 9, 6, 0, numeric.MustNewDecFromStr("0.68"),
		genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2,
		emptyAllowlist, feeCollectorsLocalnet,
		numeric.MustNewDecFromStr("0.25"), hip30CollectionAddressLocalnet,
		localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpoch(),
	)
}
