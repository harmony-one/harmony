package quorum

import (
	"bytes"
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
	roster    votepower.Roster
	voteTally VoteTally
	lastPower map[Phase]numeric.Dec
}

// Policy ..
func (v *stakedVoteWeight) Policy() Policy {
	return SuperMajorityStake
}

// AddNewVote ..
func (v *stakedVoteWeight) AddNewVote(
	p Phase, pubKeys []*bls_cosi.PublicKeyWrapper,
	sig *bls_core.Sign, headerHash common.Hash,
	height, viewID uint64) (*votepower.Ballot, error) {

	pubKeysBytes := make([]bls.SerializedPublicKey, len(pubKeys))
	signerAddr := common.Address{}
	for i, pubKey := range pubKeys {
		voter, ok := v.roster.Voters[pubKey.Bytes]
		if !ok {
			return nil, errors.Errorf("Signer not in committee: %x", pubKey.Bytes)
		}
		if i == 0 {
			signerAddr = voter.EarningAccount
		} else {
			// Aggregated signature should not contain signatures from keys belonging to different accounts,
			// to avoid malicious node catching other people's signatures and merge with their own to cause problems.
			// Harmony nodes are excluded from this rule.
			if bytes.Compare(signerAddr.Bytes(), voter.EarningAccount[:]) != 0 && !voter.IsHarmonyNode {
				return nil, errors.Errorf("Multiple signer accounts used in multi-sig: %x, %x", signerAddr.Bytes(), voter.EarningAccount)
			}
		}
		pubKeysBytes[i] = pubKey.Bytes
	}

	ballet, err := v.submitVote(p, pubKeysBytes, sig, headerHash, height, viewID)

	if err != nil {
		return ballet, err
	}

	// Accumulate total voting power
	additionalVotePower := numeric.NewDec(0)

	for _, pubKeyBytes := range pubKeysBytes {
		votingPower := v.roster.Voters[pubKeyBytes].OverallPercent
		utils.Logger().Debug().
			Str("signer", pubKeyBytes.Hex()).
			Str("votingPower", votingPower.String()).
			Msg("Signer vote counted")
		additionalVotePower = additionalVotePower.Add(votingPower)
	}

	var tallyQuorum *tallyAndQuorum
	switch p {
	case Prepare:
		tallyQuorum = v.voteTally.Prepare
	case Commit:
		tallyQuorum = v.voteTally.Commit
	case ViewChange:
		tallyQuorum = v.voteTally.ViewChange
	default:
		// Should not happen
		return nil, errors.New("stakedVoteWeight not cache this phase")
	}

	tallyQuorum.tally = tallyQuorum.tally.Add(additionalVotePower)

	t := v.QuorumThreshold()

	msg := "[AddNewVote] New Vote Added!"
	if !tallyQuorum.quorumAchieved {
		tallyQuorum.quorumAchieved = tallyQuorum.tally.GT(t)

		if tallyQuorum.quorumAchieved {
			msg = "[AddNewVote] Quorum Achieved!"
		}
	}
	utils.Logger().Info().
		Str("phase", p.String()).
		Int64("signer-count", v.SignersCount(p)).
		Str("new-power-added", additionalVotePower.String()).
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
	if mask == nil {
		return false
	}
	currentTotalPower := v.computeTotalPowerByMask(mask)
	if currentTotalPower == nil {
		return false
	}
	const msg = "[IsQuorumAchievedByMask] Voting power: need %+v, have %+v"
	utils.Logger().Debug().
		Msgf(msg, threshold, currentTotalPower)
	return (*currentTotalPower).GT(threshold)
}

// ComputeTotalPowerByMask computes the total power indicated by bitmap mask
func (v *stakedVoteWeight) computeTotalPowerByMask(mask *bls_cosi.Mask) *numeric.Dec {
	currentTotal := numeric.ZeroDec()

	for key, i := range mask.PublicsIndex {
		if enabled, err := mask.IndexEnabled(i); err == nil && enabled {
			if voter, ok := v.roster.Voters[key]; ok {
				currentTotal = currentTotal.Add(
					voter.OverallPercent,
				)
			}
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
	return v.voteTally.Commit.tally.Equal(numeric.NewDec(1))
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

	utils.Logger().Debug().
		Uint64("curEpoch", epoch.Uint64()).
		Uint32("shard-id", subCommittee.ShardID).
		Str("committee", roster.String()).
		Msg("[SetVoters] Successfully updated voters")
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
	for _, slot := range v.roster.OrderedSlots {
		identity := slot
		voter := v.roster.Voters[slot]
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

func newVoteTally() VoteTally {
	return VoteTally{
		Prepare:    &tallyAndQuorum{numeric.NewDec(0), false},
		Commit:     &tallyAndQuorum{numeric.NewDec(0), false},
		ViewChange: &tallyAndQuorum{numeric.NewDec(0), false},
	}
}

func (v *stakedVoteWeight) ResetPrepareAndCommitVotes() {
	v.lastPower[Prepare] = v.voteTally.Prepare.tally
	v.lastPower[Commit] = v.voteTally.Commit.tally

	v.reset([]Phase{Prepare, Commit})
	v.voteTally.Prepare = &tallyAndQuorum{numeric.NewDec(0), false}
	v.voteTally.Commit = &tallyAndQuorum{numeric.NewDec(0), false}
}

func (v *stakedVoteWeight) ResetViewChangeVotes() {
	v.lastPower[ViewChange] = v.voteTally.ViewChange.tally

	v.reset([]Phase{ViewChange})
	v.voteTally.ViewChange = &tallyAndQuorum{numeric.NewDec(0), false}
}

func (v *stakedVoteWeight) CurrentTotalPower(p Phase) (*numeric.Dec, error) {
	if power, ok := v.lastPower[p]; ok {
		return &power, nil
	} else {
		return nil, errors.New("stakedVoteWeight not cache this phase")
	}
}
