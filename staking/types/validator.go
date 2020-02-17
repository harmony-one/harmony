package types

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

// Define validator staking related const
const (
	MaxNameLength            = 140
	MaxIdentityLength        = 140
	MaxWebsiteLength         = 140
	MaxSecurityContactLength = 140
	MaxDetailsLength         = 280
	BlsVerificationStr       = "harmony-one"
)

var (
	errAddressNotMatch           = errors.New("Validator key not match")
	errInvalidSelfDelegation     = errors.New("self delegation can not be less than min_self_delegation")
	errInvalidTotalDelegation    = errors.New("total delegation can not be bigger than max_total_delegation")
	errMinSelfDelegationTooSmall = errors.New("min_self_delegation has to be greater than 1 ONE")
	errInvalidMaxTotalDelegation = errors.New("max_total_delegation can not be less than min_self_delegation")
	errCommissionRateTooLarge    = errors.New("commission rate and change rate can not be larger than max commission rate")
	errInvalidCommissionRate     = errors.New("commission rate, change rate and max rate should be within 0-100 percent")
	errNeedAtLeastOneSlotKey     = errors.New("need at least one slot key")
	errBLSKeysNotMatchSigs       = errors.New("bls keys and corresponding signatures could not be verified")
	errNilMinSelfDelegation      = errors.New("MinSelfDelegation can not be nil")
	errNilMaxTotalDelegation     = errors.New("MaxTotalDelegation can not be nil")
	errSlotKeyToRemoveNotFound   = errors.New("slot key to remove not found")
	errSlotKeyToAddExists        = errors.New("slot key to add already exists")
	errDuplicateSlotKeys         = errors.New("slot keys can not have duplicates")
)

// ValidatorWrapper contains validator and its delegation information
type ValidatorWrapper struct {
	Validator   `json:"validator"`
	Delegations []Delegation `json:"delegations"`

	Snapshot struct {
		Epoch *big.Int
		// The number of blocks the validator should've signed when in active mode (selected in committee)
		NumBlocksToSign *big.Int `rlp:"nil"`
		// The number of blocks the validator actually signed
		NumBlocksSigned *big.Int `rlp:"nil"`
	}
}

// VotePerShard ..
type VotePerShard struct {
	ShardID     uint32      `json:"shard-id"`
	VotingPower numeric.Dec `json:"voting-power"`
}

// KeysPerShard ..
type KeysPerShard struct {
	ShardID uint32               `json:"shard-id"`
	Keys    []shard.BlsPublicKey `json:"bls-public-keys"`
}

// ValidatorStats to record validator's performance and history records
type ValidatorStats struct {
	// The number of times they validator is jailed due to extensive downtime
	NumJailed *big.Int `rlp:"nil"`
	// TotalEffectiveStake is the total effective stake this validator has
	TotalEffectiveStake numeric.Dec `rlp:"nil"`
	// VotingPowerPerShard ..
	VotingPowerPerShard []VotePerShard
	// BLSKeyPerShard ..
	BLSKeyPerShard []KeysPerShard
}

// Validator - data fields for a validator
type Validator struct {
	// ECDSA address of the validator
	Address common.Address
	// The BLS public key of the validator for consensus
	SlotPubKeys []shard.BlsPublicKey
	// The number of the last epoch this validator is selected in committee (0 means never selected)
	LastEpochInCommittee *big.Int
	// validator's self declared minimum self delegation
	MinSelfDelegation *big.Int
	// maximum total delegation allowed
	MaxTotalDelegation *big.Int
	// Is the validator active in participating committee selection process or not
	Active bool
	// commission parameters
	Commission
	// description for the validator
	Description
	// CreationHeight is the height of creation
	CreationHeight *big.Int
	// Banned records whether this validator is banned from the network because they double-signed
	Banned bool
}

// SanityCheck checks basic requirements of a validator
func (v *Validator) SanityCheck() error {
	if _, err := v.EnsureLength(); err != nil {
		return err
	}

	if len(v.SlotPubKeys) == 0 {
		return errNeedAtLeastOneSlotKey
	}

	if v.MinSelfDelegation == nil {
		return errNilMinSelfDelegation
	}

	if v.MaxTotalDelegation == nil {
		return errNilMaxTotalDelegation
	}

	// MinSelfDelegation must be >= 1 ONE
	if v.MinSelfDelegation.Cmp(big.NewInt(denominations.One)) < 0 {
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
			errInvalidCommissionRate, "rate:%s", v.MaxRate.String(),
		)
	}

	if v.MaxChangeRate.LT(zeroPercent) || v.MaxChangeRate.GT(hundredPercent) {
		return errors.Wrapf(
			errInvalidCommissionRate, "rate:%s", v.MaxChangeRate.String(),
		)
	}

	if v.Rate.GT(v.MaxRate) {
		return errors.Wrapf(
			errCommissionRateTooLarge, "rate:%s", v.MaxRate.String(),
		)
	}

	if v.MaxChangeRate.GT(v.MaxRate) {
		return errors.Wrapf(
			errCommissionRateTooLarge, "rate:%s", v.MaxChangeRate.String(),
		)
	}

	allKeys := map[shard.BlsPublicKey]struct{}{}
	for i := range v.SlotPubKeys {
		if _, ok := allKeys[v.SlotPubKeys[i]]; !ok {
			allKeys[v.SlotPubKeys[i]] = struct{}{}
		} else {
			return errDuplicateSlotKeys
		}
	}
	return nil
}

// MarshalJSON ..
func (v *ValidatorStats) MarshalJSON() ([]byte, error) {
	type t struct {
		NumJailed           uint64         `json:"blocks-jailed"`
		TotalEffectiveStake numeric.Dec    `json:"total-effective-stake"`
		VotingPowerPerShard []VotePerShard `json:"voting-power-per-shard"`
		BLSKeyPerShard      []KeysPerShard `json:"bls-keys-per-shard"`
	}
	return json.Marshal(t{
		NumJailed:           v.NumJailed.Uint64(),
		TotalEffectiveStake: v.TotalEffectiveStake,
		VotingPowerPerShard: v.VotingPowerPerShard,
		BLSKeyPerShard:      v.BLSKeyPerShard,
	})

}

// MarshalJSON ..
func (v *Validator) MarshalJSON() ([]byte, error) {
	type t struct {
		Address            string      `json:"one-address"`
		SlotPubKeys        []string    `json:"bls-public-keys"`
		MinSelfDelegation  string      `json:"min-self-delegation"`
		MaxTotalDelegation string      `json:"max-total-delegation"`
		Active             bool        `json:"active"`
		Commission         Commission  `json:"commission"`
		Description        Description `json:"description"`
		CreationHeight     uint64      `json:"creation-height"`
	}
	slots := make([]string, len(v.SlotPubKeys))
	for i := range v.SlotPubKeys {
		slots[i] = v.SlotPubKeys[i].Hex()
	}
	return json.Marshal(t{
		Address:            common2.MustAddressToBech32(v.Address),
		SlotPubKeys:        slots,
		MinSelfDelegation:  v.MinSelfDelegation.String(),
		MaxTotalDelegation: v.MaxTotalDelegation.String(),
		Active:             v.Active,
		Commission:         v.Commission,
		Description:        v.Description,
		CreationHeight:     v.CreationHeight.Uint64(),
	})
}

func printSlotPubKeys(pubKeys []shard.BlsPublicKey) string {
	str := "["
	for i := range pubKeys {
		str += fmt.Sprintf("%d: %s,", i, pubKeys[i].Hex())
	}
	str += "]"
	return str
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
			errInvalidSelfDelegation, "no self delegation given at all",
		)
	default:
		if w.Delegations[0].Amount.Cmp(w.Validator.MinSelfDelegation) < 0 {
			return errors.Wrapf(
				errInvalidSelfDelegation,
				"have %s want %s", w.Delegations[0].Amount.String(), w.Validator.MinSelfDelegation,
			)
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

// Description - some possible IRL connections
type Description struct {
	Name            string `json:"name"`             // name
	Identity        string `json:"identity"`         // optional identity signature (ex. UPort or Keybase)
	Website         string `json:"website"`          // optional website link
	SecurityContact string `json:"security_contact"` // optional security contact info
	Details         string `json:"details"`          // optional details
}

// MarshalValidator marshals the validator object
func MarshalValidator(validator Validator) ([]byte, error) {
	return rlp.EncodeToBytes(validator)
}

// UnmarshalValidator unmarshal binary into Validator object
func UnmarshalValidator(by []byte) (*Validator, error) {
	decoded := &Validator{}
	err := rlp.DecodeBytes(by, decoded)
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
		return d, ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(d.Name), "maxNameLen", MaxNameLength)
	}
	if len(d.Identity) > MaxIdentityLength {
		return d, ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(d.Identity), "maxIdentityLen", MaxIdentityLength)
	}
	if len(d.Website) > MaxWebsiteLength {
		return d, ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(d.Website), "maxWebsiteLen", MaxWebsiteLength)
	}
	if len(d.SecurityContact) > MaxSecurityContactLength {
		return d, ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(d.SecurityContact), "maxSecurityContactLen", MaxSecurityContactLength)
	}
	if len(d.Details) > MaxDetailsLength {
		return d, ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(d.Details), "maxDetailsLen", MaxDetailsLength)
	}

	return d, nil
}

// GetAddress returns address
func (v *Validator) GetAddress() common.Address { return v.Address }

// GetName returns the name of validator in the description
func (v *Validator) GetName() string { return v.Description.Name }

// GetCommissionRate returns the commission rate of the validator
func (v *Validator) GetCommissionRate() numeric.Dec { return v.Commission.Rate }

// GetMinSelfDelegation returns the minimum amount the validator must stake
func (v *Validator) GetMinSelfDelegation() *big.Int { return v.MinSelfDelegation }

// VerifyBLSKeys checks if the public BLS key at index i of pubKeys matches the
// BLS key signature at index i of pubKeysSigs.
func VerifyBLSKeys(pubKeys []shard.BlsPublicKey, pubKeySigs []shard.BLSSignature) error {
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
func VerifyBLSKey(pubKey *shard.BlsPublicKey, pubKeySig *shard.BLSSignature) error {
	if len(pubKeySig) == 0 {
		return errBLSKeysNotMatchSigs
	}

	blsPubKey := new(bls.PublicKey)
	if err := pubKey.ToLibBLSPublicKey(blsPubKey); err != nil {
		return errBLSKeysNotMatchSigs
	}

	msgSig := bls.Sign{}
	if err := msgSig.Deserialize(pubKeySig[:]); err != nil {
		return err
	}

	messageBytes := []byte(BlsVerificationStr)
	msgHash := hash.Keccak256(messageBytes)
	if !msgSig.VerifyHash(blsPubKey, msgHash[:]) {
		return errBLSKeysNotMatchSigs
	}

	return nil
}

// CreateValidatorFromNewMsg creates validator from NewValidator message
func CreateValidatorFromNewMsg(val *CreateValidator, blockNum *big.Int) (*Validator, error) {
	desc, err := val.Description.EnsureLength()
	if err != nil {
		return nil, err
	}
	commission := Commission{val.CommissionRates, blockNum}
	pubKeys := append(val.SlotPubKeys[0:0], val.SlotPubKeys...)

	if err = VerifyBLSKeys(pubKeys, val.SlotKeySigs); err != nil {
		return nil, err
	}

	v := Validator{
		val.ValidatorAddress, pubKeys,
		new(big.Int), val.MinSelfDelegation, val.MaxTotalDelegation, true,
		commission, desc, blockNum, false,
	}
	return &v, nil
}

// UpdateValidatorFromEditMsg updates validator from EditValidator message
func UpdateValidatorFromEditMsg(validator *Validator, edit *EditValidator) error {
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
			validator.SlotPubKeys = append(validator.SlotPubKeys[:index], validator.SlotPubKeys[index+1:]...)
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
			if err := VerifyBLSKey(edit.SlotKeyToAdd, edit.SlotKeyToAddSig); err != nil {
				return err
			}
			validator.SlotPubKeys = append(validator.SlotPubKeys, *edit.SlotKeyToAdd)
		} else {
			return errSlotKeyToAddExists
		}
	}

	if edit.Active != nil {
		validator.Active = *edit.Active
	}

	return nil
}

// String returns a human readable string representation of a validator.
func (v *Validator) String() string {
	return fmt.Sprintf(`Validator
  Address:                    %s
  SlotPubKeys:                %s
  LastEpochInCommittee:           %v
  Minimum Self Delegation:     %v
  Maximum Total Delegation:     %v
  Description:                %v
  Commission:                 %v`,
		common2.MustAddressToBech32(v.Address), printSlotPubKeys(v.SlotPubKeys),
		v.LastEpochInCommittee,
		v.MinSelfDelegation, v.MaxTotalDelegation, v.Description, v.Commission,
	)
}
