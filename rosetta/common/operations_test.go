package common

import (
	"reflect"
	"sort"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	staking "github.com/harmony-one/harmony/staking/types"
)

// WARNING: Careful for client side dependencies when changing operation status!
func TestOperationStatus(t *testing.T) {
	if SuccessOperationStatus.Status != "success" {
		t.Errorf("Successfull operation status must be 'success'")
	}
	if ContractFailureOperationStatus.Status != "contract_failure" {
		t.Errorf("Contract failure status must be 'contract_failure'")
	}
	if FailureOperationStatus.Status != "failure" {
		t.Errorf("Failture status must be 'failure'")
	}
}

// WARNING: Careful for client side dependencies when changing operation status!
func TestOperationSuccessful(t *testing.T) {
	successfulOperations := []*types.OperationStatus{
		SuccessOperationStatus,
	}
	unsuccessfulOperations := []*types.OperationStatus{
		ContractFailureOperationStatus,
		FailureOperationStatus,
	}

	for _, status := range successfulOperations {
		if status.Successful != true {
			t.Errorf("Expect operation %v to be a successful operation", status)
		}
	}

	for _, status := range unsuccessfulOperations {
		if status.Successful != false {
			t.Errorf("Expect operation %v to be an unsuccessful operation", status)
		}
	}
}

// WARNING: Careful for client side dependencies when changing operation status!
func TestPlainOperationTypes(t *testing.T) {
	plainOperationTypes := PlainOperationTypes
	referenceOperationTypes := []string{
		ExpendGasOperation,
		TransferNativeOperation,
		CrossShardTransferNativeOperation,
		ContractCreationOperation,
		GenesisFundsOperation,
		PreStakingBlockRewardOperation,
		UndelegationPayoutOperation,
	}
	sort.Strings(referenceOperationTypes)
	sort.Strings(plainOperationTypes)

	if !reflect.DeepEqual(referenceOperationTypes, plainOperationTypes) {
		t.Errorf("operation types are invalid")
	}
}

func TestStakingOperationTypes(t *testing.T) {
	stakingOperationTypes := StakingOperationTypes
	referenceOperationTypes := []string{
		staking.DirectiveCreateValidator.String(),
		staking.DirectiveEditValidator.String(),
		staking.DirectiveDelegate.String(),
		staking.DirectiveUndelegate.String(),
		staking.DirectiveCollectRewards.String(),
	}
	sort.Strings(referenceOperationTypes)
	sort.Strings(stakingOperationTypes)

	if !reflect.DeepEqual(referenceOperationTypes, stakingOperationTypes) {
		t.Errorf("operation types are invalid")
	}
}
