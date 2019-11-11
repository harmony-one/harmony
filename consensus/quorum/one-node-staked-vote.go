package quorum

import (
	"math/big"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

var (
	twoThirds = numeric.NewDec(2).QuoInt64(3).Int
)

type stakedVoter struct {
	isActive, isHarmonyNode bool
	effective               numeric.Dec
}

type stakedVoteWeight struct {
	SignatureReader
	DependencyInjectionWriter
	// EPOS based staking
	validatorStakes            map[[shard.PublicKeySizeInBytes]byte]stakedVoter
	totalEffectiveStakedAmount *big.Int
}

// Policy ..
func (v *stakedVoteWeight) Policy() Policy {
	return SuperMajorityStake
}

// We must maintain 2/3 quoroum, so whatever is 2/3 staked amount,
// we divide that out & you
// IsQuorumAchieved ..
func (v *stakedVoteWeight) IsQuorumAchieved(p Phase) bool {
	// TODO Implement this logic
	return true
}

// QuorumThreshold ..
func (v *stakedVoteWeight) QuorumThreshold() *big.Int {
	return new(big.Int).Mul(v.totalEffectiveStakedAmount, twoThirds)
}

// RewardThreshold ..
func (v *stakedVoteWeight) IsRewardThresholdAchieved() bool {
	// TODO Implement
	return false
}

// HACK
var (
	hSentinel          = big.NewInt(0)
	hEffectiveSentinel = numeric.ZeroDec()
)

// Award ..
func (v *stakedVoteWeight) Award(
	Pie *big.Int, earners []common.Address, hook func(earner common.Address, due *big.Int),
) *big.Int {
	// TODO Implement
	return nil
}

// UpdateVotingPower called only at epoch change, prob need to move to CalculateShardState
// func (v *stakedVoteWeight) UpdateVotingPower(keeper effective.StakeKeeper) {
// TODO Implement
// }

func (v *stakedVoteWeight) ToggleActive(*bls.PublicKey) bool {
	// TODO Implement
	return true
}
