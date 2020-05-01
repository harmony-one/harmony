package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/numeric"
	"github.com/pkg/errors"
)

// NetworkID is the network type of the blockchain.
type NetworkID byte

// Constants for NetworkID.
const (
	MainNet NetworkID = iota
	TestNet
	LocalNet
	Pangaea
	Partner
	StressNet
	DevNet
)

type instance struct {
	numShards                       uint32
	numNodesPerShard                int
	numHarmonyOperatedNodesPerShard int
	harmonyVotePercent              numeric.Dec
	externalVotePercent             numeric.Dec
	hmyAccounts                     []genesis.DeployAccount
	fnAccounts                      []genesis.DeployAccount
	reshardingEpoch                 []*big.Int
	blocksPerEpoch                  uint64
}

// NewInstance creates and validates a new sharding configuration based
// upon given parameters.
func NewInstance(
	numShards uint32, numNodesPerShard, numHarmonyOperatedNodesPerShard int, harmonyVotePercent numeric.Dec,
	hmyAccounts []genesis.DeployAccount,
	fnAccounts []genesis.DeployAccount,
	reshardingEpoch []*big.Int, blocksE uint64,
) (Instance, error) {
	if numShards < 1 {
		return nil, errors.Errorf(
			"sharding config must have at least one shard have %d", numShards,
		)
	}
	if numNodesPerShard < 1 {
		return nil, errors.Errorf(
			"each shard must have at least one node %d", numNodesPerShard,
		)
	}
	if numHarmonyOperatedNodesPerShard < 0 {
		return nil, errors.Errorf(
			"Harmony-operated nodes cannot be negative %d", numHarmonyOperatedNodesPerShard,
		)
	}
	if numHarmonyOperatedNodesPerShard > numNodesPerShard {
		return nil, errors.Errorf(""+
			"number of Harmony-operated nodes cannot exceed "+
			"overall number of nodes per shard %d %d",
			numHarmonyOperatedNodesPerShard,
			numNodesPerShard,
		)
	}
	if harmonyVotePercent.LT(numeric.ZeroDec()) ||
		harmonyVotePercent.GT(numeric.OneDec()) {
		return nil, errors.Errorf("" +
			"total voting power of harmony nodes should be within [0, 1]",
		)
	}

	return instance{
		numShards:                       numShards,
		numNodesPerShard:                numNodesPerShard,
		numHarmonyOperatedNodesPerShard: numHarmonyOperatedNodesPerShard,
		harmonyVotePercent:              harmonyVotePercent,
		externalVotePercent:             numeric.OneDec().Sub(harmonyVotePercent),
		hmyAccounts:                     hmyAccounts,
		fnAccounts:                      fnAccounts,
		reshardingEpoch:                 reshardingEpoch,
		blocksPerEpoch:                  blocksE,
	}, nil
}

// MustNewInstance creates a new sharding configuration based upon
// given parameters.  It panics if parameter validation fails.
// It is intended to be used for static initialization.
func MustNewInstance(
	numShards uint32,
	numNodesPerShard, numHarmonyOperatedNodesPerShard int,
	harmonyVotePercent numeric.Dec,
	hmyAccounts []genesis.DeployAccount,
	fnAccounts []genesis.DeployAccount,
	reshardingEpoch []*big.Int, blocksPerEpoch uint64,
) Instance {
	sc, err := NewInstance(
		numShards, numNodesPerShard, numHarmonyOperatedNodesPerShard, harmonyVotePercent,
		hmyAccounts, fnAccounts, reshardingEpoch, blocksPerEpoch,
	)
	if err != nil {
		panic(err)
	}
	return sc
}

// BlocksPerEpoch ..
func (sc instance) BlocksPerEpoch() uint64 {
	return sc.blocksPerEpoch
}

// NumShards returns the number of shards in the network.
func (sc instance) NumShards() uint32 {
	return sc.numShards
}

// HarmonyVotePercent returns total percentage of voting power harmony nodes possess.
func (sc instance) HarmonyVotePercent() numeric.Dec {
	return sc.harmonyVotePercent
}

// ExternalVotePercent returns total percentage of voting power external validators possess.
func (sc instance) ExternalVotePercent() numeric.Dec {
	return sc.externalVotePercent
}

// NumNodesPerShard returns number of nodes in each shard.
func (sc instance) NumNodesPerShard() int {
	return sc.numNodesPerShard
}

// NumHarmonyOperatedNodesPerShard returns number of nodes in each shard
// that are operated by Harmony.
func (sc instance) NumHarmonyOperatedNodesPerShard() int {
	return sc.numHarmonyOperatedNodesPerShard
}

// HmyAccounts returns the list of Harmony accounts
func (sc instance) HmyAccounts() []genesis.DeployAccount {
	return sc.hmyAccounts
}

// FnAccounts returns the list of Foundational Node accounts
func (sc instance) FnAccounts() []genesis.DeployAccount {
	return sc.fnAccounts
}

// FindAccount returns the deploy account based on the blskey, and if the account is a leader
// or not in the bootstrapping process.
func (sc instance) FindAccount(blsPubKey string) (bool, *genesis.DeployAccount) {
	for i, item := range sc.hmyAccounts {
		if item.BLSPublicKey == blsPubKey {
			item.ShardID = uint32(i) % sc.numShards
			return uint32(i) < sc.numShards, &item
		}
	}
	for i, item := range sc.fnAccounts {
		if item.BLSPublicKey == blsPubKey {
			item.ShardID = uint32(i) % sc.numShards
			return false, &item
		}
	}
	return false, nil
}

// ReshardingEpoch returns the list of epoch number
func (sc instance) ReshardingEpoch() []*big.Int {
	return sc.reshardingEpoch
}

// ReshardingEpoch returns the list of epoch number
func (sc instance) GetNetworkID() NetworkID {
	return DevNet
}
