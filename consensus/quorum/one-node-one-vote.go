package quorum

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/staking/effective"
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

func (v *uniformVoteWeight) UpdateVotingPower(effective.StakeKeeper) {
	// NO-OP do not add anything here
}

// ToggleActive for uniform vote is a no-op, always says that voter is active
func (v *uniformVoteWeight) ToggleActive(*bls.PublicKey) bool {
	// NO-OP do not add anything here
	return true
}

// Award ..
func (v *uniformVoteWeight) Award(
	Pie *big.Int, earners []common2.Address, hook func(earner common.Address, due *big.Int),
) (payout *big.Int) {

	last := big.NewInt(0)
	count := big.NewInt(int64(len(earners)))

	for i, account := range earners {
		cur := big.NewInt(0)
		cur.Mul(Pie, big.NewInt(int64(i+1))).Div(cur, count)
		diff := big.NewInt(0).Sub(cur, last)
		hook(common.Address(account), diff)
		payout = big.NewInt(0).Add(payout, diff)
		last = cur
	}

	return
}
