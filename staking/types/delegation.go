package types

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
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
	Reward           *big.Int             `json:"reward" yaml:"reward"`
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

// Undelegate - append entry to the undelegation
func (d *Delegation) Undelegate(epoch *big.Int, amt *big.Int) error {
	if amt.Sign() <= 0 {
		return errInvalidAmount
	}
	if d.Amount.Cmp(amt) < 0 {
		return errInsufficientBalance
	}
	d.Amount.Sub(d.Amount, amt)

	exist := false
	for _, entry := range d.Entries {
		if entry.Epoch.Cmp(epoch) == 0 {
			exist = true
			entry.Amount.Add(entry.Amount, amt)
			return nil
		}
	}

	if !exist {
		item := UndelegationEntry{amt, epoch}
		d.Entries = append(d.Entries, &item)
	}
	return nil
}

// TotalInUndelegation - return the total amount of token in undelegation (locking period)
func (d *Delegation) TotalInUndelegation() *big.Int {
	total := big.NewInt(0)
	for _, entry := range d.Entries {
		total.Add(total, entry.Amount)
	}
	return total
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
