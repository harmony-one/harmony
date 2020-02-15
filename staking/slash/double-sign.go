package slash

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// Moment ..
type Moment struct {
	Epoch        *big.Int `json:"epoch"`
	Height       *big.Int `json:"block-height"`
	ViewID       uint64   `json:"view-id"`
	ShardID      uint32   `json:"shard-id"`
	TimeUnixNano *big.Int `json:"time-unix-nano"`
}

// Evidence ..
type Evidence struct {
	Moment
	ProposalHeader *block.Header `json:"header"`
}

// ConflictingBallots ..
type ConflictingBallots struct {
	AlreadyCastBallot  votepower.Ballot `json:"already-cast-vote"`
	DoubleSignedBallot votepower.Ballot `json:"double-signed-vote"`
}

// Record is an proof of a slashing made by a witness of a double-signing event
type Record struct {
	ConflictingBallots
	// the reporter who will get rewarded
	Evidence Evidence       `json:"evidence"`
	Reporter common.Address `json:"reporter"`
	Offender common.Address `json:"offender"`
}

// Application ..
type Application struct {
	TotalSlashed, TotalSnitchReward *big.Int
}

func (a *Application) String() string {
	s, _ := json.Marshal(a)
	return string(s)
}

// MarshalJSON ..
func (e Evidence) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Moment
		ProposalHeader string `json:"header"`
	}{e.Moment, e.ProposalHeader.String()})
}

// Records ..
type Records []Record

// func (r Records) MarshalJSON() ([]byte, error) {

// 	return json.Marshal(struct {
// 		Slashes []Record `json:""`
// 	}{r})
// }

func (r Records) String() string {
	s, _ := json.Marshal(r)
	return string(s)
}

// MarshalJSON ..
func (r Record) MarshalJSON() ([]byte, error) {
	reporter, offender :=
		common2.MustAddressToBech32(r.Reporter),
		common2.MustAddressToBech32(r.Offender)
	return json.Marshal(struct {
		ConflictingBallots
		Evidence         Evidence `json:"evidence"`
		Beneficiary      string   `json:"beneficiary"`
		AddressForBLSKey string   `json:"offender"`
	}{r.ConflictingBallots, r.Evidence, reporter, offender})
}

func (e Evidence) String() string {
	s, _ := json.Marshal(e)
	return string(s)
}

func (r Record) String() string {
	s, _ := json.Marshal(r)
	return string(s)
}

// Verify checks that the signature is valid
func Verify(candidate *Record) error {
	fmt.Println("need to verify the slash", candidate)

	return nil
}

var (
	oneHundredPercent = numeric.NewDec(1)
	fiftyPercent      = numeric.MustNewDecFromStr("0.5")
)

func validatorSlashApply(
	wrapper *staking.ValidatorWrapper,
	rate numeric.Dec,
	state *state.DB,
	reporter common.Address,
	slashTrack *Application,
) *staking.ValidatorWrapper {
	// Kick them off forever
	wrapper.Banned = true
	// What is the post reduced amount
	discounted := numeric.NewDecFromBigInt(
		wrapper.MinSelfDelegation,
	).Mul(rate).TruncateInt()
	// The slashed token will be half burned and
	// half credited to the reporter’s balance.
	half := numeric.NewDecFromBigInt(
		new(big.Int).Sub(wrapper.MinSelfDelegation, discounted),
	).Mul(fiftyPercent).TruncateInt()
	state.SubBalance(wrapper.Address, half)
	state.AddBalance(reporter, half)
	slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, half)
	slashTrack.TotalSnitchReward.Add(slashTrack.TotalSnitchReward, half)
	wrapper.MinSelfDelegation = discounted
	fmt.Println("after validator slash application", slashTrack.String())
	return wrapper
}

func delegatorSlashApply(
	wrapper *staking.ValidatorWrapper,
	rate numeric.Dec,
	state *state.DB,
	reporter common.Address,
	slashTrack *Application,
) *staking.ValidatorWrapper {
	for _, delegator := range wrapper.Delegations {
		discounted := numeric.NewDecFromBigInt(
			delegator.Amount,
		).Mul(rate).TruncateInt()
		half := numeric.NewDecFromBigInt(
			new(big.Int).Sub(delegator.Amount, discounted),
		).Mul(fiftyPercent).TruncateInt()
		state.SubBalance(wrapper.Address, half)
		state.AddBalance(reporter, half)
		slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, half)
		slashTrack.TotalSnitchReward.Add(slashTrack.TotalSnitchReward, half)
		delegator.Amount = discounted
	}
	fmt.Println("after delegator slash application", slashTrack.String())
	return wrapper
}

// DumpBalances ..
func (r Records) DumpBalances(state *state.DB) {
	for _, s := range r {
		oBal, reportBal := state.GetBalance(s.Offender), state.GetBalance(s.Reporter)
		wrap := state.GetStakingInfo(s.Offender)
		fmt.Printf(
			"offender %s balance %v, reporter %s balance %v \n",
			common2.MustAddressToBech32(s.Offender),
			oBal,
			common2.MustAddressToBech32(s.Reporter),
			reportBal,
		)
		for _, deleg := range wrap.Delegations {
			fmt.Printf("\tdelegator %s bal %v\n",
				common2.MustAddressToBech32(deleg.DelegatorAddress),
				state.GetBalance(deleg.DelegatorAddress),
			)
		}
	}
}

// TODO Need to keep a record in off-chain db of all the slashes?

// Apply ..
func Apply(
	state *state.DB, slashes []Record, committee []shard.BlsPublicKey,
) (*Application, error) {
	log := utils.Logger()
	rate := Rate(uint32(len(slashes)), uint32(len(committee)))
	postSlashAmount := oneHundredPercent.Sub(rate)
	slashDiff := &Application{big.NewInt(0), big.NewInt(0)}

	for _, slash := range slashes {
		if wrapper := state.GetStakingInfo(
			slash.Offender,
		); wrapper != nil {
			if err := state.UpdateStakingInfo(
				slash.Offender,
				validatorSlashApply(
					delegatorSlashApply(
						wrapper, postSlashAmount, state, slash.Reporter, slashDiff,
					),
					postSlashAmount, state, slash.Reporter, slashDiff,
				),
			); err != nil {
				fmt.Println("cannot update wrapper", err.Error())
				return nil, err
			}
		} else {
			fmt.Printf(
				"could not find validator %s\n",
				common2.MustAddressToBech32(slash.Offender),
			)
			return nil, errors.Errorf(
				"could not find validator %s",
				common2.MustAddressToBech32(slash.Offender),
			)
		}
	}
	log.Info().Str("rate", rate.String()).Int("count", len(slashes))
	fmt.Println("applying slash with a rate of", rate, slashes, slashDiff.String())
	return slashDiff, nil
}

var (
	zero                = numeric.ZeroDec()
	oneDoubleSignerRate = numeric.MustNewDecFromStr("0.02")
)

// Rate is the slashing % rate
func Rate(doubleSignerCount, committeeSize uint32) numeric.Dec {
	if doubleSignerCount == 0 || committeeSize == 0 {
		return zero
	}
	switch doubleSignerCount {
	case 1:
		return oneDoubleSignerRate
	default:
		return numeric.NewDec(
			int64(doubleSignerCount),
		).Quo(numeric.NewDec(int64(committeeSize)))
	}
}
