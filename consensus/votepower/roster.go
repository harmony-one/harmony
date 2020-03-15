package votepower

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"

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
	GroupPercent   numeric.Dec        `json:"group-percent"`
	EffectiveStake *big.Int           `json:"effective-stake"`
}

// AccommodateHarmonyVote ..
type AccommodateHarmonyVote struct {
	PureStakedVote
	IsHarmonyNode  bool        `json:"is-harmony"`
	OverallPercent numeric.Dec `json:"overall-percent"`
}

func (v AccommodateHarmonyVote) String() string {
	s, _ := json.Marshal(v)
	return string(s)
}

type topLevelRegistry struct {
	OurVotingPowerTotalPercentage   numeric.Dec
	TheirVotingPowerTotalPercentage numeric.Dec
	RawStakedTotal                  *big.Int
	HMYSlotCount                    int64
}

// Roster ..
type Roster struct {
	Voters map[shard.BlsPublicKey]*AccommodateHarmonyVote
	topLevelRegistry
	ShardID uint32
}

func (r Roster) String() string {
	s, _ := json.Marshal(r)
	return string(s)
}

// VoteOnSubcomittee ..
type VoteOnSubcomittee struct {
	AccommodateHarmonyVote
	ShardID uint32 `json:"shard-id"`
}

// AggregateRosters ..
func AggregateRosters(
	rosters []*Roster,
) map[common.Address][]VoteOnSubcomittee {
	result := map[common.Address][]VoteOnSubcomittee{}
	sort.SliceStable(rosters, func(i, j int) bool {
		return rosters[i].ShardID < rosters[j].ShardID
	})

	for _, roster := range rosters {
		for _, voteCard := range roster.Voters {
			if payload, ok := result[voteCard.EarningAccount]; ok {
				payload = append(payload, VoteOnSubcomittee{
					AccommodateHarmonyVote: *voteCard,
					ShardID:                roster.ShardID,
				})
			} else {
				result[voteCard.EarningAccount] = []VoteOnSubcomittee{}
			}
		}
	}

	return result
}

// Compute creates a new roster based off the shard.SlotList
func Compute(subComm *shard.Committee) (*Roster, error) {
	roster, staked := NewRoster(subComm.ShardID), subComm.Slots

	for i := range staked {
		if e := staked[i].EffectiveStake; e != nil {
			roster.RawStakedTotal.Add(
				roster.RawStakedTotal, e.TruncateInt(),
			)
		} else {
			roster.HMYSlotCount++
		}
	}

	asDecTotal, asDecHMYSlotCount :=
		numeric.NewDecFromBigInt(roster.RawStakedTotal),
		numeric.NewDec(roster.HMYSlotCount)
	// TODO Check for duplicate BLS Keys
	ourPercentage := numeric.ZeroDec()
	theirPercentage := numeric.ZeroDec()
	var lastStakedVoter *AccommodateHarmonyVote

	for i := range staked {
		member := AccommodateHarmonyVote{
			PureStakedVote: PureStakedVote{
				EarningAccount: staked[i].EcdsaAddress,
				Identity:       staked[i].BlsPublicKey,
				GroupPercent:   numeric.ZeroDec(),
				EffectiveStake: big.NewInt(0),
			},
			OverallPercent: numeric.ZeroDec(),
			IsHarmonyNode:  false,
		}

		// Real Staker
		if e := staked[i].EffectiveStake; e != nil {
			member.EffectiveStake.Add(
				member.EffectiveStake, (*e).TruncateInt(),
			)
			member.GroupPercent = e.Quo(asDecTotal)
			member.OverallPercent = member.GroupPercent.Mul(StakersShare)
			theirPercentage = theirPercentage.Add(member.OverallPercent)
			lastStakedVoter = &member
		} else { // Our node
			member.IsHarmonyNode = true
			member.OverallPercent = HarmonysShare.Quo(asDecHMYSlotCount)
			member.GroupPercent = member.OverallPercent.Quo(HarmonysShare)
			ourPercentage = ourPercentage.Add(member.OverallPercent)
		}

		roster.Voters[staked[i].BlsPublicKey] = &member
	}

	// NOTE Enforce voting power sums to one,
	// give diff (expect tiny amt) to last staked voter
	if diff := numeric.OneDec().Sub(
		ourPercentage.Add(theirPercentage),
	); !diff.IsZero() && lastStakedVoter != nil {
		lastStakedVoter.OverallPercent =
			lastStakedVoter.OverallPercent.Add(diff)
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
func NewRoster(shardID uint32) *Roster {
	m := map[shard.BlsPublicKey]*AccommodateHarmonyVote{}
	return &Roster{
		Voters: m,
		topLevelRegistry: topLevelRegistry{
			OurVotingPowerTotalPercentage:   numeric.ZeroDec(),
			TheirVotingPowerTotalPercentage: numeric.ZeroDec(),
			RawStakedTotal:                  big.NewInt(0),
		},
		ShardID: shardID,
	}
}
