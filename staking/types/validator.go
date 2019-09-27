package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
)

// Define validator staking related const
const (
	MaxNameLength            = 70
	MaxIdentityLength        = 3000
	MaxWebsiteLength         = 140
	MaxSecurityContactLength = 140
	MaxDetailsLength         = 280
)

// Validator - data fields for a validator
type Validator struct {
	Address           common.Address `json:"address" yaml:"address"`                         // ECDSA address of the validator
	ValidatingPubKey  bls.PublicKey  `json:"validating_pub_key" yaml:"validating_pub_key"`   // The BLS public key of the validator for consensus
	Description       Description    `json:"description" yaml:"description"`                 // description for the validator
	Active            bool           `json:"active" yaml:"active"`                           // Is the validator active in the validating process or not
	Stake             *big.Int       `json:"stake" yaml:"stake"`                             // The stake put by the validator itself
	UnbondingHeight   *big.Int       `json:"unbonding_height" yaml:"unbonding_height"`       // if unbonding, height at which this validator has begun unbonding
	Commission        Commission     `json:"commission" yaml:"commission"`                   // commission parameters
	MinSelfDelegation *big.Int       `json:"min_self_delegation" yaml:"min_self_delegation"` // validator's self declared minimum self delegation
}

// Description - description fields for a validator
type Description struct {
	Name            string `json:"name" yaml:"name"`                         // name
	Identity        string `json:"identity" yaml:"identity"`                 // optional identity signature (ex. UPort or Keybase)
	Website         string `json:"website" yaml:"website"`                   // optional website link
	SecurityContact string `json:"security_contact" yaml:"security_contact"` // optional security contact info
	Details         string `json:"details" yaml:"details"`                   // optional details
}

// NewValidator - initialize a new validator
func NewValidator(addr common.Address, pubKey bls.PublicKey, description Description, commission Commission, minAmt *big.Int) Validator {
	return Validator{
		Address:           addr,
		ValidatingPubKey:  pubKey,
		Active:            true,
		Stake:             new(big.Int),
		Description:       description,
		UnbondingHeight:   new(big.Int),
		Commission:        commission,
		MinSelfDelegation: minAmt,
	}
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
func (d Description) UpdateDescription(d2 Description) (Description, error) {
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

func (v Validator) GetAddress() common.Address     { return v.Address }
func (v Validator) IsActive() bool                 { return v.Active }
func (v Validator) GetName() string                { return v.Description.Name }
func (v Validator) GetStake() *big.Int             { return v.Stake }
func (v Validator) GetCommissionRate() Dec         { return v.Commission.Rate }
func (v Validator) GetMinSelfDelegation() *big.Int { return v.MinSelfDelegation }
