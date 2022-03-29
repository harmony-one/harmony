package slash

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/shard"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
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
	Epoch   *big.Int `json:"epoch"`
	ShardID uint32   `json:"shard-id"`
	Height  uint64   `json:"height"`
	ViewID  uint64   `json:"view-id"`
}

// Evidence ..
type Evidence struct {
	Moment
	ConflictingVotes
	Offender common.Address `json:"offender"`
}

// ConflictingVotes ..
type ConflictingVotes struct {
	FirstVote  Vote `json:"first-vote"`
	SecondVote Vote `json:"second-vote"`
}

// Vote is the vote of the double signer
type Vote struct {
	SignerPubKeys   []bls.SerializedPublicKey `json:"bls-public-keys"`
	BlockHeaderHash common.Hash               `json:"block-header-hash"`
	Signature       []byte                    `json:"bls-signature"`
}

// Record is an proof of a slashing made by a witness of a double-signing event
type Record struct {
	// the reporter who will get rewarded
	Evidence Evidence       `json:"evidence"`
	Reporter common.Address `json:"reporter"`
}

// Application tracks the slash application to state
type Application struct {
	TotalSlashed           *big.Int `json:"total-slashed"`
	TotalBeneficiaryReward *big.Int `json:"total-beneficiary-reward"`
}

func (a *Application) String() string {
	s, _ := json.Marshal(a)
	return string(s)
}

// MarshalJSON ..
func (e Evidence) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Moment
		ConflictingVotes
	}{e.Moment, e.ConflictingVotes})
}

// Records ..
type Records []Record

func (r Records) String() string {
	s, _ := json.Marshal(r)
	return string(s)
}

var (
	errNoMatchingDoubleSignKeys = errors.New("no matching double sign keys")
	errReporterAndOffenderSame  = errors.New("reporter and offender cannot be same")
	errAlreadyBannedValidator   = errors.New("cannot slash on already banned validator")
	errSignerKeyNotRightSize    = errors.New("bls keys from slash candidate not right side")
	errSlashFromFutureEpoch     = errors.New("cannot have slash from future epoch")
	errSlashBeforeStakingEpoch  = errors.New("cannot have slash before staking epoch")
	errSlashBlockNoConflict     = errors.New("cannot slash for signing on non-conflicting blocks")
)

// MarshalJSON ..
func (r Record) MarshalJSON() ([]byte, error) {
	reporter, offender :=
		common2.MustAddressToBech32(r.Reporter),
		common2.MustAddressToBech32(r.Evidence.Offender)
	return json.Marshal(struct {
		Evidence         Evidence `json:"evidence"`
		Reporter         string   `json:"reporter"`
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

// Verify checks that the slash is valid
func Verify(
	chain CommitteeReader,
	state *state.DB,
	candidate *Record,
) error {
	wrapper, err := state.ValidatorWrapper(candidate.Evidence.Offender, true, false)
	if err != nil {
		return err
	}

	if !chain.Config().IsStaking(candidate.Evidence.Epoch) {
		return errSlashBeforeStakingEpoch
	}

	if wrapper.Status == effective.Banned {
		return errAlreadyBannedValidator
	}

	if candidate.Evidence.Offender == candidate.Reporter {
		return errReporterAndOffenderSame
	}

	first, second :=
		candidate.Evidence.FirstVote,
		candidate.Evidence.SecondVote

	for _, pubKey := range append(first.SignerPubKeys, second.SignerPubKeys...) {
		if len(pubKey) != bls.PublicKeySizeInBytes {
			return errors.Wrapf(
				errSignerKeyNotRightSize, "double-signed key %x", pubKey,
			)
		}
	}

	if first.BlockHeaderHash == second.BlockHeaderHash {
		return errors.Wrapf(errSlashBlockNoConflict, "first %v+ second %v+", first, second)
	}

	doubleSignKeys := []bls.SerializedPublicKey{}
	for _, pubKey1 := range first.SignerPubKeys {
		for _, pubKey2 := range second.SignerPubKeys {
			if shard.CompareBLSPublicKey(pubKey1, pubKey2) == 0 {
				doubleSignKeys = append(doubleSignKeys, pubKey1)
				break
			}
		}
	}

	if len(doubleSignKeys) == 0 {
		return errNoMatchingDoubleSignKeys
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

	signerFound := false
	addrs := []common.Address{}
	for _, pubKey := range doubleSignKeys {
		addr, err := subCommittee.AddressForBLSKey(
			pubKey,
		)
		if err != nil {
			return err
		}

		if *addr == candidate.Evidence.Offender {
			signerFound = true
		}
		addrs = append(addrs, *addr)
	}

	if len(addrs) > 1 {
		return errors.Errorf("multiple double signer addresses found (%x) ", addrs)
	}
	if !signerFound {
		return errors.Errorf("offender address (%x) does not match the signer's address (%x)", candidate.Evidence.Offender, addrs)
	}
	// last ditch check
	if hash.FromRLPNew256(
		candidate.Evidence.FirstVote,
	) == hash.FromRLPNew256(
		candidate.Evidence.SecondVote,
	) {
		return errors.Wrapf(
			errBallotsNotDiff,
			"%s %s",
			candidate.Evidence.FirstVote.SignerPubKeys,
			candidate.Evidence.SecondVote.SignerPubKeys,
		)
	}

	for _, ballot := range [...]Vote{
		candidate.Evidence.FirstVote,
		candidate.Evidence.SecondVote,
	} {
		// now the only real assurance, cryptography
		signature := &bls_core.Sign{}
		publicKey := &bls_core.PublicKey{}

		if err := signature.Deserialize(ballot.Signature); err != nil {
			return err
		}

		for _, pubKey := range ballot.SignerPubKeys {
			publicKeyObj, err := bls.BytesToBLSPublicKey(pubKey[:])

			if err != nil {
				return err
			}
			publicKey.Add(publicKeyObj)
		}
		// slash verification only happens in staking era, therefore want commit payload for staking epoch
		commitPayload := consensus_sig.ConstructCommitPayload(chain,
			candidate.Evidence.Epoch, ballot.BlockHeaderHash, candidate.Evidence.Height, candidate.Evidence.ViewID)
		utils.Logger().Debug().
			Uint64("epoch", candidate.Evidence.Epoch.Uint64()).
			Uint64("block-number", candidate.Evidence.Height).
			Uint64("view-id", candidate.Evidence.ViewID).
			Msgf("[COMMIT-PAYLOAD] doubleSignVerify %v", hex.EncodeToString(commitPayload))

		if !signature.VerifyHash(publicKey, commitPayload) {
			return errFailVerifySlash
		}
	}

	return nil
}

var (
	errSlashDebtCannotBeNegative    = errors.New("slash debt cannot be negative")
	errValidatorNotFoundDuringSlash = errors.New("validator not found")
	errFailVerifySlash              = errors.New("could not verify bls key signature on slash")
	errBallotsNotDiff               = errors.New("ballots submitted must be different")
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
	rewardBeneficiary common.Address,
	doubleSignEpoch *big.Int,
	slashTrack *Application,
) error {

	for _, delegationSnapshot := range snapshot.Delegations {
		slashDebt := applySlashRate(delegationSnapshot.Amount, rate)
		slashDiff := &Application{big.NewInt(0), big.NewInt(0)}
		snapshotAddr := delegationSnapshot.DelegatorAddress
		for i := range current.Delegations {
			delegationNow := current.Delegations[i]
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
				for i := range delegationNow.Undelegations {
					undelegate := delegationNow.Undelegations[i]
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

				// NOTE only need to pay beneficiary here,
				// they only get half of what was actually dispersed
				halfOfSlashDebt := new(big.Int).Div(slashDiff.TotalSlashed, common.Big2)
				slashDiff.TotalBeneficiaryReward.Add(slashDiff.TotalBeneficiaryReward, halfOfSlashDebt)
				utils.Logger().Info().
					RawJSON("delegation-snapshot", []byte(delegationSnapshot.String())).
					RawJSON("delegation-current", []byte(delegationNow.String())).
					Uint64("beneficiary-reward", halfOfSlashDebt.Uint64()).
					RawJSON("application", []byte(slashDiff.String())).
					Msg("completed an application of slashing")
				state.AddBalance(rewardBeneficiary, halfOfSlashDebt)
				slashTrack.TotalBeneficiaryReward.Add(
					slashTrack.TotalBeneficiaryReward, slashDiff.TotalBeneficiaryReward,
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
	slashes Records, rate numeric.Dec, rewardBeneficiary common.Address,
) (*Application, error) {
	slashDiff := &Application{big.NewInt(0), big.NewInt(0)}
	for _, slash := range slashes {
		snapshot, err := chain.ReadValidatorSnapshotAtEpoch(
			slash.Evidence.Epoch,
			slash.Evidence.Offender,
		)

		if err != nil {
			return nil, errors.Errorf(
				"could not find validator %s",
				common2.MustAddressToBech32(slash.Evidence.Offender),
			)
		}

		current, err := state.ValidatorWrapper(slash.Evidence.Offender, true, false)
		if err != nil {
			return nil, errors.Wrapf(
				errValidatorNotFoundDuringSlash, " %s ", err.Error(),
			)
		}
		// NOTE invariant: first delegation is the validators own
		// stake, rest are external delegations.
		// Bottom line: everyone will be slashed under the same rule.
		if err := delegatorSlashApply(
			snapshot.Validator, current, rate, state,
			rewardBeneficiary, slash.Evidence.Epoch, slashDiff,
		); err != nil {
			return nil, err
		}

		// finally, kick them off forever
		current.Status = effective.Banned
		utils.Logger().Info().
			RawJSON("delegation-current", []byte(current.String())).
			RawJSON("slash", []byte(slash.String())).
			Msg("about to update staking info for a validator after a slash")

		if err := current.SanityCheck(); err != nil {
			return nil, err
		}
	}
	return slashDiff, nil
}

// IsBanned ..
func IsBanned(wrapper *staking.ValidatorWrapper) bool {
	return wrapper.Status == effective.Banned
}

// Rate is the slashing % rate
func Rate(votingPower *votepower.Roster, records Records) numeric.Dec {
	rate := numeric.ZeroDec()

	for i := range records {
		doubleSignKeys := []bls.SerializedPublicKey{}
		for _, pubKey1 := range records[i].Evidence.FirstVote.SignerPubKeys {
			for _, pubKey2 := range records[i].Evidence.SecondVote.SignerPubKeys {
				if shard.CompareBLSPublicKey(pubKey1, pubKey2) == 0 {
					doubleSignKeys = append(doubleSignKeys, pubKey1)
					break
				}
			}
		}

		for _, key := range doubleSignKeys {
			if card, exists := votingPower.Voters[key]; exists &&
				bytes.Equal(card.EarningAccount[:], records[i].Evidence.Offender[:]) {
				rate = rate.Add(card.GroupPercent)
			} else {
				utils.Logger().Debug().
					RawJSON("roster", []byte(votingPower.String())).
					RawJSON("double-sign-record", []byte(records[i].String())).
					Msg("did not have offenders voter card in roster as expected")
			}
		}

	}

	if rate.LT(oneDoubleSignerRate) {
		rate = oneDoubleSignerRate
	}

	return rate
}
