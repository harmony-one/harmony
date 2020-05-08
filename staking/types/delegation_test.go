package types

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"

	common "github.com/ethereum/go-ethereum/common"
	common2 "github.com/harmony-one/harmony/internal/common"
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
	delegation.Undelegate(epoch1, amount1)

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
	delegation.Undelegate(epoch2, amount2)

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
	delegation.Undelegate(epoch3, amount3)

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
	delegation.Undelegate(epoch4, amount4)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee)
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
	delegation.Undelegate(epoch4, amount4)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee)
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("premature delegation shouldn't be unlocked")
	}
}

func TestUnlockedFullPeriod(t *testing.T) {
	lastEpochInCommittee := big.NewInt(34)
	curEpoch := big.NewInt(34)

	epoch5 := big.NewInt(27)
	amount5 := big.NewInt(4000)
	delegation.Undelegate(epoch5, amount5)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee)
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
	delegation.Undelegate(epoch5, amount5)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee)
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("premature delegation shouldn't be unlocked")
	}
}

func TestUnlockedPremature(t *testing.T) {
	lastEpochInCommittee := big.NewInt(44)
	curEpoch := big.NewInt(44)

	epoch6 := big.NewInt(42)
	amount6 := big.NewInt(4000)
	delegation.Undelegate(epoch6, amount6)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommittee)
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("premature delegation shouldn't be unlocked")
	}
}

func TestDelegation_Copy(t *testing.T) {
	tests := []struct {
		d Delegation
	}{
		{makeNonZeroDelegation()},
		{makeZeroDelegation()},
		{Delegation{}},
	}
	for i, test := range tests {
		cp := test.d.Copy()
		if err := assertDelegationDeepCopy(cp, test.d); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func makeNonZeroDelegation() Delegation {
	return Delegation{
		DelegatorAddress: common.BigToAddress(common.Big1),
		Amount:           common.Big1,
		Reward:           common.Big1,
		Undelegations: Undelegations{
			Undelegation{
				Amount: common.Big1,
				Epoch:  common.Big1,
			},
			Undelegation{
				Amount: common.Big0,
				Epoch:  common.Big0,
			},
		},
	}
}

func makeZeroDelegation() Delegation {
	return Delegation{
		Amount:        common.Big0,
		Reward:        common.Big0,
		Undelegations: make(Undelegations, 0),
	}
}

func assertDelegationsDeepCopy(ds1, ds2 Delegations) error {
	if !reflect.DeepEqual(ds1, ds2) {
		return fmt.Errorf("not deep equal")
	}
	for i := range ds1 {
		if err := assertDelegationDeepCopy(ds1[i], ds2[i]); err != nil {
			return fmt.Errorf("[%v]: %v", i, err)
		}
	}
	return nil
}

func assertDelegationDeepCopy(d1, d2 Delegation) error {
	if !reflect.DeepEqual(d1, d2) {
		return fmt.Errorf("not deep equal")
	}
	if d1.Amount != nil && d1.Amount == d2.Amount {
		return fmt.Errorf("amount same address")
	}
	if d1.Reward != nil && d1.Reward == d2.Reward {
		return fmt.Errorf("reward same address")
	}
	if err := assertUndelegationsDeepCopy(d1.Undelegations, d2.Undelegations); err != nil {
		return fmt.Errorf("undelegations %v", err)
	}
	return nil
}

func assertUndelegationsDeepCopy(uds1, uds2 Undelegations) error {
	if !reflect.DeepEqual(uds1, uds2) {
		return fmt.Errorf("not deep equal")
	}
	for i := range uds1 {
		if err := assertUndelegationDeepCopy(uds1[i], uds2[i]); err != nil {
			return fmt.Errorf("[%v]: %v", i, err)
		}
	}
	return nil
}

func assertUndelegationDeepCopy(ud1, ud2 Undelegation) error {
	if !reflect.DeepEqual(ud1, ud2) {
		return fmt.Errorf("not deep equal")
	}
	if ud1.Amount != nil && ud1.Amount == ud2.Amount {
		return fmt.Errorf("amount same address")
	}
	if ud1.Epoch != nil && ud1.Epoch == ud2.Epoch {
		return fmt.Errorf("epoch same address")
	}
	return nil
}
