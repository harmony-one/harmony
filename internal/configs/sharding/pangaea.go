package shardingconfig

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/internal/genesis"
)

// PangaeaSchedule is the Pangaea sharding configuration schedule.
var PangaeaSchedule pangaeaSchedule

type pangaeaSchedule struct{}

func (ps pangaeaSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	return pangaeaV0
}

func (ps pangaeaSchedule) BlocksPerEpoch() uint64 {
	return 10800 // 1 day with 8 seconds/block
}

func (ps pangaeaSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	return big.NewInt(int64(blockNum / ps.BlocksPerEpoch()))
}

func (ps pangaeaSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ps.BlocksPerEpoch() == 0
}

func (ps pangaeaSchedule) VdfDifficulty() int {
	return testnetVdfDifficulty
}

func (ps pangaeaSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

func (ps pangaeaSchedule) MaxTxAmountLimit() *big.Int {
	amountBigInt := big.NewInt(int64(mainnetMaxTxAmountLimit * denominations.Nano))
	amountBigInt = amountBigInt.Mul(amountBigInt, big.NewInt(denominations.Nano))
	return amountBigInt
}

func (ps pangaeaSchedule) MaxTxsPerAccountInBlockLimit() uint64 {
	return mainnetMaxTxsPerAccountInBlockLimit
}

func (ps pangaeaSchedule) MaxTxsPerBlockLimit() int {
	return mainnetMaxTxsPerBlockLimit
}

func (ps pangaeaSchedule) TxsThrottleConfig() *TxsThrottleConfig {
	return &TxsThrottleConfig{
		MaxTxAmountLimit:             ps.MaxTxAmountLimit(),
		MaxTxsPerAccountInBlockLimit: ps.MaxTxsPerAccountInBlockLimit(),
		MaxTxsPerBlockLimit:          ps.MaxTxsPerBlockLimit(),
	}
}

var pangaeaReshardingEpoch = []*big.Int{common.Big0}

var pangaeaV0 = MustNewInstance(
	4, 200, 200, genesis.PangaeaAccounts, nil, pangaeaReshardingEpoch)
