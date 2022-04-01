package types

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	common "github.com/ethereum/go-ethereum/common"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/pkg/errors"
)

var (
	testAddr, _   = common2.Bech32ToAddress("one129r9pj3sk0re76f7zs3qz92rggmdgjhtwge62k")
	delegatorAddr = common.Address(testAddr)
	delegationAmt = big.NewInt(100000)
	// create a new delegation:
	delegation = NewDelegation(delegatorAddr, delegationAmt)
)

func TestUndelegate(t *testing.T) {
	epoch1 := big.NewInt(10)
	amount1 := big.NewInt(1000)
	delegation.Undelegate(epoch1, amount1, nil)

	// check the undelegation's Amount
	if delegation.Undelegations[0].Amount.Cmp(amount1) != 0 {
		t.Errorf("undelegate failed, amount does not match")
	}
	// check the undelegation's Epoch
	if delegation.Undelegations[0].Epoch.Cmp(epoch1) != 0 {
		t.Errorf("undelegate failed, epoch does not match")
	}

	epoch2 := big.NewInt(12)
	amount2 := big.NewInt(2000)
	delegation.Undelegate(epoch2, amount2, nil)

	// check the number of undelegations
	if len(delegation.Undelegations) != 2 {
		t.Errorf("total number of undelegations should have been two")
	}
}

func TestTotalInUndelegation(t *testing.T) {
	var totalAmount = delegation.TotalInUndelegation()

	// check the total amount of undelegation
	if totalAmount.Cmp(big.NewInt(3000)) != 0 {
		t.Errorf("total undelegation amount is not correct")
	}
}

func TestDeleteEntry(t *testing.T) {
	// add the third delegation
	// Undelegations[]: 1000, 2000, 3000
	epoch3 := big.NewInt(15)
	amount3 := big.NewInt(3000)
	delegation.Undelegate(epoch3, amount3, nil)

	// delete the second undelegation entry
	// Undelegations[]: 1000, 3000
	deleteEpoch := big.NewInt(12)
	delegation.DeleteEntry(deleteEpoch)

	// check if the Undelegtaions[1] == 3000
	if delegation.Undelegations[1].Amount.Cmp(big.NewInt(3000)) != 0 {
		t.Errorf("deleting an undelegation entry fails, amount is not correct")
	}
}

func TestUnlockedLastEpochInCommittee(t *testing.T) {
	lastEpochInCommittee := big.NewInt(17)
	curEpoch := big.NewInt(24)

	epoch4 := big.NewInt(21)
	amount4 := big.NewInt(4000)
	delegation.Undelegate(epoch4, amount4, nil)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee, 7, false)
	if result.Cmp(big.NewInt(8000)) != 0 {
		t.Errorf("removing an unlocked undelegation fails")
	}
}

func TestUnlockedLastEpochInCommitteeFail(t *testing.T) {
	delegation := NewDelegation(delegatorAddr, delegationAmt)
	lastEpochInCommittee := big.NewInt(18)
	curEpoch := big.NewInt(24)

	epoch4 := big.NewInt(21)
	amount4 := big.NewInt(4000)
	delegation.Undelegate(epoch4, amount4, nil)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee, 7, false)
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("premature delegation shouldn't be unlocked")
	}
}

func TestUnlockedFullPeriod(t *testing.T) {
	lastEpochInCommittee := big.NewInt(34)
	curEpoch := big.NewInt(34)

	epoch5 := big.NewInt(27)
	amount5 := big.NewInt(4000)
	delegation.Undelegate(epoch5, amount5, nil)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee, 7, false)
	if result.Cmp(big.NewInt(4000)) != 0 {
		t.Errorf("removing an unlocked undelegation fails")
	}
}

func TestQuickUnlock(t *testing.T) {
	lastEpochInCommittee := big.NewInt(44)
	curEpoch := big.NewInt(44)

	epoch7 := big.NewInt(44)
	amount7 := big.NewInt(4000)
	delegation.Undelegate(epoch7, amount7, nil)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee, 0, false)
	if result.Cmp(big.NewInt(4000)) != 0 {
		t.Errorf("removing an unlocked undelegation fails")
	}
}

func TestUnlockedFullPeriodFail(t *testing.T) {
	delegation := NewDelegation(delegatorAddr, delegationAmt)
	lastEpochInCommittee := big.NewInt(34)
	curEpoch := big.NewInt(34)

	epoch5 := big.NewInt(28)
	amount5 := big.NewInt(4000)
	delegation.Undelegate(epoch5, amount5, nil)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee, 7, false)
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("premature delegation shouldn't be unlocked")
	}
}

func TestUnlockedPremature(t *testing.T) {
	lastEpochInCommittee := big.NewInt(44)
	curEpoch := big.NewInt(44)

	epoch6 := big.NewInt(42)
	amount6 := big.NewInt(4000)
	delegation.Undelegate(epoch6, amount6, nil)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee, 7, false)
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("premature delegation shouldn't be unlocked")
	}
}

func TestNoEarlyUnlock(t *testing.T) {
	lastEpochInCommittee := big.NewInt(17)
	curEpoch := big.NewInt(24)

	epoch4 := big.NewInt(21)
	amount4 := big.NewInt(4000)
	delegation.Undelegate(epoch4, amount4, nil)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee, 7, true)
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("should not allow early unlock")
	}
}

func TestMinRemainingDelegation(t *testing.T) {
	// make it again so that the test is idempotent
	delegation = NewDelegation(delegatorAddr, big.NewInt(100000))
	minimumAmount := big.NewInt(50000) // half of the delegation amount
	// first undelegate such that remaining < minimum
	epoch := big.NewInt(10)
	amount := big.NewInt(50001)
	expect := "Minimum: 50000, Remaining: 49999: remaining delegation must be 0 or >= 100 ONE"
	if err := delegation.Undelegate(epoch, amount, minimumAmount); err == nil || err.Error() != expect {
		t.Errorf("Expected error %v but got %v", expect, err)
	}

	// then undelegate such that remaining >= minimum
	amount = big.NewInt(50000)
	epoch = big.NewInt(11)
	if err := delegation.Undelegate(epoch, amount, minimumAmount); err != nil {
		t.Errorf("Expected no error but got %v", err)
	}
	if len(delegation.Undelegations) != 1 {
		t.Errorf("Unexpected length %d", len(delegation.Undelegations))
	}
	if delegation.Amount.Cmp(minimumAmount) != 0 {
		t.Errorf("Unexpected delegation.Amount %d; minimumAmount %d",
			delegation.Amount,
			minimumAmount,
		)
	}

	// finally delegate such that remaining is zero
	epoch = big.NewInt(12)
	if err := delegation.Undelegate(epoch, delegation.Amount, minimumAmount); err != nil {
		t.Errorf("Expected no error but got %v", err)
	}
	if len(delegation.Undelegations) != 2 { // separate epoch
		t.Errorf("Unexpected length %d", len(delegation.Undelegations))
	}
	if delegation.Amount.Cmp(common.Big0) != 0 {
		t.Errorf("Unexpected delegation.Amount %d; minimumAmount %d",
			delegation.Amount,
			common.Big0,
		)
	}
}

func TestMergeDelegationsToAlter(t *testing.T) {
	delegators := [3]common.Address{}
	validators := [3]common.Address{}
	top := make(map[common.Address](map[common.Address]uint64))
	leaf := make(map[common.Address](map[common.Address]uint64))
	expected := make(map[common.Address](map[common.Address]uint64))
	// blank test
	top = MergeDelegationsToAlter(top, leaf)
	if len(top) != 0 {
		t.Error("Length values of top not equal to leaf")
	}
	// reassign to avoid reference issues
	top = make(map[common.Address](map[common.Address]uint64))
	leaf = make(map[common.Address](map[common.Address]uint64))
	for i := 0; i < 3; i++ {
		delegators[i] = makeTestAddr(fmt.Sprintf("delegator%d", i))
		top[delegators[i]] = make(map[common.Address]uint64)
		leaf[delegators[i]] = make(map[common.Address]uint64)
		expected[delegators[i]] = make(map[common.Address]uint64)
		validators[i] = makeTestAddr(fmt.Sprintf("validator%d", i))
	}
	// now start with real test
	// (1) case where leaf says delete, top says move
	leaf[delegators[0]][validators[0]] = math.MaxUint64
	top[delegators[0]][validators[0]] = 1
	result := MergeDelegationsToAlter(top, leaf)
	expected[delegators[0]][validators[0]] = math.MaxUint64
	if err := compareDelegationsToAlter(expected, result); err != nil {
		t.Error(err)
	}
	// (2) case where top says delete, leaf says move
	leaf[delegators[0]][validators[0]] = 1
	top[delegators[0]][validators[0]] = math.MaxUint64
	result = MergeDelegationsToAlter(top, leaf)
	if err := compareDelegationsToAlter(expected, result); err != nil {
		t.Error(err)
	}
	// (3) case where both say move
	leaf[delegators[0]][validators[0]] = 1
	top[delegators[0]][validators[0]] = 1
	result = MergeDelegationsToAlter(top, leaf)
	expected[delegators[0]][validators[0]] = 2
	if err := compareDelegationsToAlter(expected, result); err != nil {
		t.Error(err)
	}
	// (4) case where both say delete
	top[delegators[0]][validators[0]] = math.MaxUint64
	leaf[delegators[0]][validators[0]] = math.MaxUint64
	result = MergeDelegationsToAlter(top, leaf)
	expected[delegators[0]][validators[0]] = math.MaxUint64
	if err := compareDelegationsToAlter(expected, result); err != nil {
		t.Error(err)
	}
	// (5) another guy is being moved, with different values
	// and original is being deleted and a diff combo is moving too
	top[delegators[1]][validators[0]] = 2
	leaf[delegators[1]][validators[0]] = 1
	top[delegators[2]][validators[1]] = 4
	leaf[delegators[2]][validators[1]] = 3
	result = MergeDelegationsToAlter(top, leaf)
	expected[delegators[1]][validators[0]] = 3
	expected[delegators[2]][validators[1]] = 7
	if err := compareDelegationsToAlter(expected, result); err != nil {
		t.Error(err)
	}
}

func TestFindDelegationInWrapper(t *testing.T) {
	config := &params.ChainConfig{}
	config.NoNilDelegationsEpoch = big.NewInt(100)
	// has one extra delegation in there
	wrapper := makeValidValidatorWrapper()
	delegatorAddress := wrapper.Address
	delegationIndex := DelegationIndex{
		ValidatorAddress: delegatorAddress,
		Index:            0,
		BlockNum:         big.NewInt(1),
	}
	// pre no nil delegations epoch
	if delegation, actualIndex, err := FindDelegationInWrapper(
		delegatorAddress,
		&wrapper,
		&delegationIndex,
		config,
		big.NewInt(1),
	); err != nil {
		t.Error(err)
	} else if delegation == nil {
		t.Error("Delegation is nil")
	} else if actualIndex != 0 {
		t.Errorf("Actual index is not 0 but %d", actualIndex)
	}
	// post no nil delegations epoch with self delegation
	if delegation, actualIndex, err := FindDelegationInWrapper(
		delegatorAddress,
		&wrapper,
		&delegationIndex,
		config,
		big.NewInt(200),
	); err != nil {
		t.Error(err)
	} else if delegation == nil {
		t.Error("Delegation is nil")
	} else if actualIndex != 0 {
		t.Errorf("Actual index is not 0 but %d", actualIndex)
	}
	// post no nil delegations epoch with matching index
	delegationIndex.Index = 1
	delegatorAddress = wrapper.Delegations[1].DelegatorAddress
	if delegation, actualIndex, err := FindDelegationInWrapper(
		delegatorAddress,
		&wrapper,
		&delegationIndex,
		config,
		big.NewInt(200),
	); err != nil {
		t.Error(err)
	} else if delegation == nil {
		t.Error("Delegation is nil")
	} else if actualIndex != 1 {
		t.Errorf("Actual index is not 1 but %d", actualIndex)
	}
	// post no nil delegations epoch with too high index (more than length)
	delegationIndex.Index = 2
	if delegation, actualIndex, err := FindDelegationInWrapper(
		delegatorAddress,
		&wrapper,
		&delegationIndex,
		config,
		big.NewInt(200),
	); err != nil {
		t.Error(err)
	} else if delegation == nil {
		t.Error("Delegation is nil")
	} else if actualIndex != 1 {
		t.Errorf("Actual index is not 1 but %d", actualIndex)
	}
	// post no nil delegations epoch with too low index (less than the expected index)
	delegationIndex.Index = 1
	wrapper.Delegations = append(
		wrapper.Delegations,
		NewDelegation(
			common.BigToAddress(common.Big257),
			big.NewInt(500),
		),
	)
	if delegation, _, err := FindDelegationInWrapper(
		wrapper.Delegations[2].DelegatorAddress,
		&wrapper,
		&delegationIndex,
		config,
		big.NewInt(200),
	); err != nil {
		t.Error(err)
	} else if delegation != nil {
		t.Error("Delegation is not nil")
	}
	// pre no nil delegations epoch with messed up lengths
	delegationIndex.Index = 5
	if _, _, err := FindDelegationInWrapper(
		delegatorAddress,
		&wrapper,
		&delegationIndex,
		config,
		big.NewInt(1),
	); err == nil {
		t.Error("Expected error out of bounds but got nil")
	}
}

func makeTestAddr(item interface{}) common.Address {
	s := fmt.Sprintf("harmony-one-%v", item)
	return common.BytesToAddress([]byte(s))
}

// pasted here to avoid import cycle
func compareDelegationsToAlter(expected, delegationsToAlter map[common.Address](map[common.Address]uint64)) error {
	for expectedDelegator, expectedModified := range expected {
		// check it is in delegationsToAlter
		if modified, ok := delegationsToAlter[expectedDelegator]; !ok {
			return errors.Errorf("Did not find %s in delegationsToAlter", expectedDelegator.Hex())
		} else {
			for expectedValidator, expectedOffset := range expectedModified {
				if offset, ok := modified[expectedValidator]; !ok {
					return errors.Errorf("Did not find validator %s for delegator %s in delegationsToAlter", expectedValidator.Hex(), expectedDelegator.Hex())
				} else if offset != expectedOffset {
					return errors.Errorf(
						"Mismatch in validator %s for delegator %s in delegationsToAlter: expected %d actual %d",
						expectedValidator.Hex(),
						expectedDelegator.Hex(),
						expectedOffset,
						offset,
					)
				}
			}
		}
	}

	// opposite check for different keys
	for delegator, modified := range delegationsToAlter {
		if expectedModified, ok := expected[delegator]; !ok {
			return errors.Errorf("Did not find %s in expected", delegator.Hex())
		} else {
			for validator, _ := range modified {
				if _, ok := expectedModified[validator]; !ok {
					return errors.Errorf("Did not find validator %s for delegator %s in expected", validator.Hex(), delegator.Hex())
				}
			}
		}
	}
	return nil
}
