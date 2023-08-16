// Package shardingconfig defines types and utilities that deal with Harmony
// sharding configuration schedule.
package shardingconfig

import (
	"fmt"
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
)

// Schedule returns the sharding configuration instance for the given
// epoch.
type Schedule interface {
	InstanceForEpoch(epoch *big.Int) Instance

	// CalcEpochNumber returns the epoch number based on the block number
	CalcEpochNumber(blockNum uint64) *big.Int

	// IsLastBlock check if the block is the last block in the epoch
	// NOTE: This method is very critical for the epoch transition logic and other checks.
	IsLastBlock(blockNum uint64) bool

	// EpochLastBlock returns the last block number of an epoch
	// NOTE: This method id important for a few rpcs and validator APR calculation
	EpochLastBlock(epochNum uint64) uint64

	// VDFDifficulty returns number of iterations for VDF calculation
	VdfDifficulty() int

	// GetNetworkID() return networkID type.
	GetNetworkID() NetworkID

	// GetShardingStructure returns sharding structure.
	GetShardingStructure(int, int) []map[string]interface{}

	// IsSkippedEpoch returns if epoch was skipped on shard chain
	IsSkippedEpoch(uint32, *big.Int) bool
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

	// HarmonyVotePercent returns total percentage of voting power harmony nodes possess.
	HarmonyVotePercent() numeric.Dec

	// ExternalVotePercent returns total percentage of voting power external validators possess.
	ExternalVotePercent() numeric.Dec

	// HmyAccounts returns a list of Harmony accounts
	HmyAccounts() []genesis.DeployAccount

	// FnAccounts returns a list of Foundational node accounts
	FnAccounts() []genesis.DeployAccount

	// FindAccount returns the deploy account based on the blskey
	FindAccount(blsPubKey string) (bool, *genesis.DeployAccount)

	// ReshardingEpoch returns a list of Epoch while off-chain resharding happens
	ReshardingEpoch() []*big.Int

	// BlocksPerEpoch returns the number of blocks per epoch.
	BlocksPerEpoch() uint64

	// HIP-16: The absolute number of maximum effective slots per shard limit for each validator. 0 means no limit.
	SlotsLimit() int

	// ExternalAllowlist returns the list of external leader keys in allowlist(HIP18)
	ExternalAllowlist() []bls.PublicKeyWrapper

	// ExternalAllowlistLimit returns the maximum number of external leader keys on each shard(HIP18)
	ExternalAllowlistLimit() int

	// FeeCollector returns a mapping of address to decimal % of fee
	FeeCollectors() FeeCollectors

	// HIP30RecoveryAddress returns the address to which
	// HIP30EmissionSplit % income is sent
	HIP30RecoveryAddress() ethCommon.Address

	// HIP30EmissionFraction is the percentage of the emission
	// sent to the Recovery Address
	HIP30EmissionFraction() numeric.Dec
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
