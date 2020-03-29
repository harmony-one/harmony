package slash

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
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
	TimeUnixNano *big.Int `json:"time-unix-nano"`
	ShardID      uint32   `json:"shard-id"`
}

// Evidence ..
type Evidence struct {
	Moment
	ConflictingBallots
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

// Application tracks the slash application to state
type Application struct {
	TotalSlashed      *big.Int `json:'total-slashed`
	TotalSnitchReward *big.Int `json:"total-snitch-reward"`
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
	}{e.Moment, e.ConflictingBallots})
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
	errAlreadyBannedValidator  = errors.New("cannot slash on already banned validator")
	errSignerKeyNotRightSize   = errors.New("bls keys from slash candidate not right side")
	errSlashFromFutureEpoch    = errors.New("cannot have slash from future epoch")
	errSlashBlockNoConflict    = errors.New("cannot slash for signing on non-conflicting blocks")
)

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
	CurrentBlock() *types.Block
}

// Verify checks that the slash is valid
func Verify(
	chain CommitteeReader,
	state *state.DB,
	candidate *Record,
) error {
	wrapper, err := state.ValidatorWrapper(candidate.Offender)
	if err != nil {
		return err
	}

	if wrapper.Status == effective.Banned {
		return errAlreadyBannedValidator
	}

	if candidate.Offender == candidate.Reporter {
		return errReporterAndOffenderSame
	}

	first, second :=
		candidate.Evidence.AlreadyCastBallot,
		candidate.Evidence.DoubleSignedBallot
	k1, k2 := len(first.SignerPubKey), len(second.SignerPubKey)
	if k1 != shard.PublicKeySizeInBytes ||
		k2 != shard.PublicKeySizeInBytes {
		return errors.Wrapf(
			errSignerKeyNotRightSize, "cast key %d double-signed key %d", k1, k2,
		)
	}

	if first.ViewID != second.ViewID ||
		first.Height != second.Height ||
		first.BlockHeaderHash == second.BlockHeaderHash {
		return errors.Wrapf(errSlashBlockNoConflict, "first %v+ second %v+", first, second)
	}

	if shard.CompareBlsPublicKey(first.SignerPubKey, second.SignerPubKey) != 0 {
		k1, k2 := first.SignerPubKey.Hex(), second.SignerPubKey.Hex()
		return errors.Wrapf(
			errBallotSignerKeysNotSame, "%s %s", k1, k2,
		)
	}
	currentEpoch := chain.CurrentBlock().Epoch()
	// the slash can't come from the future (shard chain's epoch can't be larger than beacon chain's)
	if candidate.Evidence.Epoch.Cmp(currentEpoch) == 1 {
		return errors.Wrapf(
			errSlashFromFutureEpoch, "current-epoch %v", currentEpoch,
		)
	}

	superCommittee, err := chain.ReadShardState(candidate.Evidence.Epoch)

	if err != nil {
		return err
	}

	subCommittee, err := superCommittee.FindCommitteeByID(
		candidate.Evidence.ShardID,
	)

	if err != nil {
		return errors.Wrapf(
			err, "given shardID %d", candidate.Evidence.ShardID,
		)
	}

	if addr, err := subCommittee.AddressForBLSKey(
		second.SignerPubKey,
	); err != nil || *addr != candidate.Offender {
		return err
	}

	// last ditch check
	if hash.FromRLPNew256(
		candidate.Evidence.AlreadyCastBallot,
	) == hash.FromRLPNew256(
		candidate.Evidence.DoubleSignedBallot,
	) {
		return errors.Wrapf(
			errBallotsNotDiff,
			"%s %s",
			candidate.Evidence.AlreadyCastBallot.SignerPubKey.Hex(),
			candidate.Evidence.DoubleSignedBallot.SignerPubKey.Hex(),
		)
	}

	for _, ballot := range [...]votepower.Ballot{
		candidate.Evidence.AlreadyCastBallot,
		candidate.Evidence.DoubleSignedBallot,
	} {
		// now the only real assurance, cryptography
		signature := &bls.Sign{}
		publicKey := &bls.PublicKey{}

		if err := signature.Deserialize(ballot.Signature); err != nil {
			return err
		}
		if err := ballot.SignerPubKey.ToLibBLSPublicKey(publicKey); err != nil {
			return err
		}

		blockNumBytes := make([]byte, 8)
		// TODO(audit): add view ID into signature payload
		binary.LittleEndian.PutUint64(blockNumBytes, ballot.Height)
		commitPayload := append(blockNumBytes, ballot.BlockHeaderHash[:]...)
		if !signature.VerifyHash(publicKey, commitPayload) {
			return errFailVerifySlash
		}
	}

	return nil
}

var (
	errBLSKeysNotEqual = errors.New(
		"bls keys in ballots accompanying slash evidence not equal ",
	)
	errSlashDebtCannotBeNegative    = errors.New("slash debt cannot be negative")
	errValidatorNotFoundDuringSlash = errors.New("validator not found")
	errFailVerifySlash              = errors.New("could not verify bls key signature on slash")
	errBallotsNotDiff               = errors.New("ballots submitted must be different")
	zero                            = numeric.ZeroDec()
	oneDoubleSignerRate             = numeric.MustNewDecFromStr("0.02")
)

// applySlashRate returns (amountPostSlash, amountOfReduction, amountOfReduction / 2)
func applySlashRate(amount *big.Int, rate numeric.Dec) *big.Int {
	return numeric.NewDecFromBigInt(
		amount,
	).Mul(rate).TruncateInt()
}

// Hash is a New256 hash of an RLP encoded Record
func (r Record) Hash() common.Hash {
	return hash.FromRLPNew256(r)
}

// SetDifference returns all the records that are in ys but not in r
func (r Records) SetDifference(ys Records) Records {
	diff, set := Records{}, map[common.Hash]struct{}{}
	for i := range r {
		h := r[i].Hash()
		if _, ok := set[h]; !ok {
			set[h] = struct{}{}
		}
	}

	for i := range ys {
		h := ys[i].Hash()
		if _, ok := set[h]; !ok {
			diff = append(diff, ys[i])
		}
	}

	return diff
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
				utils.Logger().Info().
					RawJSON("delegation-snapshot", []byte(delegationSnapshot.String())).
					RawJSON("delegation-current", []byte(delegationNow.String())).
					Uint64("initial-slash-debt", slashDebt.Uint64()).
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
							utils.Logger().Info().
								RawJSON("delegation-snapshot", []byte(delegationSnapshot.String())).
								RawJSON("delegation-current", []byte(delegationNow.String())).
								Msg("paid off the slash debt")
							break
						}
						nowAmt := undelegate.Amount
						if err := payDownAsMuchAsCan(
							snapshot, current, slashDebt, nowAmt, slashDiff,
						); err != nil {
							return err
						}

						if nowAmt.Cmp(common.Big0) == 0 {
							// TODO(audit): need to remove the undelegate
							utils.Logger().Info().
								RawJSON("delegation-snapshot", []byte(delegationSnapshot.String())).
								RawJSON("delegation-current", []byte(delegationNow.String())).
								Msg("delegation amount after paying slash debt is 0")
						}
					}
				}

				// if we still have a slashdebt
				// even after taking away from delegation amount
				// and even after taking away from undelegate,
				// then we need to take from their pending rewards
				if slashDebt.Cmp(common.Big0) == 1 {
					nowAmt := delegationNow.Reward
					utils.Logger().Info().
						RawJSON("delegation-snapshot", []byte(delegationSnapshot.String())).
						RawJSON("delegation-current", []byte(delegationNow.String())).
						Uint64("slash-debt", slashDebt.Uint64()).
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
				utils.Logger().Info().
					RawJSON("delegation-snapshot", []byte(delegationSnapshot.String())).
					RawJSON("delegation-current", []byte(delegationNow.String())).
					Uint64("reporter-reward", halfOfSlashDebt.Uint64()).
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

// Apply ..
func Apply(
	chain staking.ValidatorSnapshotReader, state *state.DB,
	slashes Records, rate numeric.Dec,
) (*Application, error) {
	slashDiff := &Application{big.NewInt(0), big.NewInt(0)}
	for _, slash := range slashes {
		snapshot, err := chain.ReadValidatorSnapshotAtEpoch(
			slash.Evidence.Epoch,
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
		current.Status = effective.Banned
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
