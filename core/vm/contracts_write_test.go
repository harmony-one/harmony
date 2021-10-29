package vm

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/params"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

type writeCapablePrecompileTest struct {
	input, expected []byte
	name            string
	expectedError   error
}

func CollectRewardsFn() CollectRewardsFunc {
	return func(db StateDB, collectRewards *stakingTypes.CollectRewards) error {
		return nil
	}
}

func DelegateFn() DelegateFunc {
	return func(db StateDB, delegate *stakingTypes.Delegate) error {
		return nil
	}
}

func UndelegateFn() UndelegateFunc {
	return func(db StateDB, undelegate *stakingTypes.Undelegate) error {
		return nil
	}
}

func CreateValidatorFn() CreateValidatorFunc {
	return func(db StateDB, createValidator *stakingTypes.CreateValidator) error {
		return nil
	}
}

func EditValidatorFn() EditValidatorFunc {
	return func(db StateDB, editValidator *stakingTypes.EditValidator) error {
		return nil
	}
}

func testWriteCapablePrecompile(test writeCapablePrecompileTest, t *testing.T) {
	var env = NewEVM(Context{CollectRewards: CollectRewardsFn(),
		Delegate:        DelegateFn(),
		Undelegate:      UndelegateFn(),
		CreateValidator: CreateValidatorFn(),
		EditValidator:   EditValidatorFn(),
		ShardID:         0,
	}, nil, params.TestChainConfig, Config{})
	// use required gas to avoid out of gas errors
	p := &stakingPrecompile{}
	t.Run(fmt.Sprintf("%s", test.name), func(t *testing.T) {
		gas, err := p.RequiredGas(env, test.input)
		if err != nil {
			if test.expectedError != nil {
				if test.expectedError.Error() != err.Error() {
					t.Errorf("Expected error %v, got %v", test.expectedError, err)
				}
			} else {
				t.Errorf("Expected pass but got error %v", err)
			}
			return
		}
		contract := NewContract(AccountRef(common.HexToAddress("1337")), AccountRef(common.HexToAddress("1338")), new(big.Int), gas)
		if res, err := RunWriteCapablePrecompiledContract(p, env, contract, test.input, false); err != nil {
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
			if bytes.Compare(res, test.expected) != 0 {
				t.Errorf("Expected %v, got %v", test.expected, res)
			}
		}
	})
}

func TestWriteCapablePrecompiles(t *testing.T) {
	for _, test := range WriteCapablePrecompileTests {
		testWriteCapablePrecompile(test, t)
	}
}

func TestWriteCapablePrecompilesReadOnly(t *testing.T) {
	p := &stakingPrecompile{}
	expectedError := errWriteProtection
	res, err := RunWriteCapablePrecompiledContract(p, nil, nil, []byte{}, true)
	if err != nil {
		if err.Error() != expectedError.Error() {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	} else {
		t.Errorf("Expected an error %v but instead got result %v", expectedError, res)
	}
}

var WriteCapablePrecompileTests = []writeCapablePrecompileTest{
	{
		input:         []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 36, 109, 107, 47, 120},
		expectedError: errors.New("bad staking kind"),
		name:          "badStakingKind",
	},
	{
		input:         []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		expectedError: errors.New("Input is malformed"),
		name:          "malformedInput",
	},
	{
		input:    []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 36, 109, 107, 47, 119, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55},
		expected: nil,
		name:     "collectRewardsSuccess",
	},
	{
		input:         []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 36, 109, 107, 47, 119, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56},
		expectedError: errors.New("[StakingPrecompile] Address mismatch, expected 0x0000000000000000000000000000000000001337 have 0x0000000000000000000000000000000000001338"),
		name:          "collectRewardsAddressMismatch",
	},
	{
		input:         []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 36, 109, 107, 47, 119, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19},
		expectedError: errors.New("abi: cannot marshal in to go type: length insufficient 31 require 32"),
		name:          "collectRewardsInvalidABI",
	},
	{
		input:    []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 81, 11, 17, 187, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0},
		expected: nil,
		name:     "delegateSuccess",
	},
	{
		input:         []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 81, 11, 17, 187, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0},
		expectedError: errors.New("abi: cannot marshal in to go type: length insufficient 95 require 96"),
		name:          "delegateInvalidABI",
	},
	{
		input:         []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 81, 11, 17, 187, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0},
		expectedError: errors.New("[StakingPrecompile] Address mismatch, expected 0x0000000000000000000000000000000000001337 have 0x0000000000000000000000000000000000001338"),
		name:          "delegateAddressMismatch",
	},

	{
		input:    []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 189, 168, 192, 233, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0},
		expected: nil,
		name:     "undelegateSuccess",
	},
	{
		input:         []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 189, 168, 192, 233, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0},
		expectedError: errors.New("abi: cannot marshal in to go type: length insufficient 95 require 96"),
		name:          "undelegateInvalidABI",
	},
	{
		input:         []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 189, 168, 192, 233, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0},
		expectedError: errors.New("[StakingPrecompile] Address mismatch, expected 0x0000000000000000000000000000000000001337 have 0x0000000000000000000000000000000000001338"),
		name:          "undelegateAddressMismatch",
	},
}
