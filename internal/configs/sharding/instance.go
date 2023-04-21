package shardingconfig

import (
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"
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
	slotsLimit                      int // HIP-16: The absolute number of maximum effective slots per shard limit for each validator. 0 means no limit.
	allowlist                       Allowlist
	feeCollectors                   FeeCollectors
}

type FeeCollectors map[ethCommon.Address]numeric.Dec

// NewInstance creates and validates a new sharding configuration based
// upon given parameters.
func NewInstance(
	numShards uint32, numNodesPerShard, numHarmonyOperatedNodesPerShard, slotsLimit int, harmonyVotePercent numeric.Dec,
	hmyAccounts []genesis.DeployAccount,
	fnAccounts []genesis.DeployAccount,
	allowlist Allowlist,
	feeCollectors FeeCollectors,
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
	if slotsLimit < 0 {
		return nil, errors.Errorf("SlotsLimit cannot be negative %d", slotsLimit)
	}
	if harmonyVotePercent.LT(numeric.ZeroDec()) ||
		harmonyVotePercent.GT(numeric.OneDec()) {
		return nil, errors.Errorf("" +
			"total voting power of harmony nodes should be within [0, 1]",
		)
	}
	if len(feeCollectors) > 0 {
		total := numeric.ZeroDec() // is a copy
		for _, v := range feeCollectors {
			total = total.Add(v)
		}
		if !total.Equal(numeric.OneDec()) {
			return nil, errors.Errorf(
				"total fee collection percentage should be 1, but got %v", total,
			)
		}
	}

	return instance{
		numShards:                       numShards,
		numNodesPerShard:                numNodesPerShard,
		numHarmonyOperatedNodesPerShard: numHarmonyOperatedNodesPerShard,
		harmonyVotePercent:              harmonyVotePercent,
		externalVotePercent:             numeric.OneDec().Sub(harmonyVotePercent),
		hmyAccounts:                     hmyAccounts,
		fnAccounts:                      fnAccounts,
		allowlist:                       allowlist,
		reshardingEpoch:                 reshardingEpoch,
		blocksPerEpoch:                  blocksE,
		slotsLimit:                      slotsLimit,
		feeCollectors:                   feeCollectors,
	}, nil
}

// MustNewInstance creates a new sharding configuration based upon
// given parameters.  It panics if parameter validation fails.
// It is intended to be used for static initialization.
func MustNewInstance(
	numShards uint32,
	numNodesPerShard, numHarmonyOperatedNodesPerShard int, slotsLimitPercent float32,
	harmonyVotePercent numeric.Dec,
	hmyAccounts []genesis.DeployAccount,
	fnAccounts []genesis.DeployAccount,
	allowlist Allowlist,
	feeCollectors FeeCollectors,
	reshardingEpoch []*big.Int, blocksPerEpoch uint64,
) Instance {
	slotsLimit := int(float32(numNodesPerShard-numHarmonyOperatedNodesPerShard) * slotsLimitPercent)
	sc, err := NewInstance(
		numShards, numNodesPerShard, numHarmonyOperatedNodesPerShard, slotsLimit, harmonyVotePercent,
		hmyAccounts, fnAccounts, allowlist, feeCollectors, reshardingEpoch, blocksPerEpoch,
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

// SlotsLimit returns the max slots per shard limit for each validator
func (sc instance) SlotsLimit() int {
	return sc.slotsLimit
}

// FeeCollector returns a mapping of address to decimal % of fee
func (sc instance) FeeCollectors() FeeCollectors {
	return sc.feeCollectors
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

// ExternalAllowlist returns the list of external leader keys in allowlist(HIP18)
func (sc instance) ExternalAllowlist() []bls.PublicKeyWrapper {
	return sc.allowlist.BLSPublicKeys
}

// ExternalAllowlistLimit returns the maximum number of external leader keys on each shard
func (sc instance) ExternalAllowlistLimit() int {
	return sc.allowlist.MaxLimitPerShard
}
