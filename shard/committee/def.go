package committee

import (
	"math/big"

	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/shard"
)

//
type ShardCommitteeProvider interface {
	//
}

// Assigner provides the next committee
type Assigner interface {
	NextCommittee(
		config shardingconfig.Instance,
		newEpoch *big.Int,
		previous shard.SuperCommittee,
	) shard.SuperCommittee
	InitCommittee(s shardingconfig.Instance) shard.SuperCommittee
	IsInitEpoch(epoch *big.Int) bool
}

var (
	// GenesisAssigner ..
	GenesisAssigner Assigner = genesisPolicy{}
	// MemberAssigner ..
	MemberAssigner Assigner = mixedPolicy{}
)
