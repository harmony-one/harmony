package staking

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/common/denominations"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

func TestValidateContractAddress(t *testing.T) {
	input := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55}
	args := map[string]interface{}{}
	expectedError := errors.New("Cannot parse address from <nil>")
	if _, err := ValidateContractAddress(common.BytesToAddress(input), args, "ValidatorAddress"); err != nil {
		if expectedError.Error() != err.Error() {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	} else {
		t.Errorf("Expected error %v, got result", expectedError)
	}
}

type parseTest struct {
	input         []byte
	name          string
	expectedError error
	expected      interface{}
}

var ParseStakeMsgTests = []parseTest{
	{
		input:         []byte{109, 107, 47, 120},
		expectedError: errors.New("no method with id: 0x6d6b2f78"),
		name:          "badStakingKind",
	},
	{
		input:         []byte{0, 0},
		expectedError: errors.New("data too short (2 bytes) for abi method lookup"),
		name:          "malformedInput",
	},
	{
		input:    []byte{109, 107, 47, 119, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55},
		expected: &stakingTypes.CollectRewards{DelegatorAddress: common.HexToAddress("0x1337")},
		name:     "collectRewardsSuccess",
	},
	{
		input:         []byte{109, 107, 47, 119, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56},
		expectedError: errors.New("[StakingPrecompile] Address mismatch, expected 0x0000000000000000000000000000000000001337 have 0x0000000000000000000000000000000000001338"),
		name:          "collectRewardsAddressMismatch",
	},
	{
		input:         []byte{109, 107, 47, 119, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19},
		expectedError: errors.New("abi: cannot marshal in to go type: length insufficient 31 require 32"),
		name:          "collectRewardsInvalidABI",
	},
	{
		input: []byte{81, 11, 17, 187, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0},
		expected: &stakingTypes.Delegate{
			DelegatorAddress: common.HexToAddress("0x1337"),
			ValidatorAddress: common.HexToAddress("0x1338"),
			Amount:           new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100)),
		},
		name: "delegateSuccess",
	},
	{
		input:         []byte{81, 11, 17, 187, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0},
		expectedError: errors.New("abi: cannot marshal in to go type: length insufficient 95 require 96"),
		name:          "delegateInvalidABI",
	},
	{
		input:         []byte{81, 11, 17, 187, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0},
		expectedError: errors.New("[StakingPrecompile] Address mismatch, expected 0x0000000000000000000000000000000000001337 have 0x0000000000000000000000000000000000001338"),
		name:          "delegateAddressMismatch",
	},

	{
		input: []byte{189, 168, 192, 233, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0},
		expected: &stakingTypes.Undelegate{
			DelegatorAddress: common.HexToAddress("0x1337"),
			ValidatorAddress: common.HexToAddress("0x1338"),
			Amount:           new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100)),
		},
		name: "undelegateSuccess",
	},
	{
		input:         []byte{189, 168, 192, 233, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0},
		expectedError: errors.New("abi: cannot marshal in to go type: length insufficient 95 require 96"),
		name:          "undelegateInvalidABI",
	},
	{
		input:         []byte{189, 168, 192, 233, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0},
		expectedError: errors.New("[StakingPrecompile] Address mismatch, expected 0x0000000000000000000000000000000000001337 have 0x0000000000000000000000000000000000001338"),
		name:          "undelegateAddressMismatch",
	},
	//{
	//	input:         []byte{42, 5, 187, 113},
	//	expectedError: errors.New("abi: attempting to unmarshall an empty string while arguments are expected"),
	//	name:          "yesMethodNoData",
	//},
	//{
	//	input:         []byte{0, 0},
	//	expectedError: errors.New("data too short (2 bytes) for abi method lookup"),
	//	name:          "malformedInput",
	//},
	//{
	//	input: []byte{42, 5, 187, 113, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56},
	//	expected: &stakingTypes.MigrationMsg{
	//		From: common.HexToAddress("0x1337"),
	//		To:   common.HexToAddress("0x1338"),
	//	},
	//	name: "migrationSuccess",
	//},
	//{
	//	input:         []byte{42, 5, 187, 113, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55},
	//	expectedError: errors.New("[StakingPrecompile] Address mismatch, expected 0x0000000000000000000000000000000000001337 have 0x0000000000000000000000000000000000001338"),
	//	name:          "migrationAddressMismatch",
	//},
	//{
	//	input:         []byte{42, 6, 187, 113, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55},
	//	expectedError: errors.New("no method with id: 0x2a06bb71"),
	//	name:          "migrationNoMatchingMethod",
	//},
	//{
	//	input:         []byte{42, 5, 187, 113, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19},
	//	expectedError: errors.New("abi: cannot marshal in to go type: length insufficient 63 require 64"),
	//	name:          "migrationAddressMismatch",
	//},
}

func testParseStakeMsg(test parseTest, t *testing.T) {
	t.Run(fmt.Sprintf("%s", test.name), func(t *testing.T) {
		if res, err := ParseStakeMsg(common.HexToAddress("1337"), test.input); err != nil {
			if test.expectedError != nil {
				if test.expectedError.Error() != err.Error() {
					t.Errorf("Expected error %v, got %v", test.expectedError, err)
				}
			} else {
				t.Error(err)
			}
		} else {
			if test.expectedError != nil {
				t.Errorf("Expected an error %v but instead got result %v", test.expectedError, res)
			}
			if test.expected != nil {
				if converted, ok := res.(*stakingTypes.Delegate); ok {
					convertedExp, ok := test.expected.(*stakingTypes.Delegate)
					if !ok {
						t.Errorf("Could not converted test.expected to *stakingTypes.Delegate")
					} else if !converted.Equals(*convertedExp) {
						t.Errorf("Expected %+v but got %+v", test.expected, converted)
					}
				} else if converted, ok := res.(*stakingTypes.Undelegate); ok {
					convertedExp, ok := test.expected.(*stakingTypes.Undelegate)
					if !ok {
						t.Errorf("Could not converted test.expected to *stakingTypes.Undelegate")
					} else if !converted.Equals(*convertedExp) {
						t.Errorf("Expected %+v but got %+v", test.expected, converted)
					}
				} else if converted, ok := res.(*stakingTypes.CollectRewards); ok {
					convertedExp, ok := test.expected.(*stakingTypes.CollectRewards)
					if !ok {
						t.Errorf("Could not converted test.expected to *stakingTypes.CollectRewards")
					} else if !converted.Equals(*convertedExp) {
						t.Errorf("Expected %+v but got %+v", test.expected, converted)
					}
				} else if converted, ok := res.(*stakingTypes.MigrationMsg); ok {
					convertedExp, ok := test.expected.(*stakingTypes.MigrationMsg)
					if !ok {
						t.Errorf("Could not converted test.expected to *stakingTypes.MigrationMsg")
					} else if !converted.Equals(*convertedExp) {
						t.Errorf("Expected %+v but got %+v", test.expected, converted)
					}
				} else {
					panic("Received unexpected result from ParseStakeMsg")
				}
			} else if res != nil {
				t.Errorf("Expected nil, got %v", res)
			}
		}
	})
}

func TestParseStakeMsgs(t *testing.T) {
	for _, test := range ParseStakeMsgTests {
		testParseStakeMsg(test, t)
	}
}
