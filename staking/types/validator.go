package types

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/common/denominations"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

// Define validator staking related const
const (
	MaxNameLength            = 70
	MaxIdentityLength        = 3000
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
	errInvalidComissionRate      = errors.New("commission rate, change rate and max rate should be within 0-100 percent")
	errNeedAtLeastOneSlotKey     = errors.New("need at least one slot key")
	errBLSKeysNotMatchSigs       = errors.New("bls keys and corresponding signatures could not be verified")
)

// ValidatorWrapper contains validator and its delegation information
type ValidatorWrapper struct {
	Validator   `json:"validator" yaml:"validator" rlp:"nil"`
	Delegations []Delegation `json:"delegations" yaml:"delegations" rlp:"nil"`
}

// VotePerShard ..
type VotePerShard struct {
	ShardID     uint32
	VotingPower numeric.Dec
}

// KeysPerShard ..
type KeysPerShard struct {
	ShardID uint32
	Keys    []shard.BlsPublicKey
}

// ValidatorStats to record validator's performance and history records
type ValidatorStats struct {
	// The number of blocks the validator should've signed when in active mode (selected in committee)
	NumBlocksToSign *big.Int `json:"num_blocks_to_sign" rlp:"nil"`
	// The number of blocks the validator actually signed
	NumBlocksSigned *big.Int `json:"num_blocks_signed" rlp:"nil"`
	// The number of times they validator is jailed due to extensive downtime
	NumJailed *big.Int `json:"num_jailed" rlp:"nil"`
	// TotalEffectiveStake is the total effective stake this validator has
	TotalEffectiveStake numeric.Dec `json:"total_effective_stake" rlp:"nil"`
	// VotingPowerPerShard ..
	VotingPowerPerShard []VotePerShard
	// BLSKeyPerShard ..
	BLSKeyPerShard []KeysPerShard
}

// Validator - data fields for a validator
type Validator struct {
	// ECDSA address of the validator
	Address common.Address `json:"address" yaml:"address"`
	// The BLS public key of the validator for consensus
	SlotPubKeys []shard.BlsPublicKey `json:"slot_pub_keys" yaml:"slot_pub_keys"`
	// if unbonding, height at which this validator has begun unbonding
	UnbondingHeight *big.Int `json:"unbonding_height" yaml:"unbonding_height"`
	// validator's self declared minimum self delegation
	MinSelfDelegation *big.Int `json:"min_self_delegation" yaml:"min_self_delegation"`
	// maximum total delegation allowed
	MaxTotalDelegation *big.Int `json:"max_total_delegation" yaml:"max_total_delegation"`
	// Is the validator active in the validating process or not
	Active bool `json:"active" yaml:"active"`
	// commission parameters
	Commission `json:"commission" yaml:"commission"`
	// description for the validator
	Description `json:"description" yaml:"description"`
	// CreationHeight is the height of creation
	CreationHeight *big.Int `json:"creation_height" yaml:"creation_height"`
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

// SanityCheck checks the basic requirements
func (w *ValidatorWrapper) SanityCheck() error {
	if len(w.SlotPubKeys) == 0 {
		return errNeedAtLeastOneSlotKey
	}

	// MinSelfDelegation must be >= 1 ONE
	if w.Validator.MinSelfDelegation == nil || w.Validator.MinSelfDelegation.Cmp(big.NewInt(denominations.One)) < 0 {
		return errMinSelfDelegationTooSmall
	}

	// Self delegation must be >= MinSelfDelegation
	if len(w.Delegations) == 0 || w.Delegations[0].Amount.Cmp(w.Validator.MinSelfDelegation) < 0 {
		return errInvalidSelfDelegation
	}

	// Only enforce rules on MaxTotalDelegation is it's > 0; 0 means no limit for max total delegation
	if w.Validator.MaxTotalDelegation != nil && w.Validator.MaxTotalDelegation.Cmp(big.NewInt(0)) > 0 {
		// MaxTotalDelegation must not be less than MinSelfDelegation
		if w.Validator.MaxTotalDelegation.Cmp(w.Validator.MinSelfDelegation) < 0 {
			return errInvalidMaxTotalDelegation
		}

		totalDelegation := w.TotalDelegation()
		// Total delegation must be <= MaxTotalDelegation
		if totalDelegation.Cmp(w.Validator.MaxTotalDelegation) > 0 {
			return errInvalidTotalDelegation
		}
	}

	hundredPercent := numeric.NewDec(1)
	zeroPercent := numeric.NewDec(0)

	if w.Validator.Rate.LT(zeroPercent) || w.Validator.Rate.GT(hundredPercent) {
		return errInvalidComissionRate
	}
	if w.Validator.MaxRate.LT(zeroPercent) || w.Validator.MaxRate.GT(hundredPercent) {
		return errInvalidComissionRate
	}
	if w.Validator.MaxChangeRate.LT(zeroPercent) || w.Validator.MaxChangeRate.GT(hundredPercent) {
		return errInvalidComissionRate
	}

	if w.Validator.Rate.GT(w.Validator.MaxRate) {
		return errCommissionRateTooLarge
	}

	if w.Validator.MaxChangeRate.GT(w.Validator.MaxRate) {
		return errCommissionRateTooLarge
	}

	return nil
}

// Description - some possible IRL connections
type Description struct {
	Name            string `json:"name" yaml:"name"`                         // name
	Identity        string `json:"identity" yaml:"identity"`                 // optional identity signature (ex. UPort or Keybase)
	Website         string `json:"website" yaml:"website"`                   // optional website link
	SecurityContact string `json:"security_contact" yaml:"security_contact"` // optional security contact info
	Details         string `json:"details" yaml:"details"`                   // optional details
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

// NewDescription returns a new Description with the provided values.
func NewDescription(name, identity, website, securityContact, details string) Description {
	return Description{
		Name:            name,
		Identity:        identity,
		Website:         website,
		SecurityContact: securityContact,
		Details:         details,
	}
}

// UpdateDescription updates the fields of a given description. An error is
// returned if the resulting description contains an invalid length.
func UpdateDescription(d2 *Description) (Description, error) {
	return NewDescription(
		d2.Name,
		d2.Identity,
		d2.Website,
		d2.SecurityContact,
		d2.Details,
	).EnsureLength()
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

func verifyBLSKeys(pubKeys []shard.BlsPublicKey, pubKeySigs []shard.BlsSignature) error {
	if len(pubKeys) != len(pubKeySigs) {
		return errBLSKeysNotMatchSigs
	}

	for i := 0; i < len(pubKeys); i++ {
		if err := verifyBLSKey(pubKeys[i], pubKeySigs[i]); err != nil {
			return err
		}
	}

	return nil
}

func verifyBLSKey(pubKey shard.BlsPublicKey, pubKeySig shard.BlsSignature) error {
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
	desc, err := UpdateDescription(val.Description)
	if err != nil {
		return nil, err
	}
	commission := Commission{val.CommissionRates, blockNum}
	pubKeys := []shard.BlsPublicKey{}
	pubKeys = append(pubKeys, val.SlotPubKeys...)

	if err = verifyBLSKeys(pubKeys, val.SlotKeySigs); err != nil {
		return nil, err
	}

	// TODO: a new validator should have a minimum of 1 token as self delegation, and that should be added as a delegation entry here.
	v := Validator{
		val.ValidatorAddress, pubKeys,
		new(big.Int), val.MinSelfDelegation, val.MaxTotalDelegation, false,
		commission, desc, blockNum,
	}
	return &v, nil
}

// UpdateValidatorFromEditMsg updates validator from EditValidator message
// TODO check the validity of the fields of edit message
func UpdateValidatorFromEditMsg(validator *Validator, edit *EditValidator) error {
	if validator.Address != edit.ValidatorAddress {
		return errAddressNotMatch
	}
	if edit.Description != nil {
		desc, err := UpdateDescription(edit.Description)
		if err != nil {
			return err
		}

		validator.Description = desc
	}

	if edit.CommissionRate != nil {
		validator.Rate = *edit.CommissionRate
	}

	if edit.MinSelfDelegation != nil && edit.MinSelfDelegation.Cmp(big.NewInt(0)) != 0 {
		validator.MinSelfDelegation = edit.MinSelfDelegation
	}

	if edit.MaxTotalDelegation != nil && edit.MaxTotalDelegation.Cmp(big.NewInt(0)) != 0 {
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
			if err := verifyBLSKey(*edit.SlotKeyToAdd, edit.SlotKeyToAddSig); err != nil {
				return err
			}
			validator.SlotPubKeys = append(validator.SlotPubKeys, *edit.SlotKeyToAdd)
		}
	}
	return nil
}

// String returns a human readable string representation of a validator.
func (v *Validator) String() string {
	return fmt.Sprintf(`Validator
  Address:                    %s
  SlotPubKeys:                %s
  Unbonding Height:           %v
  Minimum Self Delegation:     %v
  Maximum Total Delegation:     %v
  Description:                %v
  Commission:                 %v`,
		common2.MustAddressToBech32(v.Address), printSlotPubKeys(v.SlotPubKeys),
		v.UnbondingHeight,
		v.MinSelfDelegation, v.MaxTotalDelegation, v.Description, v.Commission,
	)
}
