package types

import (
	"encoding/json"
	"math/big"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/shard"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/pkg/errors"
)

// Define validator staking related const
const (
	MaxNameLength            = 140
	MaxIdentityLength        = 140
	MaxWebsiteLength         = 140
	MaxSecurityContactLength = 140
	MaxDetailsLength         = 280
	BLSVerificationStr       = "harmony-one"
	TenThousand              = 10000
	APRHistoryLength         = 30
	SigningHistoryLength     = 30
)

var (
	errAddressNotMatch = errors.New("validator key not match")
	// ErrInvalidSelfDelegation ..
	ErrInvalidSelfDelegation = errors.New(
		"self delegation can not be less than min_self_delegation",
	)
	errInvalidTotalDelegation = errors.New(
		"total delegation can not be bigger than max_total_delegation",
	)
	errMinSelfDelegationTooSmall = errors.New(
		"min_self_delegation must be greater than or equal to 10,000 ONE",
	)
	errInvalidMaxTotalDelegation = errors.New(
		"max_total_delegation can not be less than min_self_delegation",
	)
	errCommissionRateTooLarge = errors.New(
		"commission rate and change rate can not be larger than max commission rate",
	)
	errInvalidCommissionRate = errors.New(
		"commission rate, change rate and max rate should be a value ranging from 0.0 to 1.0",
	)
	errNeedAtLeastOneSlotKey = errors.New("need at least one slot key")
	errBLSKeysNotMatchSigs   = errors.New(
		"bls keys and corresponding signatures could not be verified",
	)
	errNilMinSelfDelegation    = errors.New("MinSelfDelegation can not be nil")
	errNilMaxTotalDelegation   = errors.New("MaxTotalDelegation can not be nil")
	errSlotKeyToRemoveNotFound = errors.New("slot key to remove not found")
	errSlotKeyToAddExists      = errors.New("slot key to add already exists")
	errDuplicateSlotKeys       = errors.New("slot keys can not have duplicates")
	// ErrExcessiveBLSKeys ..
	ErrExcessiveBLSKeys        = errors.New("more slot keys provided than allowed")
	errCannotChangeBannedTrait = errors.New("cannot change validator banned status")
)

// ValidatorSnapshotReader ..
type ValidatorSnapshotReader interface {
	ReadValidatorSnapshotAtEpoch(
		epoch *big.Int,
		addr common.Address,
	) (*ValidatorSnapshot, error)
}

type counters struct {
	// The number of blocks the validator
	// should've signed when in active mode (selected in committee)
	NumBlocksToSign *big.Int `json:"to-sign" rlp:"nil"`
	// The number of blocks the validator actually signed
	NumBlocksSigned *big.Int `json:"signed" rlp:"nil"`
}

// ValidatorWrapper contains validator,
// its delegation information
type ValidatorWrapper struct {
	Validator
	Delegations Delegations
	//
	Counters counters `json:"-"`
	// All the rewarded accumulated so far
	BlockReward *big.Int `json:"-"`
}

// ValidatorSnapshot contains validator snapshot and the corresponding epoch
type ValidatorSnapshot struct {
	Validator *ValidatorWrapper
	Epoch     *big.Int
}

// Computed represents current epoch
// availability measures, mostly for RPC
type Computed struct {
	Signed            *big.Int    `json:"current-epoch-signed"`
	ToSign            *big.Int    `json:"current-epoch-to-sign"`
	BlocksLeftInEpoch uint64      `json:"-"`
	Percentage        numeric.Dec `json:"current-epoch-signing-percentage"`
	IsBelowThreshold  bool        `json:"-"`
}

func (c Computed) String() string {
	s, _ := json.Marshal(c)
	return string(s)
}

// NewComputed ..
func NewComputed(
	signed, toSign *big.Int,
	blocksLeft uint64,
	percent numeric.Dec,
	isBelowNow bool) *Computed {
	return &Computed{signed, toSign, blocksLeft, percent, isBelowNow}
}

// NewEmptyStats ..
func NewEmptyStats() *ValidatorStats {
	return &ValidatorStats{
		[]APREntry{},
		numeric.ZeroDec(),
		[]VoteWithCurrentEpochEarning{},
		effective.Booted,
	}
}

// CurrentEpochPerformance represents validator performance in the context of
// whatever current epoch is
type CurrentEpochPerformance struct {
	CurrentSigningPercentage Computed `json:"current-epoch-signing-percent"`
	Epoch                    uint64   `json:"epoch"`
	Block                    uint64   `json:"block"`
}

// ValidatorRPCEnhanced contains extra information for RPC consumer
type ValidatorRPCEnhanced struct {
	Wrapper              ValidatorWrapper         `json:"validator"`
	Performance          *CurrentEpochPerformance `json:"current-epoch-performance"`
	ComputedMetrics      *ValidatorStats          `json:"metrics"`
	TotalDelegated       *big.Int                 `json:"total-delegation"`
	CurrentlyInCommittee bool                     `json:"currently-in-committee"`
	EPoSStatus           string                   `json:"epos-status"`
	EPoSWinningStake     *numeric.Dec             `json:"epos-winning-stake"`
	BootedStatus         *string                  `json:"booted-status"`
	ActiveStatus         string                   `json:"active-status"`
	Lifetime             *AccumulatedOverLifetime `json:"lifetime"`
}

// AccumulatedOverLifetime ..
type AccumulatedOverLifetime struct {
	BlockReward *big.Int            `json:"reward-accumulated"`
	Signing     counters            `json:"blocks"`
	APR         numeric.Dec         `json:"apr"`
	EpochAPRs   []APREntry          `json:"epoch-apr"`
	EpochBlocks []EpochSigningEntry `json:"epoch-blocks"`
}

func (w ValidatorWrapper) String() string {
	s, _ := json.Marshal(w)
	return string(s)
}

// MarshalJSON ..
func (w ValidatorWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Validator
		Address     string      `json:"address"`
		Delegations Delegations `json:"delegations"`
	}{
		w.Validator,
		common2.MustAddressToBech32(w.Address),
		w.Delegations,
	})
}

// VoteWithCurrentEpochEarning ..
type VoteWithCurrentEpochEarning struct {
	Vote   votepower.VoteOnSubcomittee `json:"key"`
	Earned *big.Int                    `json:"earned-reward"`
}

// APREntry ..
type APREntry struct {
	Epoch *big.Int    `json:"epoch"`
	Value numeric.Dec `json:"apr"`
}

// EpochSigningEntry ..
type EpochSigningEntry struct {
	Epoch  *big.Int `json:"epoch"`
	Blocks counters `json:"blocks"`
}

// ValidatorStats to record validator's performance and history records
type ValidatorStats struct {
	// APRs is the APR history containing APR's of epochs
	APRs []APREntry `json:"-"`
	// TotalEffectiveStake is the total effective stake this validator has
	TotalEffectiveStake numeric.Dec `json:"-"`
	// MetricsPerShard ..
	MetricsPerShard []VoteWithCurrentEpochEarning `json:"by-bls-key"`
	// BootedStatus
	BootedStatus effective.BootedStatus `json:"-"`
}

func (s ValidatorStats) String() string {
	str, _ := json.Marshal(s)
	return string(str)
}

// Validator - data fields for a validator
type Validator struct {
	// ECDSA address of the validator
	Address common.Address `json:"address"`
	// The BLS public key of the validator for consensus
	SlotPubKeys []bls.SerializedPublicKey `json:"bls-public-keys"`
	// The number of the last epoch this validator is
	// selected in committee (0 means never selected)
	LastEpochInCommittee *big.Int `json:"last-epoch-in-committee"`
	// validator's self declared minimum self delegation
	MinSelfDelegation *big.Int `json:"min-self-delegation"`
	// maximum total delegation allowed
	MaxTotalDelegation *big.Int `json:"max-total-delegation"`
	// Is the validator active in participating
	// committee selection process or not
	Status effective.Eligibility `json:"-"`
	// commission parameters
	Commission
	// description for the validator
	Description
	// CreationHeight is the height of creation
	CreationHeight *big.Int `json:"creation-height"`
}

// MaxBLSPerValidator ..
const MaxBLSPerValidator = 106

var (
	oneAsBigInt  = big.NewInt(denominations.One)
	minimumStake = new(big.Int).Mul(oneAsBigInt, big.NewInt(TenThousand))
)

// SanityCheck checks basic requirements of a validator
func (v *Validator) SanityCheck() error {
	if _, err := v.EnsureLength(); err != nil {
		return err
	}

	if len(v.SlotPubKeys) == 0 {
		return errNeedAtLeastOneSlotKey
	}

	if c := len(v.SlotPubKeys); c > MaxBLSPerValidator {
		return errors.Wrapf(
			ErrExcessiveBLSKeys, "have: %d allowed: %d",
			c, MaxBLSPerValidator,
		)
	}

	if v.MinSelfDelegation == nil {
		return errNilMinSelfDelegation
	}

	if v.MaxTotalDelegation == nil {
		return errNilMaxTotalDelegation
	}

	// MinSelfDelegation must be >= 10000 ONE
	if v.MinSelfDelegation.Cmp(minimumStake) < 0 {
		return errors.Wrapf(
			errMinSelfDelegationTooSmall,
			"delegation-given %s", v.MinSelfDelegation.String(),
		)
	}

	// MaxTotalDelegation must not be less than MinSelfDelegation
	if v.MaxTotalDelegation.Cmp(v.MinSelfDelegation) < 0 {
		return errors.Wrapf(
			errInvalidMaxTotalDelegation,
			"max-total-delegation %s min-self-delegation %s",
			v.MaxTotalDelegation.String(),
			v.MinSelfDelegation.String(),
		)
	}

	if v.Rate.LT(zeroPercent) || v.Rate.GT(hundredPercent) {
		return errors.Wrapf(
			errInvalidCommissionRate, "rate:%s", v.Rate.String(),
		)
	}

	if v.MaxRate.LT(zeroPercent) || v.MaxRate.GT(hundredPercent) {
		return errors.Wrapf(
			errInvalidCommissionRate, "max rate:%s", v.MaxRate.String(),
		)
	}

	if v.MaxChangeRate.LT(zeroPercent) || v.MaxChangeRate.GT(hundredPercent) {
		return errors.Wrapf(
			errInvalidCommissionRate, "max change rate:%s", v.MaxChangeRate.String(),
		)
	}

	if v.Rate.GT(v.MaxRate) {
		return errors.Wrapf(
			errCommissionRateTooLarge,
			"rate:%s max rate:%s", v.Rate.String(), v.MaxRate.String(),
		)
	}

	if v.MaxChangeRate.GT(v.MaxRate) {
		return errors.Wrapf(
			errCommissionRateTooLarge,
			"rate:%s max change rate:%s", v.Rate.String(), v.MaxChangeRate.String(),
		)
	}

	allKeys := map[bls.SerializedPublicKey]struct{}{}
	for i := range v.SlotPubKeys {
		if _, ok := allKeys[v.SlotPubKeys[i]]; !ok {
			allKeys[v.SlotPubKeys[i]] = struct{}{}
		} else {
			return errDuplicateSlotKeys
		}
	}
	return nil
}

// TotalDelegation - return the total amount of token in delegation
func (w *ValidatorWrapper) TotalDelegation() *big.Int {
	total := big.NewInt(0)
	for _, entry := range w.Delegations {
		total.Add(total, entry.Amount)
	}
	return total
}

var (
	hundredPercent = numeric.NewDec(1)
	zeroPercent    = numeric.NewDec(0)
)

// SanityCheck checks the basic requirements
func (w *ValidatorWrapper) SanityCheck() error {
	if err := w.Validator.SanityCheck(); err != nil {
		return err
	}
	// Self delegation must be >= MinSelfDelegation
	switch len(w.Delegations) {
	case 0:
		return errors.Wrapf(
			ErrInvalidSelfDelegation, "no self delegation given at all",
		)
	default:
		if w.Status != effective.Banned &&
			w.Delegations[0].Amount.Cmp(w.Validator.MinSelfDelegation) < 0 {
			if w.Status == effective.Active {
				return errors.Wrapf(
					ErrInvalidSelfDelegation,
					"min_self_delegation %s, amount %s",
					w.Validator.MinSelfDelegation, w.Delegations[0].Amount.String(),
				)
			}
		}
	}
	totalDelegation := w.TotalDelegation()
	// Total delegation must be <= MaxTotalDelegation
	if totalDelegation.Cmp(w.Validator.MaxTotalDelegation) > 0 {
		return errors.Wrapf(
			errInvalidTotalDelegation,
			"total %s max-total %s",
			totalDelegation.String(),
			w.Validator.MaxTotalDelegation.String(),
		)
	}
	return nil
}

// RawStakePerSlot return raw stake of each slot key. If HIP16 was activated at that apoch, it only calculate raw stake for keys not exceed the slotsLimit.
func (snapshot ValidatorSnapshot) RawStakePerSlot() numeric.Dec {
	wrapper := snapshot.Validator
	instance := shard.Schedule.InstanceForEpoch(snapshot.Epoch)
	slotsLimit := instance.SlotsLimit()
	// HIP16 is acatived
	if slotsLimit > 0 {
		limitedSlotsCount := 0 // limited slots count for HIP16
		shardCount := big.NewInt(int64(instance.NumShards()))
		shardSlotsCount := make([]int, shardCount.Uint64()) // number slots keys on each shard
		for _, pubkey := range wrapper.SlotPubKeys {
			shardIndex := new(big.Int).Mod(pubkey.Big(), shardCount).Uint64()
			shardSlotsCount[shardIndex] += 1
			if shardSlotsCount[shardIndex] > slotsLimit {
				continue
			}
			limitedSlotsCount += 1
		}
		return numeric.NewDecFromBigInt(wrapper.TotalDelegation()).
			QuoInt64(int64(limitedSlotsCount))
	}
	if len(wrapper.SlotPubKeys) > 0 {
		return numeric.NewDecFromBigInt(wrapper.TotalDelegation()).
			QuoInt64(int64(len(wrapper.SlotPubKeys)))
	}
	return numeric.ZeroDec()
}

// Description - some possible IRL connections
type Description struct {
	Name            string `json:"name"`             // name
	Identity        string `json:"identity"`         // optional identity signature (ex. UPort or Keybase)
	Website         string `json:"website"`          // optional website link
	SecurityContact string `json:"security-contact"` // optional security contact info
	Details         string `json:"details"`          // optional details
}

// MarshalValidator marshals the validator object
func MarshalValidator(validator Validator) ([]byte, error) {
	return rlp.EncodeToBytes(validator)
}

// UnmarshalValidator unmarshal binary into Validator object
func UnmarshalValidator(by []byte) (Validator, error) {
	decoded := Validator{}
	err := rlp.DecodeBytes(by, &decoded)
	return decoded, err
}

// UpdateDescription returns a new Description object with d1 as the base and the fields that's not empty in d2 updated
// accordingly. An error is returned if the resulting description fields have invalid length.
func UpdateDescription(d1, d2 Description) (Description, error) {
	newDesc := d1
	if d2.Name != "" {
		newDesc.Name = d2.Name
	}
	if d2.Identity != "" {
		newDesc.Identity = d2.Identity
	}
	if d2.Website != "" {
		newDesc.Website = d2.Website
	}
	if d2.SecurityContact != "" {
		newDesc.SecurityContact = d2.SecurityContact
	}
	if d2.Details != "" {
		newDesc.Details = d2.Details
	}

	return newDesc.EnsureLength()
}

// EnsureLength ensures the length of a validator's description.
func (d Description) EnsureLength() (Description, error) {
	if len(d.Name) > MaxNameLength {
		return d, errors.Errorf(
			"exceed maximum name length %d %d", len(d.Name), MaxNameLength,
		)
	}
	if len(d.Identity) > MaxIdentityLength {
		return d, errors.Errorf(
			"exceed Maximum Length identity %d %d", len(d.Identity), MaxIdentityLength,
		)
	}
	if len(d.Website) > MaxWebsiteLength {
		return d, errors.Errorf(
			"exceed Maximum Length website %d %d", len(d.Website), MaxWebsiteLength,
		)
	}
	if len(d.SecurityContact) > MaxSecurityContactLength {
		return d, errors.Errorf(
			"exceed Maximum Length %d %d", len(d.SecurityContact), MaxSecurityContactLength,
		)
	}
	if len(d.Details) > MaxDetailsLength {
		return d, errors.Errorf(
			"exceed Maximum Length for details %d %d", len(d.Details), MaxDetailsLength,
		)
	}

	return d, nil
}

// VerifyBLSKeys checks if the public BLS key at index i of pubKeys matches the
// BLS key signature at index i of pubKeysSigs.
func VerifyBLSKeys(pubKeys []bls.SerializedPublicKey, pubKeySigs []bls.SerializedSignature) error {
	if len(pubKeys) != len(pubKeySigs) {
		return errBLSKeysNotMatchSigs
	}

	for i := 0; i < len(pubKeys); i++ {
		if err := VerifyBLSKey(&pubKeys[i], &pubKeySigs[i]); err != nil {
			return err
		}
	}

	return nil
}

// VerifyBLSKey checks if the public BLS key matches the BLS signature
func VerifyBLSKey(pubKey *bls.SerializedPublicKey, pubKeySig *bls.SerializedSignature) error {
	if len(pubKeySig) == 0 {
		return errBLSKeysNotMatchSigs
	}

	blsPubKey, err := bls.BytesToBLSPublicKey(pubKey[:])
	if err != nil {
		return errBLSKeysNotMatchSigs
	}

	msgSig := bls_core.Sign{}
	if err := msgSig.Deserialize(pubKeySig[:]); err != nil {
		return err
	}

	messageBytes := []byte(BLSVerificationStr)
	msgHash := hash.Keccak256(messageBytes)
	if !msgSig.VerifyHash(blsPubKey, msgHash[:]) {
		return errBLSKeysNotMatchSigs
	}

	return nil
}

func containsHarmonyBLSKeys(
	blsKeys []bls.SerializedPublicKey,
	hmyAccounts []genesis.DeployAccount,
	epoch *big.Int,
) error {
	for i := range blsKeys {
		if err := matchesHarmonyBLSKey(
			&blsKeys[i], hmyAccounts, epoch,
		); err != nil {
			return err
		}
	}
	return nil
}

func matchesHarmonyBLSKey(
	blsKey *bls.SerializedPublicKey,
	hmyAccounts []genesis.DeployAccount,
	epoch *big.Int,
) error {
	type publicKeyAsHex = string
	cache := map[string]map[publicKeyAsHex]struct{}{}
	return func() error {
		key := epoch.String()
		if _, ok := cache[key]; !ok {
			// one time cost per epoch
			cache[key] = map[publicKeyAsHex]struct{}{}
			for i := range hmyAccounts {
				// invariant assume it is hex
				cache[key][hmyAccounts[i].BLSPublicKey] = struct{}{}
			}
		}

		hex := blsKey.Hex()
		if _, exists := cache[key][hex]; exists {
			return errors.Wrapf(
				errDuplicateSlotKeys,
				"slot key %s conflicts with internal keys",
				hex,
			)
		}
		return nil
	}()
}

// CreateValidatorFromNewMsg creates validator from NewValidator message
func CreateValidatorFromNewMsg(
	val *CreateValidator, blockNum, epoch *big.Int,
) (*Validator, error) {
	desc, err := val.Description.EnsureLength()
	if err != nil {
		return nil, err
	}
	commission := Commission{val.CommissionRates, blockNum}
	pubKeys := append(val.SlotPubKeys[0:0], val.SlotPubKeys...)

	instance := shard.Schedule.InstanceForEpoch(epoch)
	if err := containsHarmonyBLSKeys(
		pubKeys, instance.HmyAccounts(), epoch,
	); err != nil {
		return nil, err
	}

	if err = VerifyBLSKeys(pubKeys, val.SlotKeySigs); err != nil {
		return nil, err
	}

	v := Validator{
		Address:              val.ValidatorAddress,
		SlotPubKeys:          pubKeys,
		LastEpochInCommittee: new(big.Int),
		MinSelfDelegation:    val.MinSelfDelegation,
		MaxTotalDelegation:   val.MaxTotalDelegation,
		Status:               effective.Active,
		Commission:           commission,
		Description:          desc,
		CreationHeight:       blockNum,
	}
	return &v, nil
}

// UpdateValidatorFromEditMsg updates validator from EditValidator message
func UpdateValidatorFromEditMsg(validator *Validator, edit *EditValidator, epoch *big.Int) error {
	if validator.Address != edit.ValidatorAddress {
		return errAddressNotMatch
	}
	desc, err := UpdateDescription(validator.Description, edit.Description)
	if err != nil {
		return err
	}

	validator.Description = desc

	if edit.CommissionRate != nil {
		validator.Rate = *edit.CommissionRate
	}

	if edit.MinSelfDelegation != nil && edit.MinSelfDelegation.Sign() != 0 {
		validator.MinSelfDelegation = edit.MinSelfDelegation
	}

	if edit.MaxTotalDelegation != nil && edit.MaxTotalDelegation.Sign() != 0 {
		validator.MaxTotalDelegation = edit.MaxTotalDelegation
	}

	if edit.SlotKeyToRemove != nil {
		index := -1
		for i, key := range validator.SlotPubKeys {
			if key == *edit.SlotKeyToRemove {
				index = i
				break
			}
		}
		// we found key to be removed
		if index >= 0 {
			validator.SlotPubKeys = append(
				validator.SlotPubKeys[:index], validator.SlotPubKeys[index+1:]...,
			)
		} else {
			return errSlotKeyToRemoveNotFound
		}
	}

	if edit.SlotKeyToAdd != nil {
		found := false
		for _, key := range validator.SlotPubKeys {
			if key == *edit.SlotKeyToAdd {
				found = true
				break
			}
		}
		if !found {
			instance := shard.Schedule.InstanceForEpoch(epoch)
			if err := matchesHarmonyBLSKey(
				edit.SlotKeyToAdd, instance.HmyAccounts(), epoch,
			); err != nil {
				return err
			}
			if err := VerifyBLSKey(edit.SlotKeyToAdd, edit.SlotKeyToAddSig); err != nil {
				return err
			}
			validator.SlotPubKeys = append(validator.SlotPubKeys, *edit.SlotKeyToAdd)
		} else {
			return errSlotKeyToAddExists
		}
	}

	switch validator.Status {
	case effective.Banned:
		return errCannotChangeBannedTrait
	default:
		switch edit.EPOSStatus {
		case effective.Active, effective.Inactive:
			validator.Status = edit.EPOSStatus
		default:
		}
	}

	return nil
}

// String returns a human readable string representation of a validator.
func (v Validator) String() string {
	s, _ := json.Marshal(v)
	return string(s)
}
