package votepower

import (
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

var (
	// HarmonysShare ..
	HarmonysShare = numeric.MustNewDecFromStr("0.68")
	// StakersShare ..
	StakersShare = numeric.MustNewDecFromStr("0.32")
	// ErrVotingPowerNotEqualOne ..
	ErrVotingPowerNotEqualOne = errors.New("voting power not equal to one")
)

// Ballot is a vote cast by a validator
type Ballot struct {
	SignerPubKey    shard.BlsPublicKey `json:"bls-public-key"`
	BlockHeaderHash common.Hash        `json:"block-header-hash"`
	Signature       []byte             `json:"bls-signature"`
	Height          uint64             `json:"block-height"`
	ViewID          uint64             `json:"view-id"`
}

// MarshalJSON ..
func (b Ballot) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		A string `json:"bls-public-key"`
		B string `json:"block-header-hash"`
		C string `json:"bls-signature"`
		E uint64 `json:"block-height"`
		F uint64 `json:"view-id"`
	}{
		b.SignerPubKey.Hex(),
		b.BlockHeaderHash.Hex(),
		hex.EncodeToString(b.Signature),
		b.Height,
		b.ViewID,
	})
}

// Round is a round of voting in any FBFT phase
type Round struct {
	AggregatedVote *bls.Sign
	BallotBox      map[shard.BlsPublicKey]*Ballot
}

func (b Ballot) String() string {
	data, _ := json.Marshal(b)
	return string(data)
}

// NewRound ..
func NewRound() *Round {
	return &Round{
		AggregatedVote: &bls.Sign{},
		BallotBox:      map[shard.BlsPublicKey]*Ballot{},
	}
}

// PureStakedVote ..
type PureStakedVote struct {
	EarningAccount common.Address     `json:"earning-account"`
	Identity       shard.BlsPublicKey `json:"bls-public-key"`
	VotingPower    numeric.Dec        `json:"voting-power"`
	EffectiveStake numeric.Dec        `json:"effective-stake"`
}

// AccommodateHarmonyVote ..
type AccommodateHarmonyVote struct {
	PureStakedVote
	IsHarmonyNode       bool        `json:"is-harmony"`
	AdjustedVotingPower numeric.Dec `json:"voting-adjusted"`
}

// Roster ..
type Roster struct {
	Voters                          map[shard.BlsPublicKey]AccommodateHarmonyVote
	ForEpoch                        *big.Int
	ShardID                         uint32
	OurVotingPowerTotalPercentage   numeric.Dec
	TheirVotingPowerTotalPercentage numeric.Dec
	RawStakedTotal                  *big.Int
}

// VoteOnSubcomittee ..
type VoteOnSubcomittee struct {
	Vote    AccommodateHarmonyVote
	ShardID uint32
}

// AggregatedAcrossNetwork ..
type AggregatedAcrossNetwork struct {
	StakedValidatorAddr common.Address
	TotalEffectiveStake numeric.Dec
	Votes               []VoteOnSubcomittee
}

// AggregateRosters ..
func AggregateRosters(
	rosters []*Roster,
) map[common.Address]AggregatedAcrossNetwork {
	result := map[common.Address]AggregatedAcrossNetwork{}

	for _, roster := range rosters {
		for _, voteCard := range roster.Voters {
			if payload, ok := result[voteCard.EarningAccount]; ok {
				payload.TotalEffectiveStake = payload.TotalEffectiveStake.Add(
					voteCard.EffectiveStake,
				)
				payload.Votes = append(payload.Votes, VoteOnSubcomittee{
					Vote:    voteCard,
					ShardID: roster.ShardID,
				})
			} else {
				result[voteCard.EarningAccount] = AggregatedAcrossNetwork{
					StakedValidatorAddr: voteCard.EarningAccount,
					TotalEffectiveStake: numeric.ZeroDec(),
					Votes:               []VoteOnSubcomittee{},
				}
			}
		}
	}

	return result
}

// Compute creates a new roster based off the shard.SlotList
func Compute(subComm *shard.Committee) (*Roster, error) {
	roster, staked := NewRoster(), subComm.Slots
	hmySlotCount := int64(0)
	for i := range staked {
		if staked[i].EffectiveStake != nil {
			roster.RawStakedTotal.Add(
				roster.RawStakedTotal,
				staked[i].EffectiveStake.TruncateInt(),
			)
		} else {
			hmySlotCount++
		}
	}
	asDecTotal, asDecHMYSlotCount :=
		numeric.NewDecFromBigInt(roster.RawStakedTotal),
		numeric.NewDec(hmySlotCount)
	// TODO Check for duplicate BLS Keys
	ourPercentage := numeric.ZeroDec()
	theirPercentage := numeric.ZeroDec()
	var lastStakedVoter *AccommodateHarmonyVote

	for i := range staked {
		member := AccommodateHarmonyVote{
			PureStakedVote: PureStakedVote{
				EarningAccount: staked[i].EcdsaAddress,
				Identity:       staked[i].BlsPublicKey,
				VotingPower:    numeric.ZeroDec(),
				EffectiveStake: numeric.ZeroDec(),
			},
			AdjustedVotingPower: numeric.ZeroDec(),
			IsHarmonyNode:       true,
		}

		// Real Staker
		if staked[i].EffectiveStake != nil {
			member.IsHarmonyNode = false
			member.EffectiveStake = member.EffectiveStake.Add(*staked[i].EffectiveStake)
			member.VotingPower = staked[i].EffectiveStake.Quo(asDecTotal)
			member.AdjustedVotingPower = member.VotingPower.Mul(StakersShare)
			theirPercentage = theirPercentage.Add(member.AdjustedVotingPower)
			lastStakedVoter = &member
		} else { // Our node
			member.AdjustedVotingPower = HarmonysShare.Quo(asDecHMYSlotCount)
			member.VotingPower = member.AdjustedVotingPower.Quo(HarmonysShare)
			ourPercentage = ourPercentage.Add(member.AdjustedVotingPower)
		}

		roster.Voters[staked[i].BlsPublicKey] = member
	}

	// NOTE Enforce voting power sums to one,
	// give diff (expect tiny amt) to last staked voter
	if diff := numeric.OneDec().Sub(
		ourPercentage.Add(theirPercentage),
	); !diff.IsZero() && lastStakedVoter != nil {
		lastStakedVoter.AdjustedVotingPower =
			lastStakedVoter.AdjustedVotingPower.Add(diff)
		theirPercentage = theirPercentage.Add(diff)
	}

	if lastStakedVoter != nil &&
		!ourPercentage.Add(theirPercentage).Equal(numeric.OneDec()) {
		return nil, ErrVotingPowerNotEqualOne
	}

	roster.OurVotingPowerTotalPercentage = ourPercentage
	roster.TheirVotingPowerTotalPercentage = theirPercentage
	return roster, nil
}

// NewRoster ..
func NewRoster() *Roster {
	m := map[shard.BlsPublicKey]AccommodateHarmonyVote{}
	return &Roster{
		Voters:                          m,
		OurVotingPowerTotalPercentage:   numeric.ZeroDec(),
		TheirVotingPowerTotalPercentage: numeric.ZeroDec(),
		RawStakedTotal:                  big.NewInt(0),
	}
}
