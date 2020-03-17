package quorum

import (
	"encoding/json"
	"math/big"

	"github.com/harmony-one/harmony/consensus/votepower"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

var (
	twoThird      = numeric.NewDec(2).Quo(numeric.NewDec(3))
	ninetyPercent = numeric.MustNewDecFromStr("0.90")
	totalShare    = numeric.MustNewDecFromStr("1.00")
)

// TallyResult is the result of when we calculate voting power,
// recall that it happens to us at epoch change
type TallyResult struct {
	ourPercent   numeric.Dec
	theirPercent numeric.Dec
}

type voteBox struct {
	voters       map[shard.BlsPublicKey]struct{}
	currentTotal numeric.Dec
}

type box struct {
	Prepare    *voteBox
	Commit     *voteBox
	ViewChange *voteBox
}

type stakedVoteWeight struct {
	SignatureReader
	DependencyInjectionWriter
	DependencyInjectionReader
	roster    votepower.Roster
	ballotBox box
}

// Policy ..
func (v *stakedVoteWeight) Policy() Policy {
	return SuperMajorityStake
}

// IsQuorumAchieved ..
func (v *stakedVoteWeight) IsQuorumAchieved(p Phase) bool {
	t := v.QuorumThreshold()
	currentTotalPower, err := v.computeCurrentTotalPower(p)

	if err != nil {
		utils.Logger().Error().
			AnErr("bls error", err).
			Msg("Failure in attempt bls-key reading")
		return false
	}

	utils.Logger().Info().
		Str("policy", v.Policy().String()).
		Str("phase", p.String()).
		Str("threshold", t.String()).
		Str("total-power-of-signers", currentTotalPower.String()).
		Msg("Attempt to reach quorum")
	return currentTotalPower.GT(t)
}

// IsQuorumAchivedByMask ..
func (v *stakedVoteWeight) IsQuorumAchievedByMask(mask *bls_cosi.Mask) bool {
	threshold := v.QuorumThreshold()
	currentTotalPower := v.computeTotalPowerByMask(mask)
	if currentTotalPower == nil {
		return false
	}
	return (*currentTotalPower).GT(threshold)
}
func (v *stakedVoteWeight) computeCurrentTotalPower(p Phase) (*numeric.Dec, error) {
	w := shard.BlsPublicKey{}
	members := v.Participants()
	ballot := func() *voteBox {
		switch p {
		case Prepare:
			return v.ballotBox.Prepare
		case Commit:
			return v.ballotBox.Commit
		case ViewChange:
			return v.ballotBox.ViewChange
		default:
			// Should not happen
			return nil
		}
	}()

	for i := range members {
		if err := w.FromLibBLSPublicKey(members[i]); err != nil {
			return nil, err
		}
		if _, didVote := ballot.voters[w]; !didVote &&
			v.ReadBallot(p, members[i]) != nil {
			ballot.currentTotal = ballot.currentTotal.Add(
				v.roster.Voters[w].OverallPercent,
			)
			ballot.voters[w] = struct{}{}
		}
	}

	return &ballot.currentTotal, nil
}

// ComputeTotalPowerByMask computes the total power indicated by bitmap mask
func (v *stakedVoteWeight) computeTotalPowerByMask(mask *bls_cosi.Mask) *numeric.Dec {
	pubKeys := mask.Publics
	w := shard.BlsPublicKey{}
	currentTotal := numeric.ZeroDec()

	for i := range pubKeys {
		if err := w.FromLibBLSPublicKey(pubKeys[i]); err != nil {
			return nil
		}
		if enabled, err := mask.KeyEnabled(pubKeys[i]); err == nil && enabled {
			currentTotal = currentTotal.Add(
				v.roster.Voters[w].OverallPercent,
			)
		}
	}
	return &currentTotal
}

// QuorumThreshold ..
func (v *stakedVoteWeight) QuorumThreshold() numeric.Dec {
	return twoThird
}

// RewardThreshold ..
func (v *stakedVoteWeight) IsRewardThresholdAchieved() bool {
	reached, err := v.computeCurrentTotalPower(Commit)
	if err != nil {
		utils.Logger().Error().
			AnErr("bls error", err).
			Msg("Failure in attempt bls-key reading")
		return false
	}
	return reached.GTE(ninetyPercent)
}

var (
	errSumOfVotingPowerNotOne   = errors.New("sum of total votes do not sum to 100 percent")
	errSumOfOursAndTheirsNotOne = errors.New(
		"sum of hmy nodes and stakers do not sum to 100 percent",
	)
)

func (v *stakedVoteWeight) SetVoters(
	subCommittee *shard.Committee, epoch *big.Int,
) (*TallyResult, error) {
	v.ResetPrepareAndCommitVotes()
	v.ResetViewChangeVotes()

	roster, err := votepower.Compute(subCommittee, epoch)
	if err != nil {
		return nil, err
	}
	// Hold onto this calculation
	v.roster = *roster
	return &TallyResult{
		roster.OurVotingPowerTotalPercentage,
		roster.TheirVotingPowerTotalPercentage,
	}, nil
}

func (v *stakedVoteWeight) String() string {
	s, _ := json.Marshal(v)
	return string(s)
}

// TODO remove this large method, use roster's own Marshal, mix it
// specific logic here
func (v *stakedVoteWeight) MarshalJSON() ([]byte, error) {
	voterCount := len(v.roster.Voters)
	type u struct {
		IsHarmony      bool   `json:"is-harmony-slot"`
		EarningAccount string `json:"earning-account"`
		Identity       string `json:"bls-public-key"`
		RawPercent     string `json:"voting-power-unnormalized"`
		VotingPower    string `json:"voting-power-%"`
		EffectiveStake string `json:"effective-stake,omitempty"`
	}

	type t struct {
		Policy            string `json:"policy"`
		Count             int    `json:"count"`
		Participants      []u    `json:"committee-members"`
		HmyVotingPower    string `json:"hmy-voting-power"`
		StakedVotingPower string `json:"staked-voting-power"`
		TotalStaked       string `json:"total-raw-staked"`
	}

	parts := make([]u, voterCount)
	i := 0

	for identity, voter := range v.roster.Voters {
		member := u{
			voter.IsHarmonyNode,
			common2.MustAddressToBech32(voter.EarningAccount),
			identity.Hex(),
			voter.GroupPercent.String(),
			voter.OverallPercent.String(),
			"",
		}
		if !voter.IsHarmonyNode {
			member.EffectiveStake = voter.EffectiveStake.String()
		}
		parts[i] = member
		i++
	}

	return json.Marshal(t{
		v.Policy().String(),
		voterCount,
		parts,
		v.roster.OurVotingPowerTotalPercentage.String(),
		v.roster.TheirVotingPowerTotalPercentage.String(),
		v.roster.TotalEffectiveStake.String(),
	})
}

func (v *stakedVoteWeight) AmIMemberOfCommitee() bool {
	pubKeyFunc := v.MyPublicKey()
	if pubKeyFunc == nil {
		return false
	}
	identity, _ := pubKeyFunc()
	for _, key := range identity.PublicKey {
		if w := (shard.BlsPublicKey{}); w.FromLibBLSPublicKey(key) != nil {
			_, ok := v.roster.Voters[w]
			if ok {
				return true
			}
		}
	}
	return false
}

func newBox() *voteBox {
	return &voteBox{map[shard.BlsPublicKey]struct{}{}, numeric.ZeroDec()}
}

func newBallotBox() box {
	return box{
		Prepare:    newBox(),
		Commit:     newBox(),
		ViewChange: newBox(),
	}
}

func (v *stakedVoteWeight) ResetPrepareAndCommitVotes() {
	v.reset([]Phase{Prepare, Commit})
	v.ballotBox.Prepare = newBox()
	v.ballotBox.Commit = newBox()
}

func (v *stakedVoteWeight) ResetViewChangeVotes() {
	v.reset([]Phase{ViewChange})
	v.ballotBox.ViewChange = newBox()
}
