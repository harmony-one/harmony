package quorum

import (
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/numeric"
)

type stakedVoteWeight struct {
	SignatureReader
	// EPOS based staking
	validatorStakes   map[*bls.PublicKey]numeric.Dec
	totalStakedAmount numeric.Dec
}

// Policy ..
func (v *stakedVoteWeight) Policy() Policy {
	return SuperMajorityStake
}

// IsQuorumAchieved ..
func (v *stakedVoteWeight) IsQuorumAchieved(p Phase) bool {
	return false
}

// QuorumThreshold ..
func (v *stakedVoteWeight) QuorumThreshold() int64 {
	return 100000
}

// RewardThreshold ..
func (v *stakedVoteWeight) IsRewardThresholdAchieved() bool {
	return false
}

func (v *stakedVoteWeight) UpdateVotingPower(f func(*bls.PublicKey) numeric.Dec) {
	for validatorKey, stakedAmount := range v.validatorStakes {
		newStake := f(validatorKey)
		stakedAmount.Set(numeric.NewDecFromBigInt(newStake.Int).Int)
	}
}
