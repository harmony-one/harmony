package quorum

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
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

// Award ..
func (v *uniformVoteWeight) Award(
	// Here hook is the callback which gets the amount the earner is due in just reward
	// up to the hook to do side-effects like write the statedb
	Pie *big.Int,
	earners shard.SlotList,
	hook func(earner common.Address, due *big.Int),
) *big.Int {
	payout := big.NewInt(0)
	last := big.NewInt(0)
	count := big.NewInt(int64(len(earners)))

	for i, account := range earners {
		cur := big.NewInt(0)
		cur.Mul(Pie, big.NewInt(int64(i+1))).Div(cur, count)
		diff := big.NewInt(0).Sub(cur, last)
		hook(common.Address(account.EcdsaAddress), diff)
		payout = big.NewInt(0).Add(payout, diff)
		last = cur
	}

	return payout
}

func (v *uniformVoteWeight) String() string {
	s, _ := json.Marshal(v)
	return string(s)
}

func (v *uniformVoteWeight) MarshalJSON() ([]byte, error) {
	s, _ := v.ShardIDProvider()()

	type t struct {
		Policy       string   `json:"policy"`
		ShardID      uint32   `json:"shard-id"`
		Count        int      `json:"count"`
		Participants []string `json:"committee-members"`
	}

	members := v.DumpParticipants()
	return json.Marshal(t{v.Policy().String(), s, len(members), members})
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
