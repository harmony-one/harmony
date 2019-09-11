package shardingconfig

import (
	"math/big"
	"time"

	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/internal/genesis"
)

// TestnetSchedule is the long-running public testnet sharding
// configuration schedule.
var TestnetSchedule testnetSchedule

type testnetSchedule struct{}

const (
	testnetV1Epoch = 1
	testnetV2Epoch = 2

	testnetEpochBlock1 = 78
	threeOne           = 111

	testnetVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	testnetMaxTxAmountLimit               = 1e3 // unit is in One
	testnetMaxNumRecentTxsPerAccountLimit = 1e2
	testnetMaxTxPoolSizeLimit             = 8000
	testnetMaxNumTxsPerBlockLimit         = 1000
	testnetRecentTxDuration               = time.Hour
	testnetEnableTxnThrottling            = true

	// TestNetHTTPPattern is the http pattern for testnet.
	TestNetHTTPPattern = "https://api.s%d.b.hmny.io"
	// TestNetWSPattern is the websocket pattern for testnet.
	TestNetWSPattern = "wss://ws.s%d.b.hmny.io"
)

func (testnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(big.NewInt(testnetV2Epoch)) >= 0:
		return testnetV2
	case epoch.Cmp(big.NewInt(testnetV1Epoch)) >= 0:
		return testnetV1
	default: // genesis
		return testnetV0
	}
}

func (testnetSchedule) BlocksPerEpoch() uint64 {
	// 8 seconds per block, roughly 86400 blocks, around one day
	return threeOne
}

func (ts testnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	blocks := ts.BlocksPerEpoch()
	switch {
	case blockNum >= testnetEpochBlock1:
		return big.NewInt(int64((blockNum-testnetEpochBlock1)/blocks) + 1)
	default:
		return big.NewInt(0)
	}
}

func (ts testnetSchedule) IsLastBlock(blockNum uint64) bool {
	blocks := ts.BlocksPerEpoch()
	switch {
	case blockNum < testnetEpochBlock1-1:
		return false
	case blockNum == testnetEpochBlock1-1:
		return true
	default:
		return ((blockNum-testnetEpochBlock1)%blocks == blocks-1)
	}
}

func (ts testnetSchedule) EpochLastBlock(epochNum uint64) uint64 {
	blocks := ts.BlocksPerEpoch()
	switch {
	case epochNum == 0:
		return testnetEpochBlock1 - 1
	default:
		return testnetEpochBlock1 - 1 + blocks*epochNum
	}
}

func (ts testnetSchedule) VdfDifficulty() int {
	return testnetVdfDifficulty
}

// ConsensusRatio ratio of new nodes vs consensus total nodes
func (ts testnetSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (ts testnetSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}

func (ts testnetSchedule) MaxTxAmountLimit() *big.Int {
	amountBigInt := big.NewInt(testnetMaxTxAmountLimit)
	amountBigInt = amountBigInt.Mul(amountBigInt, big.NewInt(denominations.One))
	return amountBigInt
}

func (ts testnetSchedule) MaxNumRecentTxsPerAccountLimit() uint64 {
	return testnetMaxNumRecentTxsPerAccountLimit
}

func (ts testnetSchedule) MaxTxPoolSizeLimit() int {
	return testnetMaxTxPoolSizeLimit
}

func (ts testnetSchedule) MaxNumTxsPerBlockLimit() int {
	return testnetMaxNumTxsPerBlockLimit
}

func (ts testnetSchedule) RecentTxDuration() time.Duration {
	return testnetRecentTxDuration
}

func (ts testnetSchedule) EnableTxnThrottling() bool {
	return testnetEnableTxnThrottling
}

func (ts testnetSchedule) TxsThrottleConfig() *TxsThrottleConfig {
	return &TxsThrottleConfig{
		MaxTxAmountLimit:               ts.MaxTxAmountLimit(),
		MaxNumRecentTxsPerAccountLimit: ts.MaxNumRecentTxsPerAccountLimit(),
		MaxTxPoolSizeLimit:             ts.MaxTxPoolSizeLimit(),
		MaxNumTxsPerBlockLimit:         ts.MaxNumTxsPerBlockLimit(),
		RecentTxDuration:               ts.RecentTxDuration(),
		EnableTxnThrottling:            ts.EnableTxnThrottling(),
	}
}

func (ts testnetSchedule) GetNetworkID() NetworkID {
	return TestNet
}

// GetShardingStructure is the sharding structure for testnet.
func (ts testnetSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, TestNetHTTPPattern, TestNetWSPattern)
}

var testnetReshardingEpoch = []*big.Int{big.NewInt(0), big.NewInt(testnetV1Epoch), big.NewInt(testnetV2Epoch)}

var testnetV0 = MustNewInstance(2, 150, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
var testnetV1 = MustNewInstance(2, 160, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
var testnetV2 = MustNewInstance(2, 170, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
