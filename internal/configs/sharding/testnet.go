package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
)

// TestnetSchedule is the long-running public testnet sharding
// configuration schedule.
var TestnetSchedule testnetSchedule

var ninetyPercentEpoch = big.NewInt(399)
var shardReductionEpoch = big.NewInt(486)

var feeCollectorsTestnet = FeeCollectors{
	mustAddress("0xb728AEaBF60fD01816ee9e756c18bc01dC91ba5D"): numeric.MustNewDecFromStr("0.5"),
	mustAddress("0xb41B6B8d9e68fD44caC8342BC2EEf4D59531d7d7"): numeric.MustNewDecFromStr("0.5"),
}

type testnetSchedule struct{}

const (
	// ~304 sec epochs for P2 of open staking
	testnetBlocksPerEpoch = 38

	// 4.5 hours per epoch (given 2s block time)
	testnetBlocksPerEpochV2 = 8192

	testnetVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	// TestNetHTTPPattern is the http pattern for testnet.
	TestNetHTTPPattern = "https://api.s%d.b.hmny.io"
	// TestNetWSPattern is the websocket pattern for testnet.
	TestNetWSPattern = "wss://ws.s%d.b.hmny.io"
)

func (ts testnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case params.TestnetChainConfig.IsFeeCollectEpoch(epoch):
		return testnetV4
	case epoch.Cmp(shardReductionEpoch) >= 0:
		return testnetV3
	case epoch.Cmp(ninetyPercentEpoch) >= 0:
		return testnetV2
	case params.TestnetChainConfig.IsStaking(epoch):
		return testnetV1
	default: // genesis
		return testnetV0
	}
}

func (ts testnetSchedule) BlocksPerEpochOld() uint64 {
	return testnetBlocksPerEpoch
}

func (ts testnetSchedule) BlocksPerEpoch() uint64 {
	return testnetBlocksPerEpochV2
}

func (ts testnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {

	firstBlock2s := params.TestnetChainConfig.TwoSecondsEpoch.Uint64() * ts.BlocksPerEpochOld()
	switch {
	case blockNum >= firstBlock2s:
		return big.NewInt(int64((blockNum-firstBlock2s)/ts.BlocksPerEpoch() + params.TestnetChainConfig.TwoSecondsEpoch.Uint64()))
	default: // genesis
		oldEpoch := blockNum / ts.BlocksPerEpochOld()
		return big.NewInt(int64(oldEpoch))
	}

}

func (ts testnetSchedule) IsLastBlock(blockNum uint64) bool {
	firstBlock2s := params.TestnetChainConfig.TwoSecondsEpoch.Uint64() * ts.BlocksPerEpochOld()

	switch {
	case blockNum >= firstBlock2s:
		return ((blockNum-firstBlock2s)%ts.BlocksPerEpoch() == ts.BlocksPerEpoch()-1)
	default: // genesis
		return (blockNum+1)%ts.BlocksPerEpochOld() == 0
	}
}

func (ts testnetSchedule) EpochLastBlock(epochNum uint64) uint64 {
	firstBlock2s := params.TestnetChainConfig.TwoSecondsEpoch.Uint64() * ts.BlocksPerEpochOld()

	switch {
	case params.TestnetChainConfig.IsTwoSeconds(big.NewInt(int64(epochNum))):
		return firstBlock2s - 1 + ts.BlocksPerEpoch()*(epochNum-params.TestnetChainConfig.TwoSecondsEpoch.Uint64()+1)
	default: // genesis
		return ts.BlocksPerEpochOld()*(epochNum+1) - 1
	}

}

func (ts testnetSchedule) VdfDifficulty() int {
	return testnetVdfDifficulty
}

func (ts testnetSchedule) GetNetworkID() NetworkID {
	return TestNet
}

// GetShardingStructure is the sharding structure for testnet.
func (ts testnetSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, TestNetHTTPPattern, TestNetWSPattern)
}

// IsSkippedEpoch returns if an epoch was skipped on shard due to staking epoch
func (ts testnetSchedule) IsSkippedEpoch(shardID uint32, epoch *big.Int) bool {
	return false
}

var testnetReshardingEpoch = []*big.Int{
	big.NewInt(0),
	params.TestnetChainConfig.StakingEpoch,
	params.TestnetChainConfig.TwoSecondsEpoch,
}

var (
	testnetV0 = MustNewInstance(4, 8, 8, 0, numeric.OneDec(), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, emptyAllowlist, nil, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
	testnetV1 = MustNewInstance(4, 30, 8, 0.15, numeric.MustNewDecFromStr("0.70"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, emptyAllowlist, nil, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
	testnetV2 = MustNewInstance(4, 30, 8, 0.15, numeric.MustNewDecFromStr("0.90"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, emptyAllowlist, nil, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
	testnetV3 = MustNewInstance(2, 30, 8, 0.15, numeric.MustNewDecFromStr("0.90"), genesis.TNHarmonyAccountsV1, genesis.TNFoundationalAccounts, emptyAllowlist, nil, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
	testnetV4 = MustNewInstance(2, 30, 8, 0.15, numeric.MustNewDecFromStr("0.90"), genesis.TNHarmonyAccountsV1, genesis.TNFoundationalAccounts, emptyAllowlist, feeCollectorsTestnet, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
)
