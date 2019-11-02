package committee

import (
	"math/big"

	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/shard"
)

type mixedPolicy struct{}

func (m mixedPolicy) NextCommittee(s shardingconfig.Instance, ignored *big.Int) shard.SuperCommittee {
	return m.InitCommittee(s)
}

// InitCommittee
// Only called once for the history of staking, the initial
// distribution when staking was turned on
func (m mixedPolicy) InitCommittee(s shardingconfig.Instance) shard.SuperCommittee {
	return nil
}

// func (mixedPolicy) NextCommittee(s shardingconfig.Instance) shard.SuperCommittee {
// 	switch s.QuorumPolicy() {
// 	case quorum.SuperMajorityVote:
// 		fmt.Println("called mixed policy assigner for super majority based")
// 		return GenesisAssigner.NextCommittee(s)
// 	case quorum.SuperMajorityStake:
// 		// 1) Pull top 400 stakers
// 		// 2)
// 		return nil
// 	default:
// 		// not possible
// 		return nil
// 	}
// }
