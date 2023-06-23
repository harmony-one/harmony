package slash

import (
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/shard"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

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
		commitPayload := consensus_sig.ConstructCommitPayload(chain.Config(),
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

// applySlashRate applies a decimal percentage to a bigInt value
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

func payDownByDelegationStaked(
	delegation *staking.Delegation,
	slashDebt, totalSlashed *big.Int,
) {
	payDown(delegation.Amount, slashDebt, totalSlashed)
}

func payDownByUndelegation(
	undelegation *staking.Undelegation,
	slashDebt, totalSlashed *big.Int,
) {
	payDown(undelegation.Amount, slashDebt, totalSlashed)
}

func payDownByReward(
	delegation *staking.Delegation,
	slashDebt, totalSlashed *big.Int,
) {
	payDown(delegation.Reward, slashDebt, totalSlashed)
}

func payDown(
	balance, debt, totalSlashed *big.Int,
) {
	slashAmount := new(big.Int).Set(debt)
	if balance.Cmp(debt) < 0 {
		slashAmount.Set(balance)
	}
	balance.Sub(balance, slashAmount)
	debt.Sub(debt, slashAmount)
	totalSlashed.Add(totalSlashed, slashAmount)
}

func makeSlashList(snapshot, current *staking.ValidatorWrapper) ([][2]int, *big.Int) {
	slashIndexPairs := make([][2]int, 0, len(snapshot.Delegations))
	slashDelegations := make(map[common.Address]int, len(snapshot.Delegations))
	totalStake := big.NewInt(0)
	for index, delegation := range snapshot.Delegations {
		slashDelegations[delegation.DelegatorAddress] = index
		totalStake.Add(totalStake, delegation.Amount)
	}
	for index, delegation := range current.Delegations {
		if oldIndex, exist := slashDelegations[delegation.DelegatorAddress]; exist {
			slashIndexPairs = append(slashIndexPairs, [2]int{oldIndex, index})
		}
	}
	return slashIndexPairs, totalStake
}

// delegatorSlashApply applies slashing to all delegators including the validator.
// The validator’s self-owned stake is slashed by 50%.
// The stake of external delegators is slashed by 80% of the leader’s self-owned slashed stake, each one proportionally to their stake.
func delegatorSlashApply(
	snapshot, current *staking.ValidatorWrapper,
	state *state.DB,
	rewardBeneficiary common.Address,
	doubleSignEpoch *big.Int,
	slashTrack *Application,
) error {
	// First delegation is validator's own stake
	validatorDebt := new(big.Int).Div(snapshot.Delegations[0].Amount, common.Big2)
	return delegatorSlashApplyDebt(snapshot, current, state, validatorDebt, rewardBeneficiary, doubleSignEpoch, slashTrack)
}

// delegatorSlashApply applies slashing to all delegators including the validator.
// The validator’s self-owned stake is slashed by 50%.
// The stake of external delegators is slashed by 80% of the leader’s self-owned slashed stake, each one proportionally to their stake.
func delegatorSlashApplyDebt(
	snapshot, current *staking.ValidatorWrapper,
	state *state.DB,
	validatorDebt *big.Int,
	rewardBeneficiary common.Address,
	doubleSignEpoch *big.Int,
	slashTrack *Application,
) error {
	slashIndexPairs, totalStake := makeSlashList(snapshot, current)
	validatorDelegation := &current.Delegations[0]
	totalExternalStake := new(big.Int).Sub(totalStake, validatorDelegation.Amount)
	validatorSlashed := applySlashingToDelegation(validatorDelegation, state, rewardBeneficiary, doubleSignEpoch, validatorDebt)
	totalSlahsed := new(big.Int).Set(validatorSlashed)
	// External delegators

	aggregateDebt := applySlashRate(validatorSlashed, numeric.MustNewDecFromStr("0.8"))

	for _, indexPair := range slashIndexPairs[1:] {
		snapshotIndex := indexPair[0]
		currentIndex := indexPair[1]
		delegationSnapshot := snapshot.Delegations[snapshotIndex]
		delegationCurrent := &current.Delegations[currentIndex]
		// A*(B/C) => (A*B)/C
		// slashDebt = aggregateDebt*(Amount/totalExternalStake)
		slashDebt := new(big.Int).Mul(delegationSnapshot.Amount, aggregateDebt)
		slashDebt.Div(slashDebt, totalExternalStake)

		slahsed := applySlashingToDelegation(delegationCurrent, state, rewardBeneficiary, doubleSignEpoch, slashDebt)
		totalSlahsed.Add(totalSlahsed, slahsed)
	}

	// finally, kick them off forever
	current.Status = effective.Banned
	if err := current.SanityCheck(); err != nil {
		return err
	}
	state.UpdateValidatorWrapper(current.Address, current)
	beneficiaryReward := new(big.Int).Div(totalSlahsed, common.Big2)
	state.AddBalance(rewardBeneficiary, beneficiaryReward)
	slashTrack.TotalBeneficiaryReward.Add(slashTrack.TotalBeneficiaryReward, beneficiaryReward)
	slashTrack.TotalSlashed.Add(slashTrack.TotalSlashed, totalSlahsed)
	return nil
}

// applySlashingToDelegation applies slashing to a delegator, given the amount that should be slashed.
// Also, rewards the beneficiary half of the amount that was successfully slashed.
func applySlashingToDelegation(delegation *staking.Delegation, state *state.DB, rewardBeneficiary common.Address, doubleSignEpoch *big.Int, slashDebt *big.Int) *big.Int {
	slashed := big.NewInt(0)
	debtCopy := new(big.Int).Set(slashDebt)

	payDownByDelegationStaked(delegation, debtCopy, slashed)

	// NOTE Assume did as much as could above, now check the undelegations
	for i := range delegation.Undelegations {
		if debtCopy.Sign() == 0 {
			break
		}
		undelegation := &delegation.Undelegations[i]
		// the epoch matters, only those undelegation
		// such that epoch>= doubleSignEpoch should be slashable
		if undelegation.Epoch.Cmp(doubleSignEpoch) >= 0 {
			payDownByUndelegation(undelegation, debtCopy, slashed)
		}
	}
	if debtCopy.Sign() == 1 {
		payDownByReward(delegation, debtCopy, slashed)
	}
	return slashed
}

// Apply ..
func Apply(
	chain staking.ValidatorSnapshotReader, state *state.DB,
	slashes Records, rewardBeneficiary common.Address,
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
		// NOTE invariant: first delegation is the validators own stake,
		// rest are external delegations.
		if err := delegatorSlashApply(
			snapshot.Validator, current, state,
			rewardBeneficiary, slash.Evidence.Epoch, slashDiff,
		); err != nil {
			return nil, err
		}

		utils.Logger().Info().
			RawJSON("delegation-current", []byte(current.String())).
			RawJSON("slash", []byte(slash.String())).
			Msg("slash applyed")

	}
	return slashDiff, nil
}

// IsBanned ..
func IsBanned(wrapper *staking.ValidatorWrapper) bool {
	return wrapper.Status == effective.Banned
}
