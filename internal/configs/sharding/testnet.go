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

	testnetV2Epoch = 6050 // per shard, reduce internal node from 15 to 8, and external nodes from 5 to 22
)

var (
	harmonyVotePercentFix = numeric.MustNewDecFromStr("0.70")
	// testnet failed to propose the last block of epoch 75893.
	// it stopped at 26481646, and last block of epoch 75893 is 26481647.
	harmonyVotePercentFixEpoch = big.NewInt(75894)
)

// isForked returns whether a fork scheduled at epoch s is active at the given head epoch.
func isForked(s, epoch *big.Int) bool {
	if s == nil || epoch == nil {
		return false
	}
	return s.Cmp(epoch) <= 0
}

func (ts testnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case isForked(harmonyVotePercentFixEpoch, epoch):
		return testnetV3_4
	case params.TestnetChainConfig.IsAllowlistEpoch(epoch):
		return testnetV3_3
	case params.TestnetChainConfig.IsSlotsLimited(epoch):
		return testnetV3_2
	case params.TestnetChainConfig.IsSixtyPercent(epoch):
		return testnetV3_1
	case params.TestnetChainConfig.IsTwoSeconds(epoch):
		return testnetV3
	case epoch.Cmp(big.NewInt(testnetV2Epoch)) >= 0:
		return testnetV2
	case epoch.Cmp(params.TestnetChainConfig.StakingEpoch) >= 0:
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

var testnetV0 = MustNewInstance(4, 16, 15, 0, numeric.OneDec(), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, emptyAllowlist, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpochOld())
var testnetV1 = MustNewInstance(4, 20, 15, 0, numeric.MustNewDecFromStr("0.90"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, emptyAllowlist, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpochOld())
var testnetV2 = MustNewInstance(4, 30, 8, 0, numeric.MustNewDecFromStr("0.90"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, emptyAllowlist, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpochOld())
var testnetV3 = MustNewInstance(4, 30, 8, 0, numeric.MustNewDecFromStr("0.90"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, emptyAllowlist, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
var testnetV3_1 = MustNewInstance(4, 30, 8, 0, numeric.MustNewDecFromStr("0.60"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, emptyAllowlist, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
var testnetV3_2 = MustNewInstance(4, 30, 8, 0.15, numeric.MustNewDecFromStr("0.60"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, emptyAllowlist, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
var testnetV3_3 = MustNewInstance(4, 30, 8, 0.15, numeric.MustNewDecFromStr("0.60"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetAllowlistV3_3, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
var testnetV3_4 = MustNewInstance(4, 30, 8, 0.15, harmonyVotePercentFix, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetAllowlistV3_3, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
