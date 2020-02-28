package slash

import (
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
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

const (
	haveEnoughToPayOff               = 1
	paidOffExact                     = 0
	debtCollectionsRepoUndelegations = -1
	validatorsOwnDel                 = 0
)

// invariant assumes snapshot, current can be rlp.EncodeToBytes
func payDebt(
	snapshot, current *staking.ValidatorWrapper,
	slashDebt, payment *big.Int,
	slashDiff *Application,
) error {
	utils.Logger().Info().
		RawJSON("snapshot", []byte(snapshot.String())).
		RawJSON("current", []byte(current.String())).
		Uint64("slash-debt", slashDebt.Uint64()).
		Uint64("payment", payment.Uint64()).
		RawJSON("slash-track", []byte(slashDiff.String())).
		Msg("slash debt payment before application")
	slashDiff.TotalSlashed.Add(slashDiff.TotalSlashed, payment)
	slashDebt.Sub(slashDebt, payment)
	if slashDebt.Cmp(common.Big0) == -1 {
		x1, _ := rlp.EncodeToBytes(snapshot)
		x2, _ := rlp.EncodeToBytes(current)
		utils.Logger().Info().
			Str("snapshot-rlp", hex.EncodeToString(x1)).
			Str("current-rlp", hex.EncodeToString(x2)).
			Msg("slashdebt balance cannot go below zero")
		return errSlashDebtCannotBeNegative
	}
	return nil
}

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
	ConflictingBallots
	ProposalHeader *block.Header `json:"header"`
}

// ConflictingBallots ..
type ConflictingBallots struct {
	AlreadyCastBallot  votepower.Ballot `json:"already-cast-vote"`
	DoubleSignedBallot votepower.Ballot `json:"double-signed-vote"`
}

// Record is an proof of a slashing made by a witness of a double-signing event
type Record struct {
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
		ConflictingBallots
		ProposalHeader string `json:"header"`
	}{e.Moment, e.ConflictingBallots, e.ProposalHeader.String()})
}

// Records ..
type Records []Record

func (r Records) String() string {
	s, _ := json.Marshal(r)
	return string(s)
}

var (
	errBallotSignerKeysNotSame = errors.New("conflicting ballots must have same signer key")
	errReporterAndOffenderSame = errors.New("reporter and offender cannot be same")
)

// SanityCheck fails if any of the slashes fail
func (r Records) SanityCheck() error {
	for _, record := range r {
		k1 := record.Evidence.AlreadyCastBallot.SignerPubKey
		k2 := record.Evidence.DoubleSignedBallot.SignerPubKey
		if k1 != k2 {
			return errBallotSignerKeysNotSame
		}

		if record.Offender == record.Reporter {
			return errReporterAndOffenderSame
		}

	}
	return nil
}

// MarshalJSON ..
func (r Record) MarshalJSON() ([]byte, error) {
	reporter, offender :=
		common2.MustAddressToBech32(r.Reporter),
		common2.MustAddressToBech32(r.Offender)
	return json.Marshal(struct {
		Evidence         Evidence `json:"evidence"`
		Beneficiary      string   `json:"beneficiary"`
		AddressForBLSKey string   `json:"offender"`
	}{r.Evidence, reporter, offender})
}

func (e Evidence) String() string {
	s, _ := json.Marshal(e)
	return string(s)
}

func (r Record) String() string {
	s, _ := json.Marshal(r)
	return string(s)
}

// CommitteeReader ..
type CommitteeReader interface {
	ReadShardState(epoch *big.Int) (*shard.State, error)
}

// Verify checks that the signature is valid
func Verify(chain CommitteeReader, candidate *Record) error {
	first, second :=
		candidate.Evidence.AlreadyCastBallot,
		candidate.Evidence.DoubleSignedBallot
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

	if _, err := subCommittee.AddressForBLSKey(second.SignerPubKey); err != nil {
		return err
	}
	// TODO need to finish this implementation
	return nil
}

var (
	errBLSKeysNotEqual = errors.New(
		"bls keys in ballots accompanying slash evidence not equal ",
	)
	errSlashDebtCannotBeNegative    = errors.New("slash debt cannot be negative")
	errShardIDNotKnown              = errors.New("nil subcommittee for shardID")
	errValidatorNotFoundDuringSlash = errors.New("validator not found")
	zero                            = numeric.ZeroDec()
	oneDoubleSignerRate             = numeric.MustNewDecFromStr("0.02")
)

// applySlashRate returns (amountPostSlash, amountOfReduction, amountOfReduction / 2)
func applySlashRate(amount *big.Int, rate numeric.Dec) *big.Int {
	return numeric.NewDecFromBigInt(
		amount,
	).Mul(rate).TruncateInt()
}

func payDownAsMuchAsCan(
	snapshot, current *staking.ValidatorWrapper,
	slashDebt, nowAmt *big.Int,
	slashDiff *Application,
) error {
	if nowAmt.Cmp(common.Big0) == 1 && slashDebt.Cmp(common.Big0) == 1 {
		// 0.50_amount > 0.06_debt => slash == 0.0, nowAmt == 0.44
		if nowAmt.Cmp(slashDebt) >= 0 {
			nowAmt.Sub(nowAmt, slashDebt)
			if err := payDebt(
				snapshot, current, slashDebt, slashDebt, slashDiff,
			); err != nil {
				return err
			}
		} else {
			// 0.50_amount < 2.4_debt =>, slash == 1.9, nowAmt == 0.0
			if err := payDebt(
				snapshot, current, slashDebt, nowAmt, slashDiff,
			); err != nil {
				return err
			}
			nowAmt.Sub(nowAmt, nowAmt)
		}
	}

	return nil
}

func delegatorSlashApply(
	snapshot, current *staking.ValidatorWrapper,
	rate numeric.Dec,
	state *state.DB,
	reporter common.Address,
	doubleSignEpoch *big.Int,
	slashTrack *Application,
) error {

	for _, delegationSnapshot := range snapshot.Delegations {
		slashDebt := applySlashRate(delegationSnapshot.Amount, rate)
		slashDiff := &Application{big.NewInt(0), big.NewInt(0)}
		snapshotAddr := delegationSnapshot.DelegatorAddress
		for _, delegationNow := range current.Delegations {
			if nowAmt := delegationNow.Amount; delegationNow.DelegatorAddress == snapshotAddr {
				l := utils.Logger().Info().RawJSON("delegation-snapshot", []byte(delegationSnapshot.String())).
					RawJSON("delegation-current", []byte(delegationNow.String()))

				l.Uint64("initial-slash-debt", slashDebt.Uint64()).
					Str("rate", rate.String()).
					Msg("attempt to apply slashing based on snapshot amount to current state")
				// Current delegation has some money and slashdebt is still not paid off
				// so contribute as much as can with current delegation amount
				if err := payDownAsMuchAsCan(
					snapshot, current, slashDebt, nowAmt, slashDiff,
				); err != nil {
					return err
				}

				// NOTE Assume did as much as could above, now check the undelegations
				for _, undelegate := range delegationNow.Undelegations {
					// the epoch matters, only those undelegation
					// such that epoch>= doubleSignEpoch should be slashable
					if undelegate.Epoch.Cmp(doubleSignEpoch) >= 0 {
						if slashDebt.Cmp(common.Big0) <= 0 {
							l.Msg("paid off the slash debt")
							break
						}
						nowAmt := undelegate.Amount
						if err := payDownAsMuchAsCan(
							snapshot, current, slashDebt, nowAmt, slashDiff,
						); err != nil {
							return err
						}

						if nowAmt.Cmp(common.Big0) == 0 {
							// TODO need to remove the undelegate
							l.Msg("delegation amount after paying slash debt is 0")
						}
					}
				}

				// if we still have a slashdebt
				// even after taking away from delegation amount
				// and even after taking away from undelegate,
				// then we need to take from their pending rewards
				if slashDebt.Cmp(common.Big0) == 1 {
					nowAmt := delegationNow.Reward
					l.Uint64("slash-debt", slashDebt.Uint64()).
						Uint64("now-amount-reward", nowAmt.Uint64()).
						Msg("needed to dig into reward to pay off slash debt")
					if err := payDownAsMuchAsCan(
						snapshot, current, slashDebt, nowAmt, slashDiff,
					); err != nil {
						return err
					}
				}

				// NOTE only need to pay snitch here,
				// they only get half of what was actually dispersed
				halfOfSlashDebt := new(big.Int).Div(slashDiff.TotalSlashed, common.Big2)
				slashDiff.TotalSnitchReward.Add(slashDiff.TotalSnitchReward, halfOfSlashDebt)
				l.Uint64("reporter-reward", halfOfSlashDebt.Uint64()).
					RawJSON("application", []byte(slashDiff.String())).
					Msg("completed an application of slashing")
				state.AddBalance(reporter, halfOfSlashDebt)
				slashTrack.TotalSnitchReward.Add(
					slashTrack.TotalSnitchReward, slashDiff.TotalSnitchReward,
				)
				slashTrack.TotalSlashed.Add(
					slashTrack.TotalSlashed, slashDiff.TotalSlashed,
				)
			}
		}
		// after the loops, paid off as much as could
		if slashDebt.Cmp(common.Big0) == -1 {
			x1, _ := rlp.EncodeToBytes(snapshot)
			x2, _ := rlp.EncodeToBytes(current)
			utils.Logger().Error().Str("slash-rate", rate.String()).
				Str("snapshot-rlp", hex.EncodeToString(x1)).
				Str("current-rlp", hex.EncodeToString(x2)).
				Msg("slash debt not paid off")
			return errors.Wrapf(errSlashDebtCannotBeNegative, "amt %v", slashDebt)
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

		current, err := state.ValidatorWrapper(slash.Offender)
		if err != nil {
			return nil, errors.Wrapf(
				errValidatorNotFoundDuringSlash, " %s ", err.Error(),
			)
		}
		// NOTE invariant: first delegation is the validators own
		// stake, rest are external delegations.
		// Bottom line: everyone will be slashed under the same rule.
		if err := delegatorSlashApply(
			snapshot, current, rate, state,
			slash.Reporter, slash.Evidence.Epoch, slashDiff,
		); err != nil {
			return nil, err
		}

		// finally, kick them off forever
		current.Banned, current.Active = true, false
		utils.Logger().Info().
			RawJSON("delegation-current", []byte(current.String())).
			RawJSON("slash", []byte(slash.String())).
			Msg("about to update staking info for a validator after a slash")

		if err := state.UpdateValidatorWrapper(
			snapshot.Address, current,
		); err != nil {
			return nil, err
		}
	}
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
