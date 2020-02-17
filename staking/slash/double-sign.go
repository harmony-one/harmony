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

// applySlashRate returns (amountPostSlash, amountOfReduction, amountOfReduction / 2)
func applySlashRate(
	amount *big.Int, rate numeric.Dec,
) (*big.Int, *big.Int, *big.Int) {
	amountPostSlash := numeric.NewDecFromBigInt(
		amount,
	).Mul(numeric.OneDec().Sub(rate)).TruncateInt()
	amountOfSlash := new(big.Int).Sub(amount, amountPostSlash)
	return amountPostSlash, amountOfSlash, new(big.Int).Div(amountOfSlash, common.Big2)
}

// postSlashBalanceCurrentDelegator applySlashRate
// returns (amountPostSlash, amountOfReduction, amountOfReduction / 2)
func postSlashBalanceCurrentDelegator(
	state *state.DB,
	validatorAddr common.Address,
	delegatorAddr common.Address,
	rate numeric.Dec,
) (*big.Int, *big.Int, *big.Int) {
	if wrapper := state.GetStakingInfo(validatorAddr); wrapper != nil {
		for _, delegation := range wrapper.Delegations {
			if delegation.DelegatorAddress == delegatorAddr {
				return applySlashRate(delegation.Amount, rate)
			}
		}
	}
	fmt.Println("invariant violated, should not come here")
	return nil, nil, nil
}

func delegatorSlashApply(
	delegatedToValidator *staking.ValidatorWrapper,
	rate numeric.Dec,
	state *state.DB,
	reporter common.Address,
	doubleSignEpoch *big.Int,
	slashTrack *Application,
) error {
	for _, delegationSnapshot := range delegatedToValidator.Delegations {
		postSlashBal, slashDebt, halfOfSlashDebt := postSlashBalanceCurrentDelegator(
			state, delegatedToValidator.Address,
			delegationSnapshot.DelegatorAddress, rate,
		)
		if postSlashBal.Sign() == -1 {
			stillOwe := new(big.Int).Sub(slashDebt, delegationSnapshot.Amount)
			if wrapper := state.GetStakingInfo(
				delegatedToValidator.Address,
			); wrapper != nil {
				for _, delegationNow := range wrapper.Delegations {
					if delegationNow.Hash() == delegationSnapshot.Hash() {
						for _, undelegate := range delegationNow.Undelegations {
							// the epoch matters, only those undelegation
							// such that epoch>= doubleSignEpoch should be slashable
							if undelegate.Epoch.Cmp(doubleSignEpoch) == -1 {
								continue
							}
							if stillOwe.Cmp(common.Big0) <= 0 {
								// paid off the slash debt
								break
							}
							amtPostSlash, amtSlash, halfOfSlashAmt := applySlashRate(
								undelegate.Amount, rate,
							)
							undelegate.Amount = amtPostSlash
							stillOwe.Sub(stillOwe, halfOfSlashAmt)
							state.AddBalance(reporter, halfOfSlashAmt)
							slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, amtSlash)
							slashTrack.TotalSnitchReward.Add(slashTrack.TotalSnitchReward, halfOfSlashAmt)
						}
					}
				}
				if stillOwe.Cmp(common.Big0) == 1 {
					// NOTE is this an error?
					fmt.Println("Still owe a slash debt", stillOwe)
				}
			}
		}

		// NOTE Burn other half implicitly but not adding half again to anywhere
		state.AddBalance(reporter, halfOfSlashDebt)
		delegationSnapshot.Amount = postSlashBal
		slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, slashDebt)
		slashTrack.TotalSnitchReward.Add(slashTrack.TotalSnitchReward, halfOfSlashDebt)
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

		// NOTE invariant: first delegation is the validators own
		// stake, rest are external delegations.
		// Bottom line: everyone must have have skin in the game,
		if err := delegatorSlashApply(
			snapshot, rate, state,
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
