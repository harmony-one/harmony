package types

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/internal/common"
)

// DVPair is struct that just has a delegator-validator pair with no other data.
// It is intended to be used as a marshalable pointer. For example, a DVPair can be used to construct the
// key to getting an UnbondingDelegation from state.
type DVPair struct {
	DelegatorAddress common.Address
	ValidatorAddress common.Address
}

// DVVTriplet is struct that just has a delegator-validator-validator triplet with no other data.
// It is intended to be used as a marshalable pointer. For example, a DVVTriplet can be used to construct the
// key to getting a Redelegation from state.
type DVVTriplet struct {
	DelegatorAddress    common.Address
	ValidatorSrcAddress common.Address
	ValidatorDstAddress common.Address
}

// Delegation represents the bond with tokens held by an account. It is
// owned by one delegator, and is associated with the voting power of one
// validator.
type Delegation struct {
	DelegatorAddress common.Address `json:"delegator_address" yaml:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address" yaml:"validator_address"`
	Amount           *big.Int       `json:"amount" yaml:"amount"`
}

// NewDelegation creates a new delegation object
func NewDelegation(delegatorAddr common.Address, validatorAddr common.Address,
	amount *big.Int) Delegation {

	return Delegation{
		DelegatorAddress: delegatorAddr,
		ValidatorAddress: validatorAddr,
		Amount:           amount,
	}
}

// MarshalDelegation return the delegation
func MarshalDelegation(delegation Delegation) ([]byte, error) {
	return rlp.EncodeToBytes(delegation)
}

// UnmarshalDelegation return the delegation
func UnmarshalDelegation(by []byte) (*Delegation, error) {
	decoded := &Delegation{}
	err := rlp.DecodeBytes(by, decoded)
	return decoded, err
}

// GetDelegatorAddr returns DelegatorAddr
func (d Delegation) GetDelegatorAddr() common.Address { return d.DelegatorAddress }

// GetValidatorAddr returns ValidatorAddr
func (d Delegation) GetValidatorAddr() common.Address { return d.ValidatorAddress }

// GetAmount returns amount of a delegation
func (d Delegation) GetAmount() *big.Int { return d.Amount }

// String returns a human readable string representation of a Delegation.
func (d Delegation) String() string {
	return fmt.Sprintf(`Delegation:
  Delegator: %s
  Validator: %s
  Amount:    %s`, d.DelegatorAddress,
		d.ValidatorAddress, d.Amount)
}

// Delegations is a collection of delegations
type Delegations []Delegation

// String returns the string representation of a list of delegations
func (d Delegations) String() (out string) {
	for _, del := range d {
		out += del.String() + "\n"
	}
	return strings.TrimSpace(out)
}

// UnbondingDelegation stores all of a single delegator's unbonding bonds
// for a single validator in an time-ordered list
type UnbondingDelegation struct {
	DelegatorAddress common.Address             `json:"delegator_address" yaml:"delegator_address"` // delegator
	ValidatorAddress common.Address             `json:"validator_address" yaml:"validator_address"` // validator unbonding from operator addr
	Entries          []UnbondingDelegationEntry `json:"entries" yaml:"entries"`                     // unbonding delegation entries
}

// UnbondingDelegationEntry - entry to an UnbondingDelegation
type UnbondingDelegationEntry struct {
	ExitEpoch *big.Int `json:"exit_epoch" yaml:"exit_epoch"` // epoch which the unbonding begins
	Amount    *big.Int `json:"amount" yaml:"amount"`         // atoms to receive at completion
}

// NewUnbondingDelegation - create a new unbonding delegation object
func NewUnbondingDelegation(delegatorAddr common.Address,
	validatorAddr common.Address, epoch *big.Int, amt *big.Int) UnbondingDelegation {

	entry := NewUnbondingDelegationEntry(epoch, amt)
	return UnbondingDelegation{
		DelegatorAddress: delegatorAddr,
		ValidatorAddress: validatorAddr,
		Entries:          []UnbondingDelegationEntry{entry},
	}
}

// NewUnbondingDelegationEntry - create a new unbonding delegation object
func NewUnbondingDelegationEntry(epoch *big.Int, amt *big.Int) UnbondingDelegationEntry {
	return UnbondingDelegationEntry{
		ExitEpoch: epoch,
		Amount:    amt,
	}
}

// AddEntry - append entry to the unbonding delegation
// if there exists same ExitEpoch entry, merge the amount
// TODO: check the total amount not exceed the staking amount call this function
func (d *UnbondingDelegation) AddEntry(epoch *big.Int, amt *big.Int) {
	entry := NewUnbondingDelegationEntry(epoch, amt)
	for i := range d.Entries {
		if d.Entries[i].ExitEpoch == entry.ExitEpoch {
			d.Entries[i].Amount.Add(d.Entries[i].Amount, entry.Amount)
			return
		}
	}
	// same exit epoch entry not found
	d.Entries = append(d.Entries, entry)
	return
}

// String returns a human readable string representation of an UnbondingDelegation.
func (d UnbondingDelegation) String() string {
	out := fmt.Sprintf(`Unbonding Delegations between:
      Delegator:          %s
      Validator:          %s
	  Entries:`, d.DelegatorAddress, d.ValidatorAddress)
	for i, entry := range d.Entries {
		out += fmt.Sprintf(`    Unbonding Delegation %d:
          ExitEpoch:          %v
          Amount:             %s`, i, entry.ExitEpoch, entry.Amount)
	}
	return out
}

// UnbondingDelegations is a collection of UnbondingDelegation
type UnbondingDelegations []UnbondingDelegation

func (ubds UnbondingDelegations) String() (out string) {
	for _, u := range ubds {
		out += u.String() + "\n"
	}
	return strings.TrimSpace(out)
}

// Redelegation contains the list of a particular delegator's
// redelegating bonds from a particular source validator to a
// particular destination validator
type Redelegation struct {
	DelegatorAddress    common.Address      `json:"delegator_address" yaml:"delegator_address"`         // delegator
	ValidatorSrcAddress common.Address      `json:"validator_src_address" yaml:"validator_src_address"` // validator redelegation source operator addr
	ValidatorDstAddress common.Address      `json:"validator_dst_address" yaml:"validator_dst_address"` // validator redelegation destination operator addr
	Entries             []RedelegationEntry `json:"entries" yaml:"entries"`                             // redelegation entries
}

// RedelegationEntry - entry to a Redelegation
type RedelegationEntry struct {
	Epoch  *big.Int `json:"epoch" yaml:"epoch"`   // epoch at which the redelegation took place
	Amount *big.Int `json:"amount" yaml:"amount"` // amount of destination-validator tokens created by redelegation
}

// NewRedelegation - create a new redelegation object
func NewRedelegation(delegatorAddr common.Address, validatorSrcAddr,
	validatorDstAddr common.Address, epoch *big.Int, amt *big.Int) Redelegation {
	entry := NewRedelegationEntry(epoch, amt)
	return Redelegation{
		DelegatorAddress:    delegatorAddr,
		ValidatorSrcAddress: validatorSrcAddr,
		ValidatorDstAddress: validatorDstAddr,
		Entries:             []RedelegationEntry{entry},
	}
}

// NewRedelegationEntry - create a new redelegation object
func NewRedelegationEntry(epoch *big.Int, amt *big.Int) RedelegationEntry {
	return RedelegationEntry{
		Epoch:  epoch,
		Amount: amt,
	}
}

// AddEntry - append entry to the unbonding delegation
// Merge if has same epoch field
func (d *Redelegation) AddEntry(epoch *big.Int, amt *big.Int) {
	entry := NewRedelegationEntry(epoch, amt)

	for i := range d.Entries {
		if d.Entries[i].Epoch == entry.Epoch {
			d.Entries[i].Amount.Add(d.Entries[i].Amount, entry.Amount)
			return
		}
	}
	// same epoch entry not found
	d.Entries = append(d.Entries, entry)
	return
}

// String returns a human readable string representation of a Redelegation.
func (d Redelegation) String() string {
	out := fmt.Sprintf(`Redelegations between:
  Delegator:                 %s
  Source Validator:          %s
  Destination Validator:     %s
  Entries:
`,
		d.DelegatorAddress, d.ValidatorSrcAddress, d.ValidatorDstAddress,
	)

	for i, entry := range d.Entries {
		out += fmt.Sprintf(`    Redelegation Entry #%d:
      Epoch:                     %v
      Amount:                    %v
`,
			i, entry.Epoch, entry.Amount,
		)
	}

	return strings.TrimRight(out, "\n")
}

// Redelegations are a collection of Redelegation
type Redelegations []Redelegation

func (d Redelegations) String() (out string) {
	for _, red := range d {
		out += red.String() + "\n"
	}
	return strings.TrimSpace(out)
}
