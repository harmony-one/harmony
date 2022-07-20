package votepower

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"github.com/harmony-one/harmony/shard"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/pkg/errors"
)

var (
	// ErrVotingPowerNotEqualOne ..
	ErrVotingPowerNotEqualOne = errors.New("voting power not equal to one")
)

// Ballot is a vote cast by a validator
type Ballot struct {
	SignerPubKeys   []bls.SerializedPublicKey `json:"bls-public-keys"`
	BlockHeaderHash common.Hash               `json:"block-header-hash"`
	Signature       []byte                    `json:"bls-signature"`
	Height          uint64                    `json:"block-height"`
	ViewID          uint64                    `json:"view-id"`
}

// MarshalJSON ..
func (b Ballot) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		A string `json:"bls-public-keys"`
		B string `json:"block-header-hash"`
		C string `json:"bls-signature"`
		E uint64 `json:"block-height"`
		F uint64 `json:"view-id"`
	}{
		fmt.Sprint(b.SignerPubKeys),
		b.BlockHeaderHash.Hex(),
		hex.EncodeToString(b.Signature),
		b.Height,
		b.ViewID,
	})
}

// Round is a round of voting in any FBFT phase
type Round struct {
	AggregatedVote *bls_core.Sign
	BallotBox      map[bls.SerializedPublicKey]*Ballot
}

func (b Ballot) String() string {
	data, _ := json.Marshal(b)
	return string(data)
}

// NewRound ..
func NewRound() *Round {
	return &Round{
		AggregatedVote: &bls_core.Sign{},
		BallotBox:      map[bls.SerializedPublicKey]*Ballot{},
	}
}

// PureStakedVote ..
type PureStakedVote struct {
	EarningAccount common.Address          `json:"earning-account"`
	Identity       bls.SerializedPublicKey `json:"bls-public-key"`
	GroupPercent   numeric.Dec             `json:"group-percent"`
	EffectiveStake numeric.Dec             `json:"effective-stake"`
	RawStake       numeric.Dec             `json:"raw-stake"`
}

// AccommodateHarmonyVote ..
type AccommodateHarmonyVote struct {
	PureStakedVote
	IsHarmonyNode  bool        `json:"-"`
	OverallPercent numeric.Dec `json:"overall-percent"`
}

// String ..
func (v AccommodateHarmonyVote) String() string {
	s, _ := json.Marshal(v)
	return string(s)
}

type topLevelRegistry struct {
	OurVotingPowerTotalPercentage   numeric.Dec
	TheirVotingPowerTotalPercentage numeric.Dec
	TotalEffectiveStake             numeric.Dec
	HMYSlotCount                    int64
}

// Roster ..
type Roster struct {
	Voters map[bls.SerializedPublicKey]*AccommodateHarmonyVote
	topLevelRegistry
	ShardID      uint32
	OrderedSlots []bls.SerializedPublicKey
}

func (r Roster) String() string {
	s, _ := json.Marshal(r)
	return string(s)
}

// VoteOnSubcomittee ..
type VoteOnSubcomittee struct {
	AccommodateHarmonyVote
	ShardID uint32
}

// MarshalJSON ..
func (v VoteOnSubcomittee) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PureStakedVote
		EarningAccount string      `json:"earning-account"`
		OverallPercent numeric.Dec `json:"overall-percent"`
		ShardID        uint32      `json:"shard-id"`
	}{
		v.PureStakedVote,
		common2.MustAddressToBech32(v.EarningAccount),
		v.OverallPercent,
		v.ShardID,
	})
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
			if !voteCard.IsHarmonyNode {
				voterID := VoteOnSubcomittee{
					AccommodateHarmonyVote: *voteCard,
					ShardID:                roster.ShardID,
				}
				result[voteCard.EarningAccount] = append(
					result[voteCard.EarningAccount], voterID,
				)
			}
		}
	}

	return result
}

// Compute creates a new roster based off the shard.SlotList
func Compute(subComm *shard.Committee, epoch *big.Int) (*Roster, error) {
	if epoch == nil {
		return nil, errors.New("nil epoch for roster compute")
	}
	roster, staked := NewRoster(subComm.ShardID), subComm.Slots

	for i := range staked {
		if e := staked[i].EffectiveStake; e != nil {
			roster.TotalEffectiveStake = roster.TotalEffectiveStake.Add(*e)
		} else {
			roster.HMYSlotCount++
		}
	}

	asDecHMYSlotCount := numeric.NewDec(roster.HMYSlotCount)
	// TODO Check for duplicate BLS Keys
	ourPercentage := numeric.ZeroDec()
	theirPercentage := numeric.ZeroDec()
	var lastStakedVoter *AccommodateHarmonyVote

	harmonyPercent := shard.Schedule.InstanceForEpoch(epoch).HarmonyVotePercent()
	externalPercent := shard.Schedule.InstanceForEpoch(epoch).ExternalVotePercent()

	for i := range staked {
		member := AccommodateHarmonyVote{
			PureStakedVote: PureStakedVote{
				EarningAccount: staked[i].EcdsaAddress,
				Identity:       staked[i].BLSPublicKey,
				GroupPercent:   numeric.ZeroDec(),
				EffectiveStake: numeric.ZeroDec(),
				RawStake:       numeric.ZeroDec(),
			},
			OverallPercent: numeric.ZeroDec(),
			IsHarmonyNode:  false,
		}

		// Real Staker
		if e := staked[i].EffectiveStake; e != nil {
			member.EffectiveStake = member.EffectiveStake.Add(*e)
			member.GroupPercent = e.Quo(roster.TotalEffectiveStake)
			member.OverallPercent = member.GroupPercent.Mul(externalPercent)
			theirPercentage = theirPercentage.Add(member.OverallPercent)
			lastStakedVoter = &member
		} else { // Our node
			member.IsHarmonyNode = true
			member.OverallPercent = harmonyPercent.Quo(asDecHMYSlotCount)
			member.GroupPercent = member.OverallPercent.Quo(harmonyPercent)
			ourPercentage = ourPercentage.Add(member.OverallPercent)
		}

		if _, ok := roster.Voters[staked[i].BLSPublicKey]; !ok {
			roster.Voters[staked[i].BLSPublicKey] = &member
		} else {
			utils.Logger().Debug().Str("blsKey", staked[i].BLSPublicKey.Hex()).Msg("Duplicate BLS key found")
		}
	}

	{
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
	}

	roster.OurVotingPowerTotalPercentage = ourPercentage
	roster.TheirVotingPowerTotalPercentage = theirPercentage
	for _, slot := range subComm.Slots {
		roster.OrderedSlots = append(roster.OrderedSlots, slot.BLSPublicKey)
	}
	return roster, nil
}

// NewRoster ..
func NewRoster(shardID uint32) *Roster {
	m := map[bls.SerializedPublicKey]*AccommodateHarmonyVote{}
	return &Roster{
		Voters: m,
		topLevelRegistry: topLevelRegistry{
			OurVotingPowerTotalPercentage:   numeric.ZeroDec(),
			TheirVotingPowerTotalPercentage: numeric.ZeroDec(),
			TotalEffectiveStake:             numeric.ZeroDec(),
		},
		ShardID: shardID,
	}
}

// VotePowerByMask return the vote power with the given BLS mask. The result is a number between 0 and 1.
func (r *Roster) VotePowerByMask(mask *bls.Mask) numeric.Dec {
	res := numeric.ZeroDec()

	for key, index := range mask.PublicsIndex {
		if enabled, err := mask.IndexEnabled(index); err != nil || !enabled {
			continue
		}
		voter, ok := r.Voters[key]
		if !ok {
			continue
		}
		res = res.Add(voter.OverallPercent)
	}
	return res
}
