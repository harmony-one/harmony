package quorum

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	// "github.com/harmony-one/harmony/staking/effective"
)

type uniformVoteWeight struct {
	DependencyInjectionWriter
	SignatureReader
}

// Policy ..
func (v *uniformVoteWeight) Policy() Policy {
	return SuperMajorityVote
}

// IsQuorumAchieved ..
func (v *uniformVoteWeight) IsQuorumAchieved(p Phase) bool {
	r := v.SignersCount(p) >= v.QuorumThreshold().Int64()
	utils.Logger().Info().Str("phase", p.String()).
		Int64("signers-count", v.SignersCount(p)).
		Int64("threshold", v.QuorumThreshold().Int64()).
		Int64("participants", v.ParticipantsCount()).
		Msg("Quorum details")
	return r
}

// QuorumThreshold ..
func (v *uniformVoteWeight) QuorumThreshold() *big.Int {
	return big.NewInt(v.ParticipantsCount()*2/3 + 1)
}

// RewardThreshold ..
func (v *uniformVoteWeight) IsRewardThresholdAchieved() bool {
	return v.SignersCount(Commit) >= (v.ParticipantsCount() * 9 / 10)
}

// func (v *uniformVoteWeight) UpdateVotingPower(effective.StakeKeeper) {
// NO-OP do not add anything here
// }

// ToggleActive for uniform vote is a no-op, always says that voter is active
func (v *uniformVoteWeight) ToggleActive(*bls.PublicKey) bool {
	// NO-OP do not add anything here
	return true
}

// Award ..
func (v *uniformVoteWeight) Award(
	// Here hook is the callback which gets the amount the earner is due in just reward
	// up to the hook to do side-effects like write the statedb
	Pie *big.Int, earners []common.Address, hook func(earner common.Address, due *big.Int),
) *big.Int {
	payout := big.NewInt(0)
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

	return payout
}

func (v *uniformVoteWeight) ShouldSlash(k shard.BlsPublicKey) bool {
	// No-op, no semantic meaning in one-slot-one-vote
	// fmt.Println("Called here for key:", k.Hex())
	return false
}
