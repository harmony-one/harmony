package quorum

import (
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
)

var (
	twoThirds = numeric.NewDec(2).QuoInt64(3)
)

type stakedVoteWeight struct {
	SignatureReader
	// EPOS based staking
	validatorStakes map[*bls.PublicKey]struct {
		isActive  bool
		effective numeric.Dec
	}
	totalEffectiveStakedAmount numeric.Dec
}

// Policy ..
func (v *stakedVoteWeight) Policy() Policy {
	return SuperMajorityStake
}

// IsQuorumAchieved ..
func (v *stakedVoteWeight) IsQuorumAchieved(p Phase) bool {
	return true
}

// QuorumThreshold ..
func (v *stakedVoteWeight) QuorumThreshold() int64 {
	return v.totalEffectiveStakedAmount.Mul(twoThirds).Int64()
}

// RewardThreshold ..
func (v *stakedVoteWeight) IsRewardThresholdAchieved() bool {
	return false
}

func (v *stakedVoteWeight) UpdateVotingPower(f func(*bls.PublicKey) numeric.Dec) {
	stakes := make([]numeric.Dec, len(v.validatorStakes))
	for _, s := range v.validatorStakes {
		stakes = append(stakes, s.effective)
	}
	mStake := effective.Median(stakes)

	for validatorKey, stakedAmount := range v.validatorStakes {
		rStake := f(validatorKey)
		eStake := effective.Stake(mStake, rStake)
		stakedAmount.effective.Set(eStake.Int)
	}
}

func (v *stakedVoteWeight) ToggleActive(*bls.PublicKey) bool {

	return true
}
