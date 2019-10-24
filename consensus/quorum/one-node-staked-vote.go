package quorum

import (
	"math/big"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
)

var (
	twoThirds = numeric.NewDec(2).QuoInt64(3)
)

type stakedVoter struct {
	isActive, isHarmonyNode bool
	effective               numeric.Dec
}

type stakedVoteWeight struct {
	SignatureReader
	// EPOS based staking
	validatorStakes            map[*bls.PublicKey]stakedVoter
	totalEffectiveStakedAmount numeric.Dec
}

// Policy ..
func (v *stakedVoteWeight) Policy() Policy {
	return SuperMajorityStake
}

// We must maintain 2/3 quoroum, so whatever is 2/3 staked amount,
// we divide that out & you
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

// HACK
var (
	hSentinel          = big.NewInt(0)
	hEffectiveSentinel = numeric.ZeroDec()
)

// Award ..
func (v *stakedVoteWeight) Award(
	Pie *big.Int, earners []common.Address, hook func(earner common.Address, due *big.Int),
) *big.Int {
	return nil
}

// UpdateVotingPower called only at epoch change, prob need to move to CalculateShardState
func (v *stakedVoteWeight) UpdateVotingPower(keeper effective.StakeKeeper) {
	members := keeper.Inventory()
	count := len(members.BLSPublicKeys)
	totalAmount := numeric.ZeroDec()
	realStakes := []numeric.Dec{}

	for i := 0; i < count; i++ {
		bPublic := &bls.PublicKey{}
		// TODO handle error
		bPublic.Deserialize(members.BLSPublicKeys[i][:])

		switch stake := members.WithDelegationApplied[i]; stake.Cmp(hSentinel) {
		// Our node
		case 0:
			v.validatorStakes[bPublic] = stakedVoter{
				true, true, hEffectiveSentinel,
			}
		default:
			realStakes = append(realStakes, members.WithDelegationApplied[i])
			v.validatorStakes[bPublic] = stakedVoter{
				true, false, members.WithDelegationApplied[i],
			}
		}
	}

	median := effective.Median(realStakes)
	for _, voter := range v.validatorStakes {
		if !voter.isHarmonyNode {
			voter.effective = effective.Stake(median, voter.effective)
			totalAmount.Add(voter.effective)
		}
	}

	//

}

func (v *stakedVoteWeight) ToggleActive(*bls.PublicKey) bool {

	return true
}
