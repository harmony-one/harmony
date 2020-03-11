package types

import (
	"math/big"
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

func TestRemoveUnlockUndelegations(t *testing.T) {
	lastEpochInCommitte := big.NewInt(16)
	curEpoch := big.NewInt(24)

	epoch4 := big.NewInt(21)
	amount4 := big.NewInt(4000)
	delegation.Undelegate(epoch4, amount4)

	result := delegation.RemoveUnlockedUndelegations(curEpoch, lastEpochInCommitte)
	if result.Cmp(big.NewInt(8000)) != 0 {
		t.Errorf("removing an unlocked undelegation fails")
	}
}
