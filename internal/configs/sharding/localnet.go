package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
)

// LocalnetSchedule is the local testnet sharding
// configuration schedule.
var LocalnetSchedule localnetSchedule

type localnetSchedule struct{}

const (
	localnetV1Epoch = 1
	localnetV2Epoch = 2

	localnetEpochBlock1 = 10
	twoOne              = 5

	localnetVdfDifficulty = 5000 // This takes about 10s to finish the vdf

	localnetMaxTxAmountLimit             = 1000
	localnetMaxTxsPerAccountInBlockLimit = 100
	localnetMaxTxsPerBlockLimit          = 8000
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

func (localnetSchedule) BlocksPerEpoch() uint64 {
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

func (ls localnetSchedule) VdfDifficulty() int {
	return localnetVdfDifficulty
}

func (ls localnetSchedule) MaxTxAmountLimit() *big.Int {
	return big.NewInt(localnetMaxTxAmountLimit)
}

func (ls localnetSchedule) MaxTxsPerAccountInBlockLimit() uint64 {
	return localnetMaxTxsPerAccountInBlockLimit
}

func (ls localnetSchedule) MaxTxsPerBlockLimit() int {
	return localnetMaxTxsPerBlockLimit
}

func (ls localnetSchedule) TxsThrottleConfig() *TxsThrottleConfig {
	return &TxsThrottleConfig{
		MaxTxAmountLimit:             big.NewInt(localnetMaxTxAmountLimit),
		MaxTxsPerAccountInBlockLimit: localnetMaxTxsPerAccountInBlockLimit,
		MaxTxsPerBlockLimit:          localnetMaxTxsPerBlockLimit,
	}
}

var localnetReshardingEpoch = []*big.Int{big.NewInt(0), big.NewInt(localnetV1Epoch), big.NewInt(localnetV2Epoch)}

var localnetV0 = MustNewInstance(2, 7, 5, genesis.LocalHarmonyAccounts, genesis.LocalFnAccounts, localnetReshardingEpoch)
var localnetV1 = MustNewInstance(2, 7, 5, genesis.LocalHarmonyAccountsV1, genesis.LocalFnAccountsV1, localnetReshardingEpoch)
var localnetV2 = MustNewInstance(2, 10, 4, genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2, localnetReshardingEpoch)
