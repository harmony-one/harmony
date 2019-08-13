package shardingconfig

import (
	"math/big"
	"time"

	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/internal/genesis"
)

// LocalnetSchedule is the local testnet sharding
// configuration schedule.
var LocalnetSchedule localnetSchedule

type localnetSchedule struct{}

const (
	localnetV1Epoch = 1
	localnetV2Epoch = 2

	localnetEpochBlock1 = 20
	twoOne              = 5

	localnetMaxTxAmountLimit               = 1e2 // unit is in One
	localnetMaxNumRecentTxsPerAccountLimit = 2
	localnetMaxTxPoolSizeLimit             = 8000
	localnetMaxNumTxsPerBlockLimit         = 1000
	localnetRecentTxDuration               = 100 * time.Second
)

func (localnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(big.NewInt(localnetV2Epoch)) >= 0:
		return localnetV2
	case epoch.Cmp(big.NewInt(localnetV1Epoch)) >= 0:
		return localnetV1
	default: // genesis
		return localnetV0
	}
}

func (ls localnetSchedule) BlocksPerEpoch() uint64 {
	return twoOne
}

func (ls localnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	blocks := ls.BlocksPerEpoch()
	switch {
	case blockNum >= localnetEpochBlock1:
		return big.NewInt(int64((blockNum-localnetEpochBlock1)/blocks) + 1)
	default:
		return big.NewInt(0)
	}
}

func (ls localnetSchedule) IsLastBlock(blockNum uint64) bool {
	blocks := ls.BlocksPerEpoch()
	switch {
	case blockNum < localnetEpochBlock1-1:
		return false
	case blockNum == localnetEpochBlock1-1:
		return true
	default:
		return ((blockNum-localnetEpochBlock1)%blocks == blocks-1)
	}
}

func (ls localnetSchedule) MaxTxAmountLimit() *big.Int {
	amountBigInt := big.NewInt(localnetMaxTxAmountLimit)
	amountBigInt = amountBigInt.Mul(amountBigInt, big.NewInt(denominations.One))
	return amountBigInt
}

func (ls localnetSchedule) MaxNumRecentTxsPerAccountLimit() uint64 {
	return localnetMaxNumRecentTxsPerAccountLimit
}

func (ls localnetSchedule) MaxTxPoolSizeLimit() int {
	return localnetMaxTxPoolSizeLimit
}

func (ls localnetSchedule) MaxNumTxsPerBlockLimit() int {
	return localnetMaxNumTxsPerBlockLimit
}

func (ls localnetSchedule) RecentTxDuration() time.Duration {
	return localnetRecentTxDuration
}

func (ls localnetSchedule) TxsThrottleConfig() *TxsThrottleConfig {
	return &TxsThrottleConfig{
		MaxTxAmountLimit:               ls.MaxTxAmountLimit(),
		MaxNumRecentTxsPerAccountLimit: ls.MaxNumRecentTxsPerAccountLimit(),
		MaxTxPoolSizeLimit:             ls.MaxTxPoolSizeLimit(),
		MaxNumTxsPerBlockLimit:         ls.MaxNumTxsPerBlockLimit(),
		RecentTxDuration:               ls.RecentTxDuration(),
	}
}

var localnetReshardingEpoch = []*big.Int{big.NewInt(0), big.NewInt(localnetV1Epoch), big.NewInt(localnetV2Epoch)}

var localnetV0 = MustNewInstance(2, 7, 5, genesis.LocalHarmonyAccounts, genesis.LocalFnAccounts, localnetReshardingEpoch)
var localnetV1 = MustNewInstance(2, 7, 5, genesis.LocalHarmonyAccountsV1, genesis.LocalFnAccountsV1, localnetReshardingEpoch)
var localnetV2 = MustNewInstance(2, 10, 4, genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2, localnetReshardingEpoch)
