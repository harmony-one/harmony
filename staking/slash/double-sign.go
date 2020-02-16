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

func delegatorSlashApply(
	delegations staking.Delegations,
	rate numeric.Dec,
	state *state.DB,
	reporter common.Address,
	doubleSignEpoch *big.Int,
	slashTrack *Application,
) error {
	for _, delegator := range delegations {
		amt := numeric.NewDecFromBigInt(
			delegator.Amount,
		)
		discounted := amt.Mul(rate).TruncateInt()
		half := numeric.NewDecFromBigInt(
			new(big.Int).Sub(delegator.Amount, discounted),
		).Mul(fiftyPercent)

		paidOff := numeric.ZeroDec()

		// NOTE Get the token trying to escape
		for _, undelegate := range delegator.Undelegations {
			// the epoch matters, only those undelegation
			// such that epoch>= doubleSignEpoch should be slashable
			if undelegate.Epoch.Cmp(doubleSignEpoch) == -1 {
				continue
			}
			if paidOff.Equal(half) {
				break
			}
			undelegate.Amount.SetInt64(0)
			slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, undelegate.Amount)
			paidOff = paidOff.Add(numeric.NewDecFromBigInt(undelegate.Amount))
		}

		halfB := half.TruncateInt()
		state.AddBalance(reporter, halfB)
		slashTrack.TotalSnitchReward.Add(slashTrack.TotalSnitchReward, halfB)

		// we might have had enough in undelegations to satisfy the slash debt
		if paidOff.Equal(half) {
			continue
		}
		// otherwise we need to dig deeper in their pocket
		leftoverDebt := amt.Sub(half.Sub(paidOff)).TruncateInt()
		delegator.Amount.Sub(delegator.Amount, leftoverDebt)
		paidOff.Add(numeric.NewDecFromBigInt(leftoverDebt))
		slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, leftoverDebt)

		if paidOff.Equal(half) {
			fmt.Println("special case when exactly equal", paidOff.String(), half.String())
		}

		if !paidOff.GTE(half) {
			fmt.Println(leftoverDebt.String(), "\n Now have ",
				delegator.Amount, "-had-",
				amt.String(),
				paidOff,
				half,
				delegations.String(),
			)

			return errors.Errorf(
				"paidoff %s is not >= slash debt %s",
				paidOff.String(),
				half.String(),
			)
		}
	}
	fmt.Println("after delegator slash application", slashTrack.String())
	return nil
}

// TODO Need to keep a record in off-chain db of all the slashes?

// Apply ..
func Apply(
	chain staking.ValidatorSnapshotReader, state *state.DB,
	slashes []Record, committee []shard.BlsPublicKey,
) (*Application, error) {
	log := utils.Logger()
	rate := Rate(uint32(len(slashes)), uint32(len(committee)))
	slashDiff := &Application{big.NewInt(0), big.NewInt(0)}

	for _, slash := range slashes {
		// TODO Probably won't happen but we probably should
		// be expilict about reading the right epoch validator snapshot,
		// because it needs to be the epoch of which the double sign
		// occurred
		snapshot, err := chain.ReadValidatorSnapshot(
			slash.Offender,
		)

		if err != nil {
			return nil, errors.Errorf(
				"could not find validator %s",
				common2.MustAddressToBech32(slash.Offender),
			)
		}

		nowAccountState := state.GetStakingInfo(slash.Offender)

		if nowAccountState == nil {
			return nil, errors.New("cannot be nil code field in account state")
		}

		const mustBeValidatorsDelegationToSelf = 1
		// Handle validator's delegation explicitly for sake of clarity
		validatorOwnDelegation :=
			snapshot.Delegations[:mustBeValidatorsDelegationToSelf]
		if err := delegatorSlashApply(
			validatorOwnDelegation, rate, state,
			slash.Reporter, slash.Evidence.Epoch, slashDiff,
		); err != nil {
			return nil, err
		}

		// Now can handle external to the validator delegations, third parties
		// that trusted the validator with some one token, they must also
		// be slashed so they have skin in the game
		rest := snapshot.Delegations[mustBeValidatorsDelegationToSelf:]
		if err := delegatorSlashApply(
			rest, rate, state, slash.Reporter, slash.Evidence.Epoch, slashDiff,
		); err != nil {
			return nil, err
		}
		// ...and I want those that I have not seen before
		sinceSnapshotDelegations :=
			staking.SetDifference(snapshot.Delegations, nowAccountState.Delegations)
		if err := delegatorSlashApply(
			sinceSnapshotDelegations, rate, state,
			slash.Reporter, slash.Evidence.Epoch, slashDiff,
		); err != nil {
			return nil, err
		}
		// finally, kick them off forever
		snapshot.Banned = true

		if err := state.UpdateStakingInfo(
			snapshot.Address, snapshot,
		); err != nil {
			fmt.Println("cannot update wrapper", err.Error())
			return nil, err
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

// DumpBalances ..
func (r Records) DumpBalances(state *state.DB) {
	for _, s := range r {
		oBal, reportBal := state.GetBalance(s.Offender), state.GetBalance(s.Reporter)
		wrap := state.GetStakingInfo(s.Offender)
		fmt.Printf(
			"offender %s \n\tbalance %v, \n\treporter %s balance %v \n",
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
