package shardingconfig

import (
	"math/big"

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

	testnetMaxTxAmountLimit               = 1e3 // unit is in One
	testnetMaxNumRecentTxsPerAccountLimit = 10
	testnetMaxTxsPerBlockLimit            = 8000
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

func (ts testnetSchedule) MaxTxAmountLimit() *big.Int {
	amountBigInt := big.NewInt(int64(testnetMaxTxAmountLimit * denominations.Nano))
	amountBigInt = amountBigInt.Mul(amountBigInt, big.NewInt(denominations.Nano))
	return amountBigInt
}

func (ts testnetSchedule) MaxNumRecentTxsPerAccountLimit() uint64 {
	return testnetMaxNumRecentTxsPerAccountLimit
}

func (ts testnetSchedule) MaxTxsPerBlockLimit() int {
	return testnetMaxTxsPerBlockLimit
}

func (ts testnetSchedule) TxsThrottleConfig() *TxsThrottleConfig {
	return &TxsThrottleConfig{
		MaxTxAmountLimit:               ts.MaxTxAmountLimit(),
		MaxNumRecentTxsPerAccountLimit: ts.MaxNumRecentTxsPerAccountLimit(),
		MaxTxsPerBlockLimit:            ts.MaxTxsPerBlockLimit(),
	}
}

var testnetReshardingEpoch = []*big.Int{big.NewInt(0), big.NewInt(testnetV1Epoch), big.NewInt(testnetV2Epoch)}

var testnetV0 = MustNewInstance(2, 150, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
var testnetV1 = MustNewInstance(2, 160, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
var testnetV2 = MustNewInstance(2, 170, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
