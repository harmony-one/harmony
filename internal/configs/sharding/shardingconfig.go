// Package shardingconfig defines types and utilities that deal with Harmony
// sharding configuration schedule.
package shardingconfig

import (
	"fmt"
	"math/big"
	"time"

	"github.com/harmony-one/harmony/internal/genesis"
)

// Schedule returns the sharding configuration instance for the given
// epoch.
type Schedule interface {
	InstanceForEpoch(epoch *big.Int) Instance

	// BlocksPerEpoch returns the number of blocks per each Epoch
	BlocksPerEpoch() uint64

	// CalcEpochNumber returns the epoch number based on the block number
	CalcEpochNumber(blockNum uint64) *big.Int

	// IsLastBlock check if the block is the last block in the epoch
	IsLastBlock(blockNum uint64) bool

	// EpochLastBlock returns the last block number of an epoch
	EpochLastBlock(epochNum uint64) uint64

	// VDFDifficulty returns number of iterations for VDF calculation
	VdfDifficulty() int

	// ConsensusRatio ratio of new nodes vs consensus total nodes
	ConsensusRatio() float64

	// TODO: remove it after randomness feature turned on mainnet
	//RandomnessStartingEpoch returns starting epoch of randonness generation
	RandomnessStartingEpoch() uint64

	// Max amount limit for a valid transaction
	MaxTxAmountLimit() *big.Int

	// Max number of transactions of a particular account per block level
	MaxNumRecentTxsPerAccountLimit() uint64

	// Max total number of transactions allowed as pending transactions in transaction pool
	MaxTxPoolSizeLimit() int

	// Max total number of transactions allowed to be processed per block
	MaxNumTxsPerBlockLimit() int

	// How long "recent" means for transaction in time Duration unit
	RecentTxDuration() time.Duration

	// EnableTxnThrottling is the switch for transaction throttling
	EnableTxnThrottling() bool

	// configuration for throttling pending transactions
	TxsThrottleConfig() *TxsThrottleConfig

	// GetNetworkID() return networkID type.
	GetNetworkID() NetworkID

	// GetShardingStructure returns sharding structure.
	GetShardingStructure(int, int) []map[string]interface{}
}

// Instance is one sharding configuration instance.
type Instance interface {
	// NumShards returns the number of shards in the network.
	NumShards() uint32

	// NumNodesPerShard returns number of nodes in each shard.
	NumNodesPerShard() int

	// NumHarmonyOperatedNodesPerShard returns number of nodes in each shard
	// that are operated by Harmony.
	NumHarmonyOperatedNodesPerShard() int

	// HmyAccounts returns a list of Harmony accounts
	HmyAccounts() []genesis.DeployAccount

	// FnAccounts returns a list of Foundational node accounts
	FnAccounts() []genesis.DeployAccount

	// FindAccount returns the deploy account based on the blskey
	FindAccount(blsPubKey string) (bool, *genesis.DeployAccount)

	// ReshardingEpoch returns a list of Epoch while off-chain resharding happens
	ReshardingEpoch() []*big.Int
}

// TxThrottleFlag is the throttling flag for each transaction
// Refer below enum declaration for more context.
type TxThrottleFlag int

// TxThrottleFlag is determined per transaction
// during the new block proposal and pending transactions throttling
const (
	TxSelect TxThrottleFlag = iota
	TxUnselect
	TxInvalid
)

func (result TxThrottleFlag) String() string {
	switch result {
	case TxSelect:
		return "TxSelect"
	case TxUnselect:
		return "TxUnselect"
	case TxInvalid:
		return "TxInvalid"
	}
	return "TxThrottleUnknown"
}

// TxsThrottleConfig contains configuration for throttling pending transactions per node block
type TxsThrottleConfig struct {
	// Max amount limit for a valid transaction
	MaxTxAmountLimit *big.Int

	// Max number of transactions of a particular account for the past hour
	RecentTxDuration time.Duration

	// Max number of transactions of a particular account for the past hour
	MaxNumRecentTxsPerAccountLimit uint64

	// Max total number of transactions allowed as pending transactions in transaction pool
	MaxTxPoolSizeLimit int

	// Max total number of transactions allowed to be processed per block
	MaxNumTxsPerBlockLimit int

	// EnableTxnThrottling is the switch for transaction throttling
	EnableTxnThrottling bool
}

// genShardingStructure return sharding structure, given shard number and its patterns.
func genShardingStructure(shardNum, shardID int, httpPattern, wsPattern string) []map[string]interface{} {
	res := []map[string]interface{}{}
	for i := 0; i < shardNum; i++ {
		res = append(res, map[string]interface{}{
			"current": int(shardID) == i,
			"shardID": i,
			"http":    fmt.Sprintf(httpPattern, i),
			"ws":      fmt.Sprintf(wsPattern, i),
		})
	}
	return res
}
