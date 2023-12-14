package types

import (
	"encoding/json"
	"errors"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
)

var (
	errInsufficientBalance = errors.New("insufficient balance to undelegate")
	errInvalidAmount       = errors.New("invalid amount, must be positive")
)

const (
	// LockPeriodInEpoch is the number of epochs a undelegated token needs to be before it's released to the delegator's balance
	LockPeriodInEpoch = 7
	// LockPeriodInEpochV2 there is no extended locking time besides the current epoch time.
	LockPeriodInEpochV2 = 0
)

// Delegation represents the bond with tokens held by an account. It is
// owned by one delegator, and is associated with the voting power of one
// validator.
type Delegation struct {
	DelegatorAddress common.Address
	Amount           *big.Int
	Reward           *big.Int
	Undelegations    Undelegations
}

// Delegations ..
type Delegations []Delegation

// String ..
func (d Delegations) String() string {
	s, _ := json.Marshal(d)
	return string(s)
}

// MarshalJSON ..
func (d Delegation) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		DelegatorAddress string        `json:"delegator-address"`
		Amount           *big.Int      `json:"amount"`
		Reward           *big.Int      `json:"reward"`
		Undelegations    Undelegations `json:"undelegations"`
	}{common2.MustAddressToBech32(d.DelegatorAddress), d.Amount,
		d.Reward, d.Undelegations,
	})
}

func (d Delegation) String() string {
	s, _ := json.Marshal(d)
	return string(s)
}

// Hash is a New256 hash of an RLP encoded Delegation
func (d Delegation) Hash() common.Hash {
	return hash.FromRLPNew256(d)
}

// SetDifference ..
func SetDifference(xs, ys []Delegation) []Delegation {
	diff := []Delegation{}
	xsHashed, ysHashed :=
		make([]common.Hash, len(xs)), make([]common.Hash, len(ys))
	for i := range xs {
		xsHashed[i] = xs[i].Hash()
	}
	for i := range ys {
		ysHashed[i] = ys[i].Hash()
		for j := range xsHashed {
			if ysHashed[j] != xsHashed[j] {
				diff = append(diff, ys[j])
			}
		}
	}

	return diff
}

// Undelegation represents one undelegation entry
type Undelegation struct {
	Amount *big.Int `json:"amount"`
	Epoch  *big.Int `json:"epoch"`
}

// Undelegations ..
type Undelegations []Undelegation

// String ..
func (u Undelegations) String() string {
	s, _ := json.Marshal(u)
	return string(s)
}

// DelegationIndexes is a slice of DelegationIndex
type DelegationIndexes []DelegationIndex

// DelegationIndex stored the index of a delegation in the validator's delegation list
type DelegationIndex struct {
	ValidatorAddress common.Address
	Index            uint64
	BlockNum         *big.Int
}

// NewDelegation creates a new delegation object
func NewDelegation(delegatorAddr common.Address,
	amount *big.Int) Delegation {
	return Delegation{
		DelegatorAddress: delegatorAddr,
		Amount:           amount,
		Reward:           big.NewInt(0),
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
	for _, entry := range d.Undelegations {
		if entry.Epoch.Cmp(epoch) == 0 {
			exist = true
			entry.Amount.Add(entry.Amount, amt)
			return nil
		}
	}

	if !exist {
		item := Undelegation{amt, epoch}
		d.Undelegations = append(d.Undelegations, item)

		// Always sort the undelegate by epoch in increasing order
		sort.SliceStable(
			d.Undelegations,
			func(i, j int) bool { return d.Undelegations[i].Epoch.Cmp(d.Undelegations[j].Epoch) < 0 },
		)
	}

	return nil
}

// TotalInUndelegation - return the total amount of token in undelegation (locking period)
func (d *Delegation) TotalInUndelegation() *big.Int {
	total := big.NewInt(0)
	for i := range d.Undelegations {
		total.Add(total, d.Undelegations[i].Amount)
	}
	return total
}

// DeleteEntry - delete an entry from the undelegation
// Opimize it
func (d *Delegation) DeleteEntry(epoch *big.Int) {
	entries := []Undelegation{}
	for i, entry := range d.Undelegations {
		if entry.Epoch.Cmp(epoch) == 0 {
			entries = append(d.Undelegations[:i], d.Undelegations[i+1:]...)
		}
	}
	if entries != nil {
		d.Undelegations = entries
	}
}

// RemoveUnlockedUndelegations removes all fully unlocked
// undelegations and returns the total sum
func (d *Delegation) RemoveUnlockedUndelegations(
	curEpoch, lastEpochInCommittee *big.Int, lockPeriod int, noEarlyUnlock bool, isMaxRate bool,
) *big.Int {
	totalWithdraw := big.NewInt(0)
	count := 0
	for j := range d.Undelegations {
		epochsSinceUndelegation := big.NewInt(0).Sub(curEpoch, d.Undelegations[j].Epoch).Int64()
		// >=7 epochs have passed since undelegation, or
		lockPeriodApplies := epochsSinceUndelegation >= int64(lockPeriod)
		// >=7 epochs have passed since unelection during the noEarlyUnlock configuration
		earlyUnlockPeriodApplies := big.NewInt(0).Sub(curEpoch, lastEpochInCommittee).Int64() >= int64(lockPeriod) && !noEarlyUnlock
		maxRateApplies := isMaxRate && epochsSinceUndelegation > int64(lockPeriod)
		if lockPeriodApplies || earlyUnlockPeriodApplies {
			if !maxRateApplies {
				totalWithdraw.Add(totalWithdraw, d.Undelegations[j].Amount)
			}
			count++
		} else {
			break
		}

	}
	d.Undelegations = d.Undelegations[count:]
	return totalWithdraw
}
