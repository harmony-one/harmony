// Package shardingconfig defines types and utilities that deal with Harmony
// sharding configuration schedule.
package shardingconfig

import (
	"math/big"

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

	// VDFDifficulty returns number of iterations for VDF calculation
	VdfDifficulty() int

	// ConsensusRatio ratio of new nodes vs consensus total nodes
	ConsensusRatio() float64

	// TODO: remove it after randomness feature turned on mainnet
	//RandomnessStartingEpoch returns starting epoch of randonness generation
	RandomnessStartingEpoch() uint64

	// FirstCrossLinkBlock returns the first cross link block number that will be accepted into beacon chain
	FirstCrossLinkBlock() uint64
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
