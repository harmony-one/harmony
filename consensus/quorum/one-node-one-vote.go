package quorum

import (
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/numeric"
)

type uniformVoteWeight struct {
	SignatureReader
}

// Policy ..
func (v *uniformVoteWeight) Policy() Policy {
	return SuperMajorityVote
}

// IsQuorumAchieved ..
func (v *uniformVoteWeight) IsQuorumAchieved(p Phase) bool {
	return v.SignatoriesCount(p) >= v.QuorumThreshold()
}

// QuorumThreshold ..
func (v *uniformVoteWeight) QuorumThreshold() int64 {
	return v.ParticipantsCount()*2/3 + 1
}

// RewardThreshold ..
func (v *uniformVoteWeight) IsRewardThresholdAchieved() bool {
	return v.SignatoriesCount(Commit) >= (v.ParticipantsCount() * 9 / 10)
}

func (v *uniformVoteWeight) UpdateVotingPower(func(*bls.PublicKey) numeric.Dec) {
	// NO-OP do not add anything here
}

func (v *uniformVoteWeight) ToggleActive(*bls.PublicKey) bool {
	// NO-OP do not add anything here
	return true
}
