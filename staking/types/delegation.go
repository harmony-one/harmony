package types

import (
	"errors"
	"math/big"

	"github.com/harmony-one/harmony/internal/common"
)

var (
	errInsufficientBalance = errors.New("Insufficient balance to undelegate")
	errInvalidAmount       = errors.New("Invalid amount, must be positive")
)

// Delegation represents the bond with tokens held by an account. It is
// owned by one delegator, and is associated with the voting power of one
// validator.
type Delegation struct {
	DelegatorAddress common.Address       `json:"delegator_address" yaml:"delegator_address"`
	Amount           *big.Int             `json:"amount" yaml:"amount"`
	Entries          []*UndelegationEntry `json:"entries" yaml:"entries"`
}

// UndelegationEntry represents one undelegation entry
type UndelegationEntry struct {
	Amount *big.Int
	Epoch  *big.Int
}

// NewDelegation creates a new delegation object
func NewDelegation(delegatorAddr common.Address,
	amount *big.Int) Delegation {
	return Delegation{
		DelegatorAddress: delegatorAddr,
		Amount:           amount,
	}
}

// AddEntry - append entry to the undelegation
func (d *Delegation) AddEntry(epoch *big.Int, amt *big.Int) error {
	if d.Amount.Cmp(amt) < 0 {
		return errInsufficientBalance
	}
	if amt.Sign() <= 0 {
		return errInvalidAmount
	}
	d.Amount.Sub(d.Amount, amt)

	for _, entry := range d.Entries {
		if entry.Epoch.Cmp(epoch) == 0 {
			entry.Amount.Add(entry.Amount, amt)
			return nil
		}
	}
	item := UndelegationEntry{amt, epoch}
	d.Entries = append(d.Entries, &item)
	return nil
}

// DeleteEntry - delete an entry from the undelegation
// Opimize it
func (d *Delegation) DeleteEntry(epoch *big.Int) {
	entries := []*UndelegationEntry{}
	for i, entry := range d.Entries {
		if entry.Epoch.Cmp(epoch) == 0 {
			entries = append(d.Entries[:i], d.Entries[i+1:]...)
		}
	}
	if entries != nil {
		d.Entries = entries
	}
}
