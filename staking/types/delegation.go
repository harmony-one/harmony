package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/pkg/errors"
)

var (
	errInsufficientBalance   = errors.New("insufficient balance to undelegate")
	errInvalidAmount         = errors.New("invalid amount, must be positive")
	ErrUndelegationRemaining = errors.New("remaining delegation must be 0 or >= 100 ONE")
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
func (d *Delegation) Undelegate(
	epoch *big.Int,
	amt *big.Int,
	minimumRemainingDelegation *big.Int,
) error {
	if amt.Sign() <= 0 {
		return errInvalidAmount
	}
	if d.Amount.Cmp(amt) < 0 {
		return errInsufficientBalance
	}
	if minimumRemainingDelegation != nil {
		finalAmount := big.NewInt(0).Sub(d.Amount, amt)
		if finalAmount.Cmp(minimumRemainingDelegation) < 0 && finalAmount.Cmp(common.Big0) != 0 {
			return errors.Wrapf(ErrUndelegationRemaining,
				fmt.Sprintf("Minimum: %d, Remaining: %d", minimumRemainingDelegation, finalAmount),
			)
		}
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
	curEpoch, lastEpochInCommittee *big.Int, lockPeriod int, noEarlyUnlock bool,
) *big.Int {
	totalWithdraw := big.NewInt(0)
	count := 0
	for j := range d.Undelegations {
		if big.NewInt(0).Sub(curEpoch, d.Undelegations[j].Epoch).Int64() >= int64(lockPeriod) ||
			(!noEarlyUnlock && big.NewInt(0).Sub(curEpoch, lastEpochInCommittee).Int64() >= int64(lockPeriod)) {
			// need to wait at least 7 epochs to withdraw; or the validator has been out of committee for 7 epochs
			totalWithdraw.Add(totalWithdraw, d.Undelegations[j].Amount)
			count++
		} else {
			break
		}

	}
	d.Undelegations = d.Undelegations[count:]
	return totalWithdraw
}

func MergeDelegationsToAlter(
	top map[common.Address](map[common.Address]uint64),
	leaf map[common.Address](map[common.Address]uint64),
) map[common.Address](map[common.Address]uint64) {
	if len(top) == 0 { // first merge
		return leaf
	}
	for delegator, mod := range leaf {
		var topDelegator map[common.Address]uint64
		var ok bool
		if topDelegator, ok = top[delegator]; !ok {
			topDelegator = make(map[common.Address]uint64)
		}
		// iterate over offsets
		for validator, offset := range mod {
			// check whether such an offset exists
			if topOffset, ok := topDelegator[validator]; !ok {
				// if not, set it
				topDelegator[validator] = offset
			} else {
				// else, add to existing
				if topOffset == math.MaxUint64 { // marked for deletion by top
					continue
				} else if offset == math.MaxUint64 { // marked for deletion by leaf
					topDelegator[validator] = offset
				} else {
					topDelegator[validator] += offset // increase the offset
				}
			}
		}
		// insert delegator into top
		top[delegator] = topDelegator
	}
	return top
}

// func PrintDelegationsToAlter(
// 	delegationsToAlter map[common.Address](map[common.Address]uint64),
// ) {
// 	for delegatorAddress, x := range delegationsToAlter {
// 		fmt.Printf("Delegator: %s\n", delegatorAddress.Hex())
// 		for validatorAddress, offset := range x {
// 			fmt.Printf("\tValidator: %s, Offset: %d\n", validatorAddress.Hex(), offset)
// 		}
// 	}
// }

func FindDelegationInWrapper(
	delegatorAddress common.Address,
	wrapper *ValidatorWrapper,
	delegationIndex *DelegationIndex,
	chainConfig *params.ChainConfig,
	epoch *big.Int,
) (*Delegation, uint64, error) {
	var delegation *Delegation
	if !chainConfig.IsPostNoNilDelegations(epoch) {
		if delegationIndex.Index >= uint64(len(wrapper.Delegations)) {
			// index out of bounds
			utils.Logger().Warn().
				Str("validator", delegationIndex.ValidatorAddress.String()).
				Uint64("delegation index", delegationIndex.Index).
				Int("delegations length", len(wrapper.Delegations)).
				Msg("Delegation index out of bound")
			return nil, math.MaxUint64, errors.New("Delegation index out of bound")
		}
		return &wrapper.Delegations[delegationIndex.Index], delegationIndex.Index, nil
	} else {
		if delegationIndex.Index == 0 {
			// since this is referring to self delegation
			// and it is never removed
			// no address comparison is needed
			return &wrapper.Delegations[0], 0, nil
		}
		var startingPoint uint64
		if delegationIndex.Index >= uint64(len(wrapper.Delegations)) {
			// we never remove the (inactive) validator
			// so this will never overflow
			startingPoint = uint64(len(wrapper.Delegations)) - 1
		} else {
			startingPoint = delegationIndex.Index
		}
		// delegationIndex is as of previous block
		// and between the two blocks only removal takes place
		// go backwards only, not forward
		// do not check 0 because we've already taken care of
		// validator's self delegation
		for j := startingPoint; j > 0; j-- {
			delegation = &wrapper.Delegations[j]
			if bytes.Equal(
				delegation.DelegatorAddress[:],
				delegatorAddress[:],
			) {
				return delegation, j, nil
			}
		}
	}
	return nil, math.MaxUint64, nil
}
