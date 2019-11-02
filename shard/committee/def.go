package committee

import (
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/shard"
)

// Assigner provides the next committee
type Assigner interface {
	NextCommittee(shardingconfig.Instance) shard.SuperCommittee
}

var (
	// GenesisAssigner ..
	GenesisAssigner Assigner = genesisPolicy{}
	// MemberAssigner ..
	MemberAssigner Assigner = mixedPolicy{}
)
