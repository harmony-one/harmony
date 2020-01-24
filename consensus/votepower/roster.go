package votepower

import (
	"encoding/json"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/utils"
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
	SignerPubKey      shard.BlsPublicKey `json:"bls-public-key"`
	BlockLeader       shard.BlsPublicKey `json:"leader-when-signed"`
	BlockHeightHeight uint64             `json:"block-height"`
	Signature         *bls.Sign          `json:"signature"`
}

// Round is a round of voting in any FBFT phase
type Round struct {
	AggregatedVote *bls.Sign
	BallotBox      map[string]*Ballot
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
						staking.VotePerShard{roster.ShardID, value.EffectivePercent},
					)
					for i := range payload.BLSPublicKeysOwned {
						if payload.BLSPublicKeysOwned[i].ShardID == roster.ShardID {
							payload.BLSPublicKeysOwned[roster.ShardID].Keys = append(
								payload.BLSPublicKeysOwned[roster.ShardID].Keys, key,
							)
						}
					}
				} else {
					result[value.EarningAccount] = Staker{
						TotalEffectiveStake: value.EffectiveStake,
						VotingPower: []staking.VotePerShard{
							{roster.ShardID, value.EffectivePercent},
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
			EffectivePercent: numeric.ZeroDec(),
			EffectiveStake:   numeric.ZeroDec(),
		}

		// Real Staker
		if staked[i].EffectiveStake != nil {
			member.IsHarmonyNode = false
			member.EffectiveStake = member.EffectiveStake.Add(*staked[i].EffectiveStake)
			member.EffectivePercent = staked[i].EffectiveStake.
				Quo(roster.RawStakedTotal).
				Mul(StakersShare)
			theirPercentage = theirPercentage.Add(member.EffectivePercent)
			lastStakedVoter = &member
		} else { // Our node
			member.EffectivePercent = HarmonysShare.Quo(ourCount)
			ourPercentage = ourPercentage.Add(member.EffectivePercent)
		}

		roster.Voters[staked[i].BlsPublicKey] = member
	}

	// NOTE Enforce voting power sums to one, give diff (expect tiny amt) to last staked voter
	if diff := numeric.OneDec().Sub(
		ourPercentage.Add(theirPercentage),
	); !diff.IsZero() && lastStakedVoter != nil {
		utils.Logger().Info().
			Str("theirs", theirPercentage.String()).
			Str("ours", ourPercentage.String()).
			Str("diff", diff.String()).
			Str("combined", theirPercentage.Add(diff).Add(ourPercentage).String()).
			Str("bls-public-key-of-receipent", lastStakedVoter.Identity.Hex()).
			Msg("voting power of hmy & staked slots not sum to 1, giving diff to staked slot")
		lastStakedVoter.EffectivePercent = lastStakedVoter.EffectivePercent.Add(diff)
		theirPercentage = theirPercentage.Add(diff)
	}

	if lastStakedVoter != nil &&
		!ourPercentage.Add(theirPercentage).Equal(numeric.OneDec()) {
		utils.Logger().Error().
			Str("theirs", theirPercentage.String()).
			Str("ours", ourPercentage.String()).
			Msg("Total voting power not equal 100 percent")
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
