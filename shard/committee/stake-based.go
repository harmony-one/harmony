package committee

import (
	"fmt"

	"github.com/harmony-one/harmony/consensus/quorum"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/shard"
)

type mixedPolicy struct{}

func (mixedPolicy) NextCommittee(s shardingconfig.Instance) shard.SuperCommittee {
	switch s.QuorumPolicy() {
	case quorum.SuperMajorityVote:
		fmt.Println("called mixed policy assigner for super majority based")
		return GenesisAssigner.NextCommittee(s)
	case quorum.SuperMajorityStake:
		return nil
	}
	return nil
}
