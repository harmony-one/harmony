package staking

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
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

func TestParseBigIntFromKey(t *testing.T) {
	args := map[string]interface{}{}
	expectedError := errors.New("Cannot parse BigInt from <nil>")
	if _, err := ParseBigIntFromKey(args, "PotentialBigInt"); err != nil {
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

type parseRoTest struct {
	input         []byte
	name          string
	expectedError error
	expected      *stakingTypes.ReadOnlyStakeMsg
}

var ParseRoStakeMsgTests = []parseRoTest{
	{
		input:         []byte{102, 54, 146, 228, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55},
		name:          "getBalanceAvailableForRedelegationSuccess",
		expectedError: nil,
		expected: &stakingTypes.ReadOnlyStakeMsg{
			DelegatorAddress: common.HexToAddress("0x1337"),
			What:             "BalanceAvailableForRedelegation",
		},
	},
	{
		input:         []byte{90, 112, 28, 167, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55},
		name:          "getBalanceDelegatedByDelegator",
		expectedError: nil,
		expected: &stakingTypes.ReadOnlyStakeMsg{
			DelegatorAddress: common.HexToAddress("0x1337"),
			What:             "BalanceDelegatedByDelegator",
		},
	},
	{
		input:         []byte{55, 162, 85, 128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56},
		name:          "getDelegationByDelegatorAndValidatorSuccess",
		expectedError: nil,
		expected: &stakingTypes.ReadOnlyStakeMsg{
			DelegatorAddress: common.HexToAddress("0x1337"),
			ValidatorAddress: common.HexToAddress("0x1338"),
			What:             "DelegationByDelegatorAndValidator",
		},
	},
	{
		input:         []byte{116, 78, 71, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56},
		name:          "getValidatorTotalDelegationSuccess",
		expectedError: nil,
		expected: &stakingTypes.ReadOnlyStakeMsg{
			ValidatorAddress: common.HexToAddress("0x1338"),
			What:             "ValidatorTotalDelegation",
		},
	},
	{
		input:         []byte{62, 128, 166, 177, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56},
		name:          "getValidatorMaxTotalDelegationSuccess",
		expectedError: nil,
		expected: &stakingTypes.ReadOnlyStakeMsg{
			ValidatorAddress: common.HexToAddress("0x1338"),
			What:             "ValidatorMaxTotalDelegation",
		},
	},
	{
		input:         []byte{163, 16, 98, 79, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56},
		name:          "getValidatorStatus",
		expectedError: nil,
		expected: &stakingTypes.ReadOnlyStakeMsg{
			ValidatorAddress: common.HexToAddress("0x1338"),
			What:             "ValidatorStatus",
		},
	},
	{
		input:         []byte{75, 21, 53, 214},
		name:          "getTotalStakingSnapshot",
		expectedError: nil,
		expected: &stakingTypes.ReadOnlyStakeMsg{
			What: "TotalStakingSnapshot",
		},
	},
	{
		input:         []byte{255, 69, 95, 178},
		name:          "getMedianRawStakeSnapshot",
		expectedError: nil,
		expected: &stakingTypes.ReadOnlyStakeMsg{
			What: "MedianRawStakeSnapshot",
		},
	},
	{
		input:         []byte{41, 202, 191, 238},
		name:          "NoMethodFailure",
		expectedError: errors.New("no method with id: 0x29cabfee"),
	},
	{
		input:         []byte{62, 128, 166, 177},
		name:          "MissingDataFailure",
		expectedError: errors.New("abi: attempting to unmarshall an empty string while arguments are expected"),
	},
	{
		input:         []byte{41, 202, 19},
		name:          "MissingBytesFailure",
		expectedError: errors.New("data too short (3 bytes) for abi method lookup"),
	},
}

func testParseRoStakeMsg(test parseRoTest, t *testing.T) {
	t.Run(test.name, func(t *testing.T) {
		if res, err := ParseReadOnlyStakeMsg(test.input); err != nil {
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
				if strings.Compare(test.expected.What, res.What) != 0 {
					t.Errorf(
						"Unequal value for what %s and %s",
						test.expected.What, res.What,
					)
				}
				if strings.Compare(test.expected.What, "DelegationByDelegatorAndValidator") == 0 {
					if !bytes.Equal(test.expected.DelegatorAddress[:], res.DelegatorAddress[:]) {
						t.Errorf(
							"Unequal value for delegator address %s and %s",
							test.expected.DelegatorAddress.Hex(),
							res.DelegatorAddress.Hex(),
						)
					}
					if !bytes.Equal(test.expected.ValidatorAddress[:], res.ValidatorAddress[:]) {
						t.Errorf(
							"Unequal value for validator address %s and %s",
							test.expected.ValidatorAddress.Hex(),
							res.ValidatorAddress.Hex(),
						)
					}
				} else if strings.Compare(test.expected.What, "ValidatorMaxTotalDelegation") == 0 ||
					strings.Compare(test.expected.What, "ValidatorTotalDelegation") == 0 ||
					strings.Compare(test.expected.What, "ValidatorStatus") == 0 ||
					strings.Compare(test.expected.What, "ValidatorCommissionRate") == 0 {
					if !bytes.Equal(test.expected.ValidatorAddress[:], res.ValidatorAddress[:]) {
						t.Errorf(
							"Unequal value for validator address %s and %s",
							test.expected.ValidatorAddress.Hex(),
							res.ValidatorAddress.Hex(),
						)
					}
				} else if strings.Compare(test.expected.What, "BalanceAvailableForRedelegation") == 0 ||
					strings.Compare(test.expected.What, "BalanceDelegatedByDelegator") == 0 {
					if !bytes.Equal(test.expected.DelegatorAddress[:], res.DelegatorAddress[:]) {
						t.Errorf(
							"Unequal value for delegator address %s and %s",
							test.expected.DelegatorAddress.Hex(),
							res.DelegatorAddress.Hex(),
						)
					}
				} else if strings.Compare(test.expected.What, "SlashingHeightFromBlockForValidator") == 0 {
					if !bytes.Equal(test.expected.ValidatorAddress[:], res.ValidatorAddress[:]) {
						t.Errorf(
							"Unequal value for validator address %s and %s",
							test.expected.ValidatorAddress.Hex(),
							res.ValidatorAddress.Hex(),
						)
					}
				} else if strings.Compare(test.expected.What, "TotalStakingSnapshot") == 0 ||
					strings.Compare(test.expected.What, "MedianRawStakeSnapshot") == 0 {
				} else {
					t.Fatalf("Got %s as what", test.expected.What)
				}
			}
		}
	})
}

func TestParseRoStakeMsgs(t *testing.T) {
	for _, test := range ParseRoStakeMsgTests {
		testParseRoStakeMsg(test, t)
	}
}
