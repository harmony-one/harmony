package quorum

import (
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
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
	stakes := make([]numeric.Dec, len(v.validatorStakes))
	for _, s := range v.validatorStakes {
		stakes = append(stakes, s)
	}
	mStake := effective.Median(stakes)

	for validatorKey, stakedAmount := range v.validatorStakes {
		rStake := f(validatorKey)
		eStake := effective.Stake(mStake, rStake)
		stakedAmount.Set(eStake.Int)
	}
}

func (v *stakedVoteWeight) ToggleActive(*bls.PublicKey) bool {

	return true
}
