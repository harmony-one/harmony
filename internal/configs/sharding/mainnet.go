package shardingconfig

import (
	"math/big"
	"time"

	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/internal/genesis"
)

const (
	mainnetEpochBlock1 = 344064 // 21 * 2^14
	blocksPerShard     = 16384  // 2^14
	mainnetV0_1Epoch   = 1
	mainnetV0_2Epoch   = 5
	mainnetV0_3Epoch   = 8
	mainnetV0_4Epoch   = 10
	mainnetV1Epoch     = 12

	mainnetMaxTxAmountLimit               = 1e3 // unit is interface{} One
	mainnetMaxNumRecentTxsPerAccountLimit = 1e2
	mainnetMaxTxPoolSizeLimit             = 8000
	mainnetMaxNumTxsPerBlockLimit         = 1000
	mainnetRecentTxDuration               = time.Hour
)

// MainnetSchedule is the mainnet sharding configuration schedule.
var MainnetSchedule mainnetSchedule

type mainnetSchedule struct{}

func (mainnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(big.NewInt(mainnetV1Epoch)) >= 0:
		// twelfth resharding epoch around 08/16/2019 11:00pm PDT
		return mainnetV1
	case epoch.Cmp(big.NewInt(mainnetV0_4Epoch)) >= 0:
		// tenth resharding epoch around 08/13/2019 9:00pm PDT
		return mainnetV0_4
	case epoch.Cmp(big.NewInt(mainnetV0_3Epoch)) >= 0:
		// eighth resharding epoch around 08/10/2019 6:00pm PDT
		return mainnetV0_3
	case epoch.Cmp(big.NewInt(mainnetV0_2Epoch)) >= 0:
		// fifth resharding epoch around 08/06/2019 2:30am PDT
		return mainnetV0_2
	case epoch.Cmp(big.NewInt(mainnetV0_1Epoch)) >= 0:
		// first resharding epoch around 07/30/2019 10:30pm PDT
		return mainnetV0_1
	default: // genesis
		return mainnetV0
	}
}

func (mainnetSchedule) BlocksPerEpoch() uint64 {
	return blocksPerShard
}

func (ms mainnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	blocks := ms.BlocksPerEpoch()
	switch {
	case blockNum >= mainnetEpochBlock1:
		return big.NewInt(int64((blockNum-mainnetEpochBlock1)/blocks) + 1)
	default:
		return big.NewInt(0)
	}
}

func (ms mainnetSchedule) IsLastBlock(blockNum uint64) bool {
	blocks := ms.BlocksPerEpoch()
	switch {
	case blockNum < mainnetEpochBlock1-1:
		return false
	case blockNum == mainnetEpochBlock1-1:
		return true
	default:
		return ((blockNum-mainnetEpochBlock1)%blocks == blocks-1)
	}
}

func (ms mainnetSchedule) MaxTxAmountLimit() *big.Int {
	amountBigInt := big.NewInt(mainnetMaxTxAmountLimit)
	amountBigInt = amountBigInt.Mul(amountBigInt, big.NewInt(denominations.One))
	return amountBigInt
}

func (ms mainnetSchedule) MaxNumRecentTxsPerAccountLimit() uint64 {
	return mainnetMaxNumRecentTxsPerAccountLimit
}

func (ms mainnetSchedule) MaxTxPoolSizeLimit() int {
	return mainnetMaxTxPoolSizeLimit
}

func (ms mainnetSchedule) MaxNumTxsPerBlockLimit() int {
	return mainnetMaxNumTxsPerBlockLimit
}

func (ms mainnetSchedule) RecentTxDuration() time.Duration {
	return mainnetRecentTxDuration
}

func (ms mainnetSchedule) TxsThrottleConfig() *TxsThrottleConfig {
	return &TxsThrottleConfig{
		MaxTxAmountLimit:               ms.MaxTxAmountLimit(),
		MaxNumRecentTxsPerAccountLimit: ms.MaxNumRecentTxsPerAccountLimit(),
		MaxTxPoolSizeLimit:             ms.MaxTxPoolSizeLimit(),
		MaxNumTxsPerBlockLimit:         ms.MaxNumTxsPerBlockLimit(),
		RecentTxDuration:               ms.RecentTxDuration(),
	}
}

var mainnetReshardingEpoch = []*big.Int{big.NewInt(0), big.NewInt(mainnetV0_1Epoch), big.NewInt(mainnetV0_2Epoch), big.NewInt(mainnetV0_3Epoch), big.NewInt(mainnetV0_4Epoch), big.NewInt(mainnetV1Epoch)}

var mainnetV0 = MustNewInstance(4, 150, 112, genesis.HarmonyAccounts, genesis.FoundationalNodeAccounts, mainnetReshardingEpoch)
var mainnetV0_1 = MustNewInstance(4, 152, 112, genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV0_1, mainnetReshardingEpoch)
var mainnetV0_2 = MustNewInstance(4, 200, 148, genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV0_2, mainnetReshardingEpoch)
var mainnetV0_3 = MustNewInstance(4, 210, 148, genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV0_3, mainnetReshardingEpoch)
var mainnetV0_4 = MustNewInstance(4, 216, 148, genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV0_4, mainnetReshardingEpoch)
var mainnetV1 = MustNewInstance(4, 250, 170, genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV1, mainnetReshardingEpoch)

//var mainnetV1 = MustNewInstance(4, 400, 280)
//var mainnetV2 = MustNewInstance(6, 400, 199)
