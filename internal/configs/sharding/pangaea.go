package shardingconfig

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/common/denominations"

	"github.com/harmony-one/harmony/internal/genesis"
)

const (
	// PangaeaHTTPPattern is the http pattern for pangaea.
	PangaeaHTTPPattern = "http://s%d.pga.hmny.io:9500"
	// PangaeaWSPattern is the websocket pattern for pangaea.
	PangaeaWSPattern = "ws://s%d.pga.hmny.io:9800"
)

// PangaeaSchedule is the Pangaea sharding configuration schedule.
var PangaeaSchedule pangaeaSchedule

type pangaeaSchedule struct{}

func (pangaeaSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	return pangaeaV0
}

func (pangaeaSchedule) BlocksPerEpoch() uint64 {
	return 10800 // 1 day with 8 seconds/block
}

func (ps pangaeaSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	return big.NewInt(int64(blockNum / ps.BlocksPerEpoch()))
}

func (ps pangaeaSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ps.BlocksPerEpoch() == 0
}

func (pangaeaSchedule) VdfDifficulty() int {
	return testnetVdfDifficulty
}

func (pangaeaSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

var pangaeaReshardingEpoch = []*big.Int{common.Big0}

var pangaeaV0 = MustNewInstance(
	4, 250, 20, genesis.PangaeaAccounts, genesis.FoundationalPangaeaAccounts, pangaeaReshardingEpoch)

func (pangaeaSchedule) FirstCrossLinkBlock() uint64 {
	return testnetFirstCrossLinkBlock
}

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (pangaeaSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}

func (pangaeaSchedule) MaxTxAmountLimit() *big.Int {
	amountBigInt := big.NewInt(mainnetMaxTxAmountLimit)
	amountBigInt = amountBigInt.Mul(amountBigInt, big.NewInt(denominations.One))
	return amountBigInt
}

func (pangaeaSchedule) MaxNumRecentTxsPerAccountLimit() uint64 {
	return mainnetMaxNumRecentTxsPerAccountLimit
}

func (pangaeaSchedule) MaxTxPoolSizeLimit() int {
	return mainnetMaxTxPoolSizeLimit
}

func (pangaeaSchedule) MaxNumTxsPerBlockLimit() int {
	return mainnetMaxNumTxsPerBlockLimit
}

func (pangaeaSchedule) RecentTxDuration() time.Duration {
	return mainnetRecentTxDuration
}

func (ps pangaeaSchedule) TxsThrottleConfig() *TxsThrottleConfig {
	return &TxsThrottleConfig{
		MaxTxAmountLimit:               ps.MaxTxAmountLimit(),
		MaxNumRecentTxsPerAccountLimit: ps.MaxNumRecentTxsPerAccountLimit(),
		MaxTxPoolSizeLimit:             ps.MaxTxPoolSizeLimit(),
		MaxNumTxsPerBlockLimit:         ps.MaxNumTxsPerBlockLimit(),
		RecentTxDuration:               ps.RecentTxDuration(),
	}
}

func (pangaeaSchedule) GetNetworkID() NetworkID {
	return Pangaea
}

// GetShardingStructure is the sharding structure for mainnet.
func (pangaeaSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, PangaeaHTTPPattern, PangaeaWSPattern)
}
