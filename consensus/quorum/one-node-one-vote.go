package quorum

import (
	"encoding/json"

	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

type uniformVoteWeight struct {
	DependencyInjectionWriter
	DependencyInjectionReader
	SignatureReader
}

// Policy ..
func (v *uniformVoteWeight) Policy() Policy {
	return SuperMajorityVote
}

// IsQuorumAchieved ..
func (v *uniformVoteWeight) IsQuorumAchieved(p Phase) bool {
	r := v.SignersCount(p) >= v.TwoThirdsSignersCount()
	utils.Logger().Info().Str("phase", p.String()).
		Int64("signers-count", v.SignersCount(p)).
		Int64("threshold", v.TwoThirdsSignersCount()).
		Int64("participants", v.ParticipantsCount()).
		Msg("Quorum details")
	return r
}

// IsQuorumAchivedByMask ..
func (v *uniformVoteWeight) IsQuorumAchievedByMask(mask *bls_cosi.Mask) bool {
	threshold := v.TwoThirdsSignersCount()
	currentTotalPower := utils.CountOneBits(mask.Bitmap)
	if currentTotalPower < threshold {
		utils.Logger().Warn().
			Msgf("[IsQuorumAchievedByMask] Not enough voting power: need %+v, have %+v", threshold, currentTotalPower)
		return false
	}
	utils.Logger().Debug().
		Msgf("[IsQuorumAchievedByMask] have enough voting power: need %+v, have %+v",
			threshold, currentTotalPower)
	return true
}

// QuorumThreshold ..
func (v *uniformVoteWeight) QuorumThreshold() numeric.Dec {
	return numeric.NewDec(v.TwoThirdsSignersCount())
}

// RewardThreshold ..
func (v *uniformVoteWeight) IsRewardThresholdAchieved() bool {
	return v.SignersCount(Commit) >= (v.ParticipantsCount() * 9 / 10)
}

func (v *uniformVoteWeight) SetVoters(shard.SlotList) (*TallyResult, error) {
	// NO-OP do not add anything here
	return nil, nil
}

func (v *uniformVoteWeight) String() string {
	s, _ := json.Marshal(v)
	return string(s)
}

func (v *uniformVoteWeight) MarshalJSON() ([]byte, error) {
	type t struct {
		Policy       string   `json:"policy"`
		Count        int      `json:"count"`
		Participants []string `json:"committee-members"`
	}

	members := v.DumpParticipants()
	return json.Marshal(t{v.Policy().String(), len(members), members})
}

func (v *uniformVoteWeight) AmIMemberOfCommitee() bool {
	pubKeyFunc := v.MyPublicKey()
	if pubKeyFunc == nil {
		return false
	}
	identity, _ := pubKeyFunc()
	everyone := v.DumpParticipants()
	for _, key := range identity.PublicKey {
		myVoterID := key.SerializeToHexStr()
		for i := range everyone {
			if everyone[i] == myVoterID {
				return true
			}
		}
	}
	return false
}

func (v *uniformVoteWeight) ResetPrepareAndCommitVotes() {
	v.reset([]Phase{Prepare, Commit})
}

func (v *uniformVoteWeight) ResetViewChangeVotes() {
	v.reset([]Phase{ViewChange})
}
