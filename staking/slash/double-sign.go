package slash

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

const (
	haveEnoughToPayOff               = 1
	paidOffExact                     = 0
	debtCollectionsRepoUndelegations = -1
	validatorsOwnDel                 = 0
)

// Moment ..
type Moment struct {
	Epoch        *big.Int `json:"epoch"`
	Height       *big.Int `json:"block-height"`
	TimeUnixNano *big.Int `json:"time-unix-nano"`
	ViewID       uint64   `json:"view-id"`
	ShardID      uint32   `json:"shard-id"`
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
func Verify(chain committee.ChainReader, candidate *Record) error {
	first, second := candidate.AlreadyCastBallot, candidate.DoubleSignedBallot
	if shard.CompareBlsPublicKey(first.SignerPubKey, second.SignerPubKey) != 0 {
		k1, k2 := first.SignerPubKey.Hex(), second.SignerPubKey.Hex()
		return errors.Wrapf(
			errBLSKeysNotEqual, "%s %s", k1, k2,
		)
	}
	superCommittee, err := chain.ReadShardState(candidate.Evidence.Epoch)
	if err != nil {
		return err
	}
	subCommittee := superCommittee.FindCommitteeByID(
		candidate.Evidence.ShardID,
	)
	if subCommittee == nil {
		return errors.Wrapf(
			errShardIDNotKnown, "given shardID %d", candidate.Evidence.ShardID,
		)
	}

	// for _, key := range subCommittee.BLSPublicKeys() {
	// 	if shard.CompareBlsPublicKey(
	// 		shard.FromLibBLSPublicKeyUnsafe(key),
	// 		second.SignerPubKey) == 0 {
	// 		//
	// 	}

	// }

	// candidate.ConflictingBallots

	// TODO Why this one printng have 00000000 for signature?
	fmt.Println("need to verify the slash", candidate)

	return nil
}

var (
	errBLSKeysNotEqual = errors.New(
		"bls keys in ballots accompanying slash evidence not equal ",
	)
	errSlashDebtNotFullyAccountedFor = errors.New(
		"slash debt was not fully accounted for, still non-zero",
	)
	errShardIDNotKnown              = errors.New("nil subcommittee for shardID")
	errValidatorNotFoundDuringSlash = errors.New("validator not found")
	zero                            = numeric.ZeroDec()
	oneDoubleSignerRate             = numeric.MustNewDecFromStr("0.02")
	oneHundredPercent               = numeric.NewDec(1)
	fiftyPercent                    = numeric.MustNewDecFromStr("0.5")
)

// applySlashRate returns (amountPostSlash, amountOfReduction, amountOfReduction / 2)
func applySlashRate(amount *big.Int, rate numeric.Dec) *big.Int {
	return numeric.NewDecFromBigInt(
		amount,
	).Mul(rate).TruncateInt()
}

func delegatorSlashApply(
	snapshot, current *staking.ValidatorWrapper,
	rate numeric.Dec,
	state *state.DB,
	reporter common.Address,
	doubleSignEpoch *big.Int,
	slashTrack *Application,
) error {
	for i, delegationSnapshot := range snapshot.Delegations {
		slashDebt := applySlashRate(delegationSnapshot.Amount, rate)
		halfOfSlashDebt := new(big.Int).Div(slashDebt, common.Big2)
		snapshotAddr := delegationSnapshot.DelegatorAddress
		for j, delegationNow := range current.Delegations {
			if nowAmt := delegationNow.Amount; delegationNow.DelegatorAddress == snapshotAddr {
				state.AddBalance(reporter, halfOfSlashDebt)
				// NOTE only need to pay snitch here
				slashTrack.TotalSnitchReward.Add(
					slashTrack.TotalSnitchReward, halfOfSlashDebt,
				)
				// NOTE handle validator self delegation special case
				if i == validatorsOwnDel && j == validatorsOwnDel {
					// say my slash debt is 1.6, and my current amount is 1.2
					// then while I can't drop below 1, I could still pay 0.2
					// to bring my debt down to 1.4
					if partialPayment := new(big.Int).Sub(
						// 1.2 - 1
						nowAmt, big.NewInt(denominations.One),
						// 0.2 > 0 == true ?
					); partialPayment.Cmp(common.Big0) == 1 {
						// Mutate wrt partial payment application
						slashDebt.Sub(slashDebt, partialPayment)
						nowAmt.Sub(nowAmt, partialPayment)
						slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, partialPayment)
					}
					// NOTE Assume did as much as could above, now check the undelegations
					for _, undelegate := range delegationNow.Undelegations {
						// the epoch matters, only those undelegation
						// such that epoch>= doubleSignEpoch should be slashable
						if undelegate.Epoch.Cmp(doubleSignEpoch) >= 0 {
							if slashDebt.Cmp(common.Big0) <= 0 {
								// paid off the slash debt
								break
							}

							switch newBal := new(big.Int).Sub(
								// My slash debt is 1.6 and my undelegate amount is 1.0
								// so if (1.6 - 1.0) > 0
								undelegate.Amount,
								slashDebt); newBal.Cmp(common.Big0) {
							case paidOffExact:
								undelegate.Amount.SetInt64(0)
							case haveEnoughToPayOff:
								slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, slashDebt)
								undelegate.Amount.Sub(undelegate.Amount, slashDebt)
								slashDebt.SetInt64(0)
								undelegate.Amount.Set(newBal)
							case -1:
								// But do have enough to pay off something at least
								partialPayment := new(big.Int).Sub(slashDebt, new(big.Int).Abs(newBal))
								slashDebt.Sub(slashDebt, partialPayment)
								slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, partialPayment)
								undelegate.Amount.SetInt64(0)
							}
						}
					}
					// NOTE at high slashing % rates, we could hit this situation
					// because of the special casing of logic on min-self-delegation
					if slashDebt.Cmp(common.Big0) == 1 {
						slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, slashDebt)
						slashDebt.Sub(slashDebt, slashDebt)
					}
				} else {
					// NOTE everyone else, that is every plain delegation
					// Current delegation has some money and slashdebt is still not paid off
					// so contribute as much as can with current delegation amount
					if nowAmt.Cmp(common.Big0) == 1 && slashDebt.Cmp(common.Big0) == 1 {
						newBal := new(big.Int).Sub(nowAmt, slashDebt)
						slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, slashDebt)
						slashDebt.Sub(slashDebt, slashDebt)
						nowAmt.Set(newBal)
					}
					// NOTE Assume did as much as could above, now check the undelegations
					for _, undelegate := range delegationNow.Undelegations {
						// the epoch matters, only those undelegation
						// such that epoch>= doubleSignEpoch should be slashable
						if undelegate.Epoch.Cmp(doubleSignEpoch) >= 0 {
							if slashDebt.Cmp(common.Big0) <= 0 {
								// paid off the slash debt
								break
							}

							if amt := undelegate.Amount; amt.Cmp(common.Big0) == 1 &&
								slashDebt.Cmp(common.Big0) == 1 {
								newBal := new(big.Int).Sub(amt, slashDebt)
								slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, slashDebt)
								amt.Set(newBal)
								slashDebt.Sub(slashDebt, slashDebt)
							}
						}
					}
				}
			}
		}

		if slashDebt.Cmp(common.Big0) != 0 {
			return errors.Wrapf(errSlashDebtNotFullyAccountedFor, "amt %v", slashDebt)
		}
	}
	return nil
}

// TODO Need to keep a record in off-chain db of all the slashes?

// Apply ..
func Apply(
	chain staking.ValidatorSnapshotReader, state *state.DB,
	slashes Records, rate numeric.Dec,
) (*Application, error) {
	log := utils.Logger()
	slashDiff := &Application{big.NewInt(0), big.NewInt(0)}
	log.Info().Int("count", len(slashes)).
		Str("rate", rate.String()).
		Msg("apply slashes")

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

		current := state.GetStakingInfo(slash.Offender)
		if current == nil {
			addr := common2.MustAddressToBech32(slash.Offender)
			return nil, errors.Wrapf(
				errValidatorNotFoundDuringSlash, "lookup %s", addr,
			)
		}
		// NOTE invariant: first delegation is the validators own
		// stake, rest are external delegations.
		// Bottom line: everyone must have have skin in the game,
		if err := delegatorSlashApply(
			snapshot, current, rate, state,
			slash.Reporter, slash.Evidence.Epoch, slashDiff,
		); err != nil {
			return nil, err
		}

		// finally, kick them off forever
		current.Banned = true
		if err := state.UpdateStakingInfo(
			snapshot.Address, current,
		); err != nil {
			return nil, err
		}
	}

	log.Info().Str("rate", rate.String()).Int("count", len(slashes))
	return slashDiff, nil
}

// Rate is the slashing % rate
func Rate(doubleSignerCount, committeeSize int) numeric.Dec {
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
