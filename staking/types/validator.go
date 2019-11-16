package types

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/harmony-one/harmony/common/denominations"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
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
)

var (
	errAddressNotMatch           = errors.New("Validator key not match")
	errInvalidSelfDelegation     = errors.New("self delegation can not be less than min_self_delegation")
	errInvalidTotalDelegation    = errors.New("total delegation can not be bigger than max_total_delegation")
	errMinSelfDelegationTooSmall = errors.New("min_self_delegation has to be greater than 1 ONE")
	errInvalidMaxTotalDelegation = errors.New("max_total_delegation can not be less than min_self_delegation")
)

// ValidatorWrapper contains validator and its delegation information
type ValidatorWrapper struct {
	Validator   `json:"validator" yaml:"validator" rlp:"nil"`
	Delegations []Delegation `json:"delegations" yaml:"delegations" rlp:"nil"`
	// TODO: move snapshot into off-chain db.
	SnapshotValidator   *Validator   `json:"snapshot_validator" yaml:"snaphost_validator" rlp:"nil"`
	SnapshotDelegations []Delegation `json:"snapshot_delegations" yaml:"snapshot_delegations" rlp:"nil"`
}

// Validator - data fields for a validator
type Validator struct {
	// ECDSA address of the validator
	Address common.Address `json:"address" yaml:"address"`
	// The BLS public key of the validator for consensus
	SlotPubKeys []shard.BlsPublicKey `json:"validating_pub_key" yaml:"validating_pub_key"`
	// The stake put by the validator itself
	Stake *big.Int `json:"stake" yaml:"stake"`
	// if unbonding, height at which this validator has begun unbonding
	UnbondingHeight *big.Int `json:"unbonding_height" yaml:"unbonding_height"`
	// validator's self declared minimum self delegation
	MinSelfDelegation *big.Int `json:"min_self_delegation" yaml:"min_self_delegation"`
	// maximum total delgation allowed
	MaxTotalDelegation *big.Int `json:"min_self_delegation" yaml:"min_self_delegation"`
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
	// MinSelfDelegation must be >= 1 ONE
	if w.Validator.MinSelfDelegation.Cmp(big.NewInt(denominations.One)) < 0 {
		return errMinSelfDelegationTooSmall
	}

	// MaxTotalDelegation must not be less than MinSelfDelegation
	if w.Validator.MaxTotalDelegation.Cmp(w.Validator.MinSelfDelegation) < 0 {
		return errInvalidMaxTotalDelegation
	}

	selfDelegation := w.Delegations[0].Amount
	// Self delegation must be >= MinSelfDelegation
	if selfDelegation.Cmp(w.Validator.MinSelfDelegation) < 0 {
		return errInvalidSelfDelegation
	}

	totalDelegation := w.TotalDelegation()
	// Total delegation must be <= MaxTotalDelegation
	if totalDelegation.Cmp(w.Validator.MaxTotalDelegation) > 0 {
		return errInvalidTotalDelegation
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

// GetStake returns the total staking amount
func (v *Validator) GetStake() *big.Int { return v.Stake }

// GetCommissionRate returns the commission rate of the validator
func (v *Validator) GetCommissionRate() numeric.Dec { return v.Commission.Rate }

// GetMinSelfDelegation returns the minimum amount the validator must stake
func (v *Validator) GetMinSelfDelegation() *big.Int { return v.MinSelfDelegation }

// CreateValidatorFromNewMsg creates validator from NewValidator message
func CreateValidatorFromNewMsg(val *CreateValidator, blockNum *big.Int) (*Validator, error) {
	desc, err := UpdateDescription(val.Description)
	if err != nil {
		return nil, err
	}
	commission := Commission{val.CommissionRates, blockNum}
	pubKeys := []shard.BlsPublicKey{}
	pubKeys = append(pubKeys, val.SlotPubKeys...)

	// TODO: a new validator should have a minimum of 1 token as self delegation, and that should be added as a delegation entry here.
	v := Validator{
		val.ValidatorAddress, pubKeys,
		val.Amount, new(big.Int), val.MinSelfDelegation, val.MaxTotalDelegation, false,
		commission, desc, blockNum}
	return &v, nil
}

// UpdateValidatorFromEditMsg updates validator from EditValidator message
// TODO check the validity of the fields of edit message
func UpdateValidatorFromEditMsg(validator *Validator, edit *EditValidator) error {
	if validator.Address != edit.ValidatorAddress {
		return errAddressNotMatch
	}
	desc, err := UpdateDescription(edit.Description)
	if err != nil {
		return err
	}

	validator.Description = desc

	if edit.CommissionRate != nil {
		validator.Rate = *edit.CommissionRate
	}

	if edit.MinSelfDelegation != nil {
		validator.MinSelfDelegation = edit.MinSelfDelegation
	}

	if edit.MaxTotalDelegation != nil {
		validator.MaxTotalDelegation = edit.MaxTotalDelegation
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
			validator.SlotPubKeys = append(validator.SlotPubKeys, *edit.SlotKeyToAdd)
		}
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
	return nil
}

// String returns a human readable string representation of a validator.
func (v *Validator) String() string {
	return fmt.Sprintf(`Validator
  Address:                    %s
  SlotPubKeys:                %s
  Stake:                      %s
  Unbonding Height:           %v
  Minimum SelfDelegation:     %v
  Description:                %v
  Commission:                 %v`,
		common2.MustAddressToBech32(v.Address), printSlotPubKeys(v.SlotPubKeys),
		v.Stake, v.UnbondingHeight,
		v.MinSelfDelegation, v.Description, v.Commission,
	)
}
