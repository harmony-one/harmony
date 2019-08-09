package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/common/denominations"
)

const (
	// VLBPE is a Very Large Block Per Epoch
	VLBPE = 1000000000000
)

type fixedSchedule struct {
	instance Instance
}

// InstanceForEpoch returns the fixed sharding configuration instance regardless
// the given epoch.
func (s fixedSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	return s.instance
}

func (s fixedSchedule) BlocksPerEpoch() uint64 {
	return VLBPE
}

func (s fixedSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	return big.NewInt(int64(blockNum / s.BlocksPerEpoch()))
}

func (s fixedSchedule) IsLastBlock(blockNum uint64) bool {
	blocks := s.BlocksPerEpoch()
	return blockNum%blocks == blocks-1
}

func (s fixedSchedule) MaxTxAmountLimit() *big.Int {
	amountBigInt := big.NewInt(int64(mainnetMaxTxAmountLimit * denominations.Nano))
	amountBigInt = amountBigInt.Mul(amountBigInt, big.NewInt(denominations.Nano))
	return amountBigInt
}

func (s fixedSchedule) MaxTxsPerAccountInBlockLimit() uint64 {
	return mainnetMaxTxsPerAccountInBlockLimit
}

func (s fixedSchedule) MaxTxsPerBlockLimit() int {
	return mainnetMaxTxsPerBlockLimit
}

func (s fixedSchedule) TxsThrottleConfig() *TxsThrottleConfig {
	return &TxsThrottleConfig{
		MaxTxAmountLimit:             s.MaxTxAmountLimit(),
		MaxTxsPerAccountInBlockLimit: s.MaxTxsPerAccountInBlockLimit(),
		MaxTxsPerBlockLimit:          s.MaxTxsPerBlockLimit(),
	}
}

// NewFixedSchedule returns a sharding configuration schedule that uses the
// given config instance for all epochs.  Useful for testing.
func NewFixedSchedule(instance Instance) Schedule {
	return fixedSchedule{instance: instance}
}
