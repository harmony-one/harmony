package shardingconfig

import (
	"fmt"
	"math/big"

	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
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

type localnetSchedule struct{}

const (
	localnetV1Epoch = 1

	localnetEpochBlock1      = 5
	localnetBlocksPerEpoch   = 64
	localnetBlocksPerEpochV2 = 64

	localnetVdfDifficulty = 5000 // This takes about 10s to finish the vdf
)

func (ls localnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
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
	return localnetBlocksPerEpoch
}

func (ls localnetSchedule) BlocksPerEpoch() uint64 {
	return localnetBlocksPerEpochV2
}

func (ls localnetSchedule) twoSecondsFirstBlock() uint64 {
	if params.LocalnetChainConfig.TwoSecondsEpoch.Uint64() == 0 {
		return 0
	}
	return (params.LocalnetChainConfig.TwoSecondsEpoch.Uint64()-1)*ls.BlocksPerEpochOld() + localnetEpochBlock1
}

func (ls localnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	firstBlock2s := ls.twoSecondsFirstBlock()
	switch {
	case blockNum < localnetEpochBlock1:
		return big.NewInt(0)
	case blockNum < firstBlock2s:
		return big.NewInt(int64((blockNum-localnetEpochBlock1)/ls.BlocksPerEpochOld() + 1))
	default:
		extra := uint64(0)
		if firstBlock2s == 0 {
			blockNum -= localnetEpochBlock1
			extra = 1
		}
		return big.NewInt(int64(extra + (blockNum-firstBlock2s)/ls.BlocksPerEpoch() + params.LocalnetChainConfig.TwoSecondsEpoch.Uint64()))
	}
}

func (ls localnetSchedule) IsLastBlock(blockNum uint64) bool {
	switch {
	case blockNum < localnetEpochBlock1-1:
		return false
	case blockNum == localnetEpochBlock1-1:
		return true
	default:
		firstBlock2s := ls.twoSecondsFirstBlock()
		switch {
		case blockNum >= firstBlock2s:
			if firstBlock2s == 0 {
				blockNum -= localnetEpochBlock1
			}
			return ((blockNum-firstBlock2s)%ls.BlocksPerEpoch() == ls.BlocksPerEpoch()-1)
		default: // genesis
			blocks := ls.BlocksPerEpochOld()
			return ((blockNum-localnetEpochBlock1)%blocks == blocks-1)
		}
	}
}

func (ls localnetSchedule) EpochLastBlock(epochNum uint64) uint64 {
	switch {
	case epochNum == 0:
		return localnetEpochBlock1 - 1
	default:
		switch {
		case params.LocalnetChainConfig.IsTwoSeconds(big.NewInt(int64(epochNum))):
			blocks := ls.BlocksPerEpoch()
			firstBlock2s := ls.twoSecondsFirstBlock()
			block2s := (1 + epochNum - params.LocalnetChainConfig.TwoSecondsEpoch.Uint64()) * blocks
			if firstBlock2s == 0 {
				return block2s - blocks + localnetEpochBlock1 - 1
			}
			return firstBlock2s + block2s - 1
		default: // genesis
			blocks := ls.BlocksPerEpochOld()
			return localnetEpochBlock1 + blocks*epochNum - 1
		}
	}
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

var (
	localnetReshardingEpoch = []*big.Int{
		big.NewInt(0), big.NewInt(localnetV1Epoch), params.LocalnetChainConfig.StakingEpoch, params.LocalnetChainConfig.TwoSecondsEpoch,
	}
	// Number of shards, how many slots on each , how many slots owned by Harmony
	localnetV0   = MustNewInstance(2, 7, 5, 0, numeric.OneDec(), genesis.LocalHarmonyAccounts, genesis.LocalFnAccounts, emptyAllowlist, nil, localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpochOld())
	localnetV1   = MustNewInstance(2, 8, 5, 0, numeric.OneDec(), genesis.LocalHarmonyAccountsV1, genesis.LocalFnAccountsV1, emptyAllowlist, nil, localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpochOld())
	localnetV2   = MustNewInstance(2, 9, 6, 0, numeric.MustNewDecFromStr("0.68"), genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2, emptyAllowlist, nil, localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpochOld())
	localnetV3   = MustNewInstance(2, 9, 6, 0, numeric.MustNewDecFromStr("0.68"), genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2, emptyAllowlist, nil, localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpoch())
	localnetV3_1 = MustNewInstance(2, 9, 6, 0, numeric.MustNewDecFromStr("0.68"), genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2, emptyAllowlist, nil, localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpoch())
	localnetV3_2 = MustNewInstance(2, 9, 6, 0, numeric.MustNewDecFromStr("0.68"), genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2, emptyAllowlist, feeCollectorsLocalnet, localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpoch())
)
