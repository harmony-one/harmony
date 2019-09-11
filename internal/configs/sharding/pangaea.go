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
	PangaeaHTTPPattern = "https://api.s%d.pga.hmny.io"
	// PangaeaWSPattern is the websocket pattern for pangaea.
	PangaeaWSPattern = "wss://ws.s%d.pga.hmny.io"
	// transaction throttling disabled on pangaea network
	pangaeaEnableTxnThrottling = false
)

// PangaeaSchedule is the Pangaea sharding configuration schedule.
var PangaeaSchedule pangaeaSchedule

type pangaeaSchedule struct{}

func (ps pangaeaSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	return pangaeaV0
}

func (ps pangaeaSchedule) BlocksPerEpoch() uint64 {
	return 2700 // 6 hours with 8 seconds/block
}

func (ps pangaeaSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	return big.NewInt(int64(blockNum / ps.BlocksPerEpoch()))
}

func (ps pangaeaSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ps.BlocksPerEpoch() == 0
}

func (ps pangaeaSchedule) EpochLastBlock(epochNum uint64) uint64 {
	blocks := ps.BlocksPerEpoch()
	return blocks*(epochNum+1) - 1
}

func (ps pangaeaSchedule) VdfDifficulty() int {
	return testnetVdfDifficulty
}

func (ps pangaeaSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

var pangaeaReshardingEpoch = []*big.Int{common.Big0}

var pangaeaV0 = MustNewInstance(
	4, 250, 20, genesis.PangaeaAccounts, genesis.FoundationalPangaeaAccounts, pangaeaReshardingEpoch)

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (ps pangaeaSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}

func (ps pangaeaSchedule) MaxTxAmountLimit() *big.Int {
	amountBigInt := big.NewInt(mainnetMaxTxAmountLimit)
	amountBigInt = amountBigInt.Mul(amountBigInt, big.NewInt(denominations.One))
	return amountBigInt
}

func (ps pangaeaSchedule) MaxNumRecentTxsPerAccountLimit() uint64 {
	return mainnetMaxNumRecentTxsPerAccountLimit
}

func (ps pangaeaSchedule) MaxTxPoolSizeLimit() int {
	return mainnetMaxTxPoolSizeLimit
}

func (ps pangaeaSchedule) MaxNumTxsPerBlockLimit() int {
	return mainnetMaxNumTxsPerBlockLimit
}

func (ps pangaeaSchedule) RecentTxDuration() time.Duration {
	return mainnetRecentTxDuration
}

func (ps pangaeaSchedule) EnableTxnThrottling() bool {
	return pangaeaEnableTxnThrottling
}

func (ps pangaeaSchedule) TxsThrottleConfig() *TxsThrottleConfig {
	return &TxsThrottleConfig{
		MaxTxAmountLimit:               ps.MaxTxAmountLimit(),
		MaxNumRecentTxsPerAccountLimit: ps.MaxNumRecentTxsPerAccountLimit(),
		MaxTxPoolSizeLimit:             ps.MaxTxPoolSizeLimit(),
		MaxNumTxsPerBlockLimit:         ps.MaxNumTxsPerBlockLimit(),
		RecentTxDuration:               ps.RecentTxDuration(),
		EnableTxnThrottling:            ps.EnableTxnThrottling(),
	}
}

func (pangaeaSchedule) GetNetworkID() NetworkID {
	return Pangaea
}

// GetShardingStructure is the sharding structure for mainnet.
func (pangaeaSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, PangaeaHTTPPattern, PangaeaWSPattern)
}
