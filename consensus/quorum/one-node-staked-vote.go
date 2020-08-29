package quorum

import (
	"encoding/json"
	"math/big"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/harmony-one/harmony/internal/utils"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/consensus/votepower"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

var (
	twoThird = numeric.NewDec(2).Quo(numeric.NewDec(3))
)

// TallyResult is the result of when we calculate voting power,
// recall that it happens to us at epoch change
type TallyResult struct {
	ourPercent   numeric.Dec
	theirPercent numeric.Dec
}

type tallyAndQuorum struct {
	tally          numeric.Dec
	quorumAchieved bool
}

// VoteTally is the vote tally for each phase
type VoteTally struct {
	Prepare    *tallyAndQuorum
	Commit     *tallyAndQuorum
	ViewChange *tallyAndQuorum
}

type stakedVoteWeight struct {
	SignatureReader
	DependencyInjectionWriter
	DependencyInjectionReader
	roster    votepower.Roster
	voteTally VoteTally
}

// Policy ..
func (v *stakedVoteWeight) Policy() Policy {
	return SuperMajorityStake
}

// AddNewVote ..
func (v *stakedVoteWeight) AddNewVote(
	p Phase, pubKeyBytes bls.SerializedPublicKey,
	sig *bls_core.Sign, headerHash common.Hash,
	height, viewID uint64) (*votepower.Ballot, error) {

	// TODO(audit): pass in sig as byte[] too, so no need to serialize
	ballet, err := v.SubmitVote(p, pubKeyBytes, sig, headerHash, height, viewID)

	if err != nil {
		return ballet, err
	}

	// Accumulate total voting power
	additionalVotePower := v.roster.Voters[pubKeyBytes].OverallPercent
	tallyQuorum := func() *tallyAndQuorum {
		switch p {
		case Prepare:
			return v.voteTally.Prepare
		case Commit:
			return v.voteTally.Commit
		case ViewChange:
			return v.voteTally.ViewChange
		default:
			// Should not happen
			return nil
		}
	}()

	tallyQuorum.tally = tallyQuorum.tally.Add(additionalVotePower)

	t := v.QuorumThreshold()

	msg := "Attempt to reach quorum"
	if !tallyQuorum.quorumAchieved {
		tallyQuorum.quorumAchieved = tallyQuorum.tally.GT(t)

		if tallyQuorum.quorumAchieved {
			msg = "Quorum Achieved!"
		}
	}
	utils.Logger().Info().
		Str("phase", p.String()).
		Int64("signer-count", v.SignersCount(p)).
		Str("total-power-of-signers", tallyQuorum.tally.String()).
		Msg(msg)
	return ballet, nil
}

// IsQuorumAchieved ..
func (v *stakedVoteWeight) IsQuorumAchieved(p Phase) bool {
	switch p {
	case Prepare:
		return v.voteTally.Prepare.quorumAchieved
	case Commit:
		return v.voteTally.Commit.quorumAchieved
	case ViewChange:
		return v.voteTally.ViewChange.quorumAchieved
	default:
		// Should not happen
		return false
	}
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

func (v *stakedVoteWeight) currentTotalPower(p Phase) (*numeric.Dec, error) {
	switch p {
	case Prepare:
		return &v.voteTally.Prepare.tally, nil
	case Commit:
		return &v.voteTally.Commit.tally, nil
	case ViewChange:
		return &v.voteTally.ViewChange.tally, nil
	default:
		// Should not happen
		return nil, errors.New("wrong phase is provided")
	}
}

// ComputeTotalPowerByMask computes the total power indicated by bitmap mask
func (v *stakedVoteWeight) computeTotalPowerByMask(mask *bls_cosi.Mask) *numeric.Dec {
	currentTotal := numeric.ZeroDec()

	for key, i := range mask.PublicsIndex {
		if enabled, err := mask.IndexEnabled(i); err == nil && enabled {
			currentTotal = currentTotal.Add(
				v.roster.Voters[key].OverallPercent,
			)
		}
	}
	return &currentTotal
}

// QuorumThreshold ..
func (v *stakedVoteWeight) QuorumThreshold() numeric.Dec {
	return twoThird
}

// IsAllSigsCollected ..
func (v *stakedVoteWeight) IsAllSigsCollected() bool {
	return v.SignersCount(Commit) == v.ParticipantsCount()
}

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

// HACK later remove - unify votepower in UI (aka MarshalJSON)
func (v *stakedVoteWeight) SetRawStake(key bls.SerializedPublicKey, d numeric.Dec) {
	if voter, ok := v.roster.Voters[key]; ok {
		voter.RawStake = d
	}
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
		RawStake       string `json:"raw-stake,omitempty"`
	}

	type t struct {
		Policy              string `json:"policy"`
		Count               int    `json:"count"`
		Externals           int    `json:"external-validator-slot-count"`
		Participants        []u    `json:"committee-members"`
		HmyVotingPower      string `json:"hmy-voting-power"`
		StakedVotingPower   string `json:"staked-voting-power"`
		TotalRawStake       string `json:"total-raw-stake"`
		TotalEffectiveStake string `json:"total-effective-stake"`
	}

	parts := make([]u, voterCount)
	i, externalCount := 0, 0

	totalRaw := numeric.ZeroDec()
	for identity, voter := range v.roster.Voters {
		member := u{
			voter.IsHarmonyNode,
			common2.MustAddressToBech32(voter.EarningAccount),
			identity.Hex(),
			voter.GroupPercent.String(),
			voter.OverallPercent.String(),
			"",
			"",
		}
		if !voter.IsHarmonyNode {
			externalCount++
			member.EffectiveStake = voter.EffectiveStake.String()
			member.RawStake = voter.RawStake.String()
			totalRaw = totalRaw.Add(voter.RawStake)
		}
		parts[i] = member
		i++
	}

	return json.Marshal(t{
		v.Policy().String(),
		voterCount,
		externalCount,
		parts,
		v.roster.OurVotingPowerTotalPercentage.String(),
		v.roster.TheirVotingPowerTotalPercentage.String(),
		totalRaw.String(),
		v.roster.TotalEffectiveStake.String(),
	})
}

func (v *stakedVoteWeight) AmIMemberOfCommitee() bool {
	pubKeyFunc := v.MyPublicKey()
	if pubKeyFunc == nil {
		return false
	}
	identity, _ := pubKeyFunc()
	for _, key := range identity {
		_, ok := v.roster.Voters[key.Bytes]
		if ok {
			return true
		}
	}
	return false
}

func newVoteTally() VoteTally {
	return VoteTally{
		Prepare:    &tallyAndQuorum{numeric.NewDec(0), false},
		Commit:     &tallyAndQuorum{numeric.NewDec(0), false},
		ViewChange: &tallyAndQuorum{numeric.NewDec(0), false},
	}
}

func (v *stakedVoteWeight) ResetPrepareAndCommitVotes() {
	v.reset([]Phase{Prepare, Commit})
	v.voteTally.Prepare = &tallyAndQuorum{numeric.NewDec(0), false}
	v.voteTally.Commit = &tallyAndQuorum{numeric.NewDec(0), false}
}

func (v *stakedVoteWeight) ResetViewChangeVotes() {
	v.reset([]Phase{ViewChange})
	v.voteTally.ViewChange = &tallyAndQuorum{numeric.NewDec(0), false}
}
