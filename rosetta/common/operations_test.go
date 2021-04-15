package common

import (
	"math/big"
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
		NativeTransferOperation,
		NativeCrossShardTransferOperation,
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

func TestCreateValidatorOperationMetadata_UnmarshalFromInterface(t *testing.T) {
	data := map[string]interface{}{
		"validatorAddress":   "one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy",
		"commissionRate":     100000000000000000,
		"maxCommissionRate":  900000000000000000,
		"maxChangeRate":      50000000000000000,
		"minSelfDelegation":  10,
		"maxTotalDelegation": 3000,
		"amount":             100,
		"name":               "Alice",
		"website":            "alice.harmony.one",
		"identity":           "alice",
		"securityContact":    "Bob",
		"details":            "Don't mess with me!!!",
	}
	s := CreateValidatorOperationMetadata{}
	err := s.UnmarshalFromInterface(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.ValidatorAddress != "one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy" {
		t.Fatal("wrong validator address")
	}
	if s.CommissionRate.Cmp(new(big.Int).SetInt64(100000000000000000)) != 0 {
		t.Fatal("wrong commission rate")
	}
	if s.MaxCommissionRate.Cmp(new(big.Int).SetInt64(900000000000000000)) != 0 {
		t.Fatal("wrong max commission rate")
	}
	if s.MaxChangeRate.Cmp(new(big.Int).SetInt64(50000000000000000)) != 0 {
		t.Fatal("wrong max change rate")
	}
	if s.MinSelfDelegation.Cmp(new(big.Int).SetInt64(10)) != 0 {
		t.Fatal("wrong min self delegation")
	}
	if s.MaxTotalDelegation.Cmp(new(big.Int).SetInt64(3000)) != 0 {
		t.Fatal("wrong max total delegation")
	}
	if s.Amount.Cmp(new(big.Int).SetInt64(100)) != 0 {
		t.Fatal("wrong amount")
	}
	if s.Name != "Alice" {
		t.Fatal("wrong name")
	}
	if s.Website != "alice.harmony.one" {
		t.Fatal("wrong website")
	}
	if s.Identity != "alice" {
		t.Fatal("wrong identity")
	}
	if s.SecurityContact != "Bob" {
		t.Fatal("wrong security contact")
	}
	if s.Details != "Don't mess with me!!!" {
		t.Fatal("wrong detail")
	}
}

func TestEditValidatorOperationMetadata_UnmarshalFromInterface(t *testing.T) {
	data := map[string]interface{}{
		"validatorAddress":   "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9",
		"commissionRate":     100000000000000000,
		"minSelfDelegation":  10,
		"maxTotalDelegation": 3000,
		"name":               "Alice",
		"website":            "alice.harmony.one",
		"identity":           "alice",
		"securityContact":    "Bob",
		"details":            "Don't mess with me!!!",
	}
	s := EditValidatorOperationMetadata{}
	err := s.UnmarshalFromInterface(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.ValidatorAddress != "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9" {
		t.Fatal("wrong validator address")
	}
	if s.CommissionRate.Cmp(new(big.Int).SetInt64(100000000000000000)) != 0 {
		t.Fatal("wrong commission rate")
	}
	if s.MinSelfDelegation.Cmp(new(big.Int).SetInt64(10)) != 0 {
		t.Fatal("wrong min self delegation")
	}
	if s.MaxTotalDelegation.Cmp(new(big.Int).SetInt64(3000)) != 0 {
		t.Fatal("wrong max total delegation")
	}
	if s.Name != "Alice" {
		t.Fatal("wrong name")
	}
	if s.Website != "alice.harmony.one" {
		t.Fatal("wrong website")
	}
	if s.Identity != "alice" {
		t.Fatal("wrong identity")
	}
	if s.SecurityContact != "Bob" {
		t.Fatal("wrong security contact")
	}
	if s.Details != "Don't mess with me!!!" {
		t.Fatal("wrong detail")
	}
}

func TestDelegateOperationMetadata_UnmarshalFromInterface(t *testing.T) {
	data := map[string]interface{}{
		"validatorAddress": "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9",
		"delegatorAddress": "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9",
		"amount":           20000,
	}
	s := DelegateOperationMetadata{}
	err := s.UnmarshalFromInterface(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.ValidatorAddress != "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9" {
		t.Fatal("wrong validator address")
	}
	if s.DelegatorAddress != "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9" {
		t.Fatal("wrong delegator address")
	}
	if s.Amount.Cmp(new(big.Int).SetInt64(20000)) != 0 {
		t.Fatal("wrong amount")
	}
}

func TestUndelegateOperationMetadata_UnmarshalFromInterface(t *testing.T) {
	data := map[string]interface{}{
		"validatorAddress": "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9",
		"delegatorAddress": "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9",
		"amount":           20000,
	}
	s := UndelegateOperationMetadata{}
	err := s.UnmarshalFromInterface(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.ValidatorAddress != "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9" {
		t.Fatal("wrong validator address")
	}
	if s.DelegatorAddress != "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9" {
		t.Fatal("wrong delegator address")
	}
	if s.Amount.Cmp(new(big.Int).SetInt64(20000)) != 0 {
		t.Fatal("wrong amount")
	}
}
