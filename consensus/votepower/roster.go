package votepower

import (
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
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
	BallotBox      map[string]*Ballot
}

func (b Ballot) String() string {
	data, _ := json.Marshal(b)
	return string(data)
}

// NewRound ..
func NewRound() *Round {
	return &Round{AggregatedVote: nil, BallotBox: map[string]*Ballot{}}
}

type stakedVoter struct {
	IsActive         bool               `json:"is-active"`
	IsHarmonyNode    bool               `json:"is-harmony"`
	EarningAccount   common.Address     `json:"earning-account"`
	Identity         shard.BlsPublicKey `json:"bls-public-key"`
	RawPercent       numeric.Dec        `json:"voting-power-unnormalized"`
	EffectivePercent numeric.Dec        `json:"voting"`
	EffectiveStake   numeric.Dec        `json:"effective-stake"`
}

// Roster ..
type Roster struct {
	Voters                          map[shard.BlsPublicKey]stakedVoter
	OurVotingPowerTotalPercentage   numeric.Dec
	TheirVotingPowerTotalPercentage numeric.Dec
	RawStakedTotal                  numeric.Dec
	HmySlotCount                    int64
}

// Staker ..
type Staker struct {
	TotalEffectiveStake numeric.Dec
	VotingPower         []staking.VotePerShard
	BLSPublicKeysOwned  []staking.KeysPerShard
}

// RosterPerShard ..
type RosterPerShard struct {
	ShardID uint32
	Record  *Roster
}

// AggregateRosters ..
func AggregateRosters(rosters []RosterPerShard) map[common.Address]Staker {
	result := map[common.Address]Staker{}
	sort.SliceStable(rosters,
		func(i, j int) bool { return rosters[i].ShardID < rosters[j].ShardID },
	)

	for _, roster := range rosters {
		for key, value := range roster.Record.Voters {
			if !value.IsHarmonyNode {
				payload, alreadyExists := result[value.EarningAccount]
				if alreadyExists {
					payload.TotalEffectiveStake = payload.TotalEffectiveStake.Add(
						value.EffectiveStake,
					)
					payload.VotingPower = append(payload.VotingPower,
						staking.VotePerShard{
							ShardID:             roster.ShardID,
							VotingPowerRaw:      value.RawPercent,
							VotingPowerAdjusted: value.EffectivePercent,
							EffectiveStake:      value.EffectiveStake,
						},
					)
					for i := range payload.BLSPublicKeysOwned {
						if payload.BLSPublicKeysOwned[i].ShardID == roster.ShardID {
							payload.BLSPublicKeysOwned[i].Keys = append(
								payload.BLSPublicKeysOwned[i].Keys, key,
							)
						}
					}
				} else {
					result[value.EarningAccount] = Staker{
						TotalEffectiveStake: value.EffectiveStake,
						VotingPower: []staking.VotePerShard{
							{roster.ShardID, value.RawPercent,
								value.EffectivePercent, value.EffectiveStake},
						},
						BLSPublicKeysOwned: []staking.KeysPerShard{
							{roster.ShardID, []shard.BlsPublicKey{key}}},
					}
				}
			}
		}
	}

	return result
}

// JSON dump
func (r *Roster) JSON() string {
	v := map[string]stakedVoter{}
	for k, value := range r.Voters {
		v[k.Hex()] = value
	}
	c := struct {
		Voters map[string]stakedVoter `json:"voters"`
		Our    string                 `json:"ours"`
		Their  string                 `json:"theirs"`
		Raw    string                 `json:"raw-total"`
	}{
		v,
		r.OurVotingPowerTotalPercentage.String(),
		r.TheirVotingPowerTotalPercentage.String(),
		r.RawStakedTotal.String(),
	}
	b, _ := json.Marshal(&c)
	return string(b)
}

// Compute creates a new roster based off the shard.SlotList
func Compute(staked shard.SlotList) (*Roster, error) {
	roster := NewRoster()
	for i := range staked {
		if staked[i].EffectiveStake == nil {
			roster.HmySlotCount++
		} else {
			roster.RawStakedTotal = roster.RawStakedTotal.Add(
				*staked[i].EffectiveStake,
			)
		}
	}
	// TODO Check for duplicate BLS Keys
	ourCount := numeric.NewDec(roster.HmySlotCount)
	ourPercentage := numeric.ZeroDec()
	theirPercentage := numeric.ZeroDec()
	var lastStakedVoter *stakedVoter

	for i := range staked {
		member := stakedVoter{
			IsActive:         true,
			IsHarmonyNode:    true,
			EarningAccount:   staked[i].EcdsaAddress,
			Identity:         staked[i].BlsPublicKey,
			RawPercent:       numeric.ZeroDec(),
			EffectivePercent: numeric.ZeroDec(),
			EffectiveStake:   numeric.ZeroDec(),
		}

		// Real Staker
		if staked[i].EffectiveStake != nil {
			member.IsHarmonyNode = false
			member.EffectiveStake = member.EffectiveStake.Add(*staked[i].EffectiveStake)
			member.RawPercent = staked[i].EffectiveStake.Quo(roster.RawStakedTotal)
			member.EffectivePercent = member.RawPercent.Mul(StakersShare)
			theirPercentage = theirPercentage.Add(member.EffectivePercent)
			lastStakedVoter = &member
		} else { // Our node
			member.EffectivePercent = HarmonysShare.Quo(ourCount)
			member.RawPercent = member.EffectivePercent.Quo(HarmonysShare)
			ourPercentage = ourPercentage.Add(member.EffectivePercent)
		}

		roster.Voters[staked[i].BlsPublicKey] = member
	}

	// NOTE Enforce voting power sums to one, give diff (expect tiny amt) to last staked voter
	if diff := numeric.OneDec().Sub(
		ourPercentage.Add(theirPercentage),
	); !diff.IsZero() && lastStakedVoter != nil {
		lastStakedVoter.EffectivePercent = lastStakedVoter.EffectivePercent.Add(diff)
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
	return &Roster{
		map[shard.BlsPublicKey]stakedVoter{},
		numeric.ZeroDec(),
		numeric.ZeroDec(),
		numeric.ZeroDec(),
		0,
	}
}
