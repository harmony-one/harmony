package vm

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	bls12381 "github.com/harmony-one/harmony/crypto/bls/bls12381"
	"github.com/harmony-one/harmony/internal/params"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
	"github.com/stretchr/testify/require"
)

type writeCapablePrecompileTest struct {
	input, expected []byte
	name            string
	expectedError   error
	value           *big.Int
}

func CollectRewardsFn() CollectRewardsFunc {
	return func(db StateDB, rosettaTracer RosettaTracer, collectRewards *stakingTypes.CollectRewards) error {
		return nil
	}
}

func DelegateFn() DelegateFunc {
	return func(db StateDB, rosettaTracer RosettaTracer, delegate *stakingTypes.Delegate) error {
		return nil
	}
}

func UndelegateFn() UndelegateFunc {
	return func(db StateDB, rosettaTracer RosettaTracer, undelegate *stakingTypes.Undelegate) error {
		return nil
	}
}

func CreateValidatorFn() CreateValidatorFunc {
	return func(db StateDB, rosettaTracer RosettaTracer, createValidator *stakingTypes.CreateValidator) error {
		return nil
	}
}

func EditValidatorFn() EditValidatorFunc {
	return func(db StateDB, rosettaTracer RosettaTracer, editValidator *stakingTypes.EditValidator) error {
		return nil
	}
}

//func MigrateDelegationsFn() MigrateDelegationsFunc {
//	return func(db StateDB, migrationMsg *stakingTypes.MigrationMsg) ([]interface{}, error) {
//		return nil, nil
//	}
//}

func CalculateMigrationGasFn() CalculateMigrationGasFunc {
	return func(db StateDB, migrationMsg *stakingTypes.MigrationMsg, homestead bool, istanbul bool) (uint64, error) {
		return 0, nil
	}
}

func testWriteCapablePrecompile(test writeCapablePrecompileTest, t *testing.T, env *EVM, p WriteCapablePrecompiledContract) {
	t.Run(test.name, func(t *testing.T) {
		contract := NewContract(AccountRef(common.HexToAddress("1337")), AccountRef(common.HexToAddress("1338")), test.value, 0)
		gas, err := p.RequiredGas(env, contract, test.input)
		if err != nil {
			t.Error(err)
		}
		contract.Gas = gas
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
			if !bytes.Equal(res, test.expected) {
				t.Errorf("Expected %v, got %v", test.expected, res)
			}
		}
	})
}

func testStakingPrecompile(test writeCapablePrecompileTest, t *testing.T) {
	var env = NewEVM(Context{CollectRewards: CollectRewardsFn(),
		Delegate:        DelegateFn(),
		Undelegate:      UndelegateFn(),
		CreateValidator: CreateValidatorFn(),
		EditValidator:   EditValidatorFn(),
		ShardID:         0,
		//MigrateDelegations:    MigrateDelegationsFn(),
		CalculateMigrationGas: CalculateMigrationGasFn(),
	}, nil, params.TestChainConfig, Config{})
	p := &stakingPrecompile{}
	testWriteCapablePrecompile(test, t, env, p)
}

var StakingPrecompileTests = []writeCapablePrecompileTest{
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
		expected: nil,
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
		input:    []byte{81, 11, 17, 187, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0},
		expected: nil,
		name:     "delegateSuccess",
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
		input:    []byte{189, 168, 192, 233, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0},
		expected: nil,
		name:     "undelegateSuccess",
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
	//	input:    []byte{42, 5, 187, 113, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56},
	//	expected: nil,
	//	name:     "migrationSuccess",
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

func TestStakingPrecompiles(t *testing.T) {
	for _, test := range StakingPrecompileTests {
		testStakingPrecompile(test, t)
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

func transfer(db StateDB, sender, recipient common.Address, amount *big.Int, txType types.TransactionType) {
}

func testCrossShardXferPrecompile(test writeCapablePrecompileTest, t *testing.T) {
	// this EVM needs stateDB, Transfer, and NumShards
	var db ethdb.Database
	var err error
	defer func() {
		if db != nil {
			db.Close()
		}
	}()
	if db, err = rawdb.NewLevelDBDatabase("/tmp/harmony_shard_0", 256, 1024, "", false); err != nil {
		db = nil
		t.Fatalf("Could not initialize db %s", err)
	}
	stateCache := state.NewDatabase(db)
	state, err := state.New(common.Hash{}, stateCache, nil)
	if err != nil {
		t.Fatalf("Error while initializing state %s", err)
	}
	var env = NewEVM(Context{
		NumShards: 2,
		Transfer:  transfer,
		CanTransfer: func(_ StateDB, _ common.Address, _ *big.Int) bool {
			return true
		},
	}, state, params.TestChainConfig, Config{})
	p := &crossShardXferPrecompile{}
	testWriteCapablePrecompile(test, t, env, p)
}

func TestCrossShardXferPrecompile(t *testing.T) {
	for _, test := range CrossShardXferPrecompileTests {
		testCrossShardXferPrecompile(test, t)
	}
}

var CrossShardXferPrecompileTests = []writeCapablePrecompileTest{
	{
		input:    []byte{40, 72, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 165, 36, 21, 19, 218, 159, 68, 99, 241, 212, 135, 75, 84, 141, 251, 172, 41, 217, 31, 52, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		expected: nil,
		name:     "crossShardSuccess",
		value:    new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)),
	},
	{
		input:         []byte{40, 72, 8, 1},
		expectedError: errors.New("no method with id: 0x28480801"),
		name:          "crossShardMethodSigFail",
	},
	{
		input:         []byte{40, 72, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 165, 36, 21, 19, 218, 159, 68, 99, 241, 212, 135, 75, 84, 141, 251, 172, 41, 217, 31, 52, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		expectedError: errors.New("abi: cannot marshal in to go type: length insufficient 95 require 96"),
		name:          "crossShardMalformedInputFail",
	},
	{
		input:         []byte{40, 72, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 165, 36, 21, 19, 218, 159, 68, 99, 241, 212, 135, 75, 84, 141, 251, 172, 41, 217, 31, 52, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 6},
		expectedError: errors.New("toShardId out of bounds"),
		name:          "crossShardOutOfBoundsFail",
	},
	{
		input:         []byte{40, 72, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 165, 36, 21, 19, 218, 159, 68, 99, 241, 212, 135, 75, 84, 141, 251, 172, 41, 217, 31, 52, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		expectedError: errors.New("from and to shard id can't be equal"),
		name:          "crossShardSameShardFail",
	},
	{
		input:         []byte{40, 72, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 107, 199, 94, 45, 99, 16, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 165, 36, 21, 19, 218, 159, 68, 99, 241, 212, 135, 75, 84, 141, 251, 172, 41, 217, 31, 52, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		expectedError: errors.New("argument value and msg.value not equal"),
		name:          "crossShardDiffValueFail",
		value:         new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18)),
	},
}

// Test vectors for BLS12-381 operations
var (
	ValidEncodedG1Point = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 18, 197, 15, 52, 2, 230, 214, 179, 99, 213, 233, 80, 214, 247, 166, 184, 83, 87, 242, 190, 135, 129, 67, 197, 157, 114, 124, 161, 35, 173, 93, 135, 49, 73, 42, 83, 0, 60, 129, 150, 149, 214, 52, 10, 117, 68, 135, 143, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 15, 206, 242, 175, 75, 198, 157, 115, 27, 63, 63, 34, 57, 175, 95, 74, 248, 26, 179, 59, 252, 201, 250, 120, 225, 15, 77, 203, 26, 211, 8, 16, 160, 246, 232, 196, 42, 38, 152, 99, 214, 174, 143, 82, 90, 255, 151, 254}
	ValidEncodedG2Point = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 15, 227, 3, 244, 236, 28, 181, 28, 251, 207, 78, 17, 140, 105, 6, 216, 247, 56, 249, 212, 35, 109, 11, 1, 119, 68, 64, 21, 215, 24, 78, 253, 6, 172, 84, 213, 174, 109, 95, 246, 19, 98, 9, 9, 215, 71, 244, 149, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 16, 85, 11, 104, 177, 210, 97, 15, 88, 15, 171, 96, 41, 96, 71, 7, 120, 130, 150, 237, 203, 126, 130, 233, 160, 61, 31, 248, 24, 211, 5, 96, 3, 106, 211, 56, 137, 91, 185, 241, 171, 203, 228, 27, 137, 31, 207, 159, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 25, 38, 102, 108, 232, 159, 63, 56, 38, 46, 113, 62, 26, 252, 35, 124, 208, 69, 154, 225, 128, 138, 46, 127, 192, 94, 240, 55, 90, 64, 193, 253, 75, 21, 139, 196, 221, 55, 14, 125, 151, 113, 93, 114, 2, 162, 200, 211, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 17, 194, 238, 196, 62, 7, 234, 74, 196, 238, 148, 8, 222, 65, 107, 152, 64, 242, 200, 187, 12, 95, 67, 72, 111, 81, 92, 254, 113, 194, 48, 181, 34, 161, 186, 154, 132, 55, 40, 30, 230, 3, 245, 107, 196, 63, 199, 13}
	ValidScalar         = new(bls12381.PointG1).Set(bls12381.NewG1().One())
	PairingResult       = []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
)

// Enum type for the bls type
type BlsType int

const (
	BLS_RANDOM_G1 BlsType = iota
	BLS_RANDOM_G2
	BLS_RANDOM_SCALAR
	BLS_VALID_G1
	BLS_VALID_G2
	BLS_VALID_SCALAR
	BLS_RANDOM_PAIRING
	BLS_G1_ONE
	BLS_G1_ZERO
	BLS_G2_ONE
	BLS_G2_ZERO
	BLS_SCALAR_ONE
	BLS_INVALID_G1
	BLS_INVALID_G2
)

// Enum type for the bls output type
type BlsOutputType int

const (
	BLS_OUTPUT_EXPECT_FAIL BlsOutputType = iota
	BLS_OUTPUT_FIXED
	BLS_OUTPUT_IS_FIRST_INPUT // output will be the first input
	BLS_OUTPUT_G1_ONE
	BLS_OUTPUT_G1_ZERO
	BLS_OUTPUT_G2_ONE
	BLS_OUTPUT_G2_ZERO
)

func BigIntTo32Bytes(n *big.Int) []byte {
	bytes := n.Bytes()
	if len(bytes) > 32 {
		panic("number too large for 32-byte array")
	}
	result := make([]byte, 32)
	copy(result[32-len(bytes):], bytes)
	return result
}

// generateBLS generates a bls point from precompiles
func generateBLS(t BlsType) []byte {
	switch t {
	case BLS_RANDOM_G1:
		g := bls12381.NewG1()
		x := g.CreateRandomG1()
		return g.EncodePoint(x)
	case BLS_RANDOM_G2:
		g := bls12381.NewG1()
		x := g.CreateRandomG1()
		return g.EncodePoint(x)
	case BLS_RANDOM_SCALAR:
		max := big.NewInt(1000)
		s := bls12381.CreateRandomScalar(max)
		return BigIntTo32Bytes(s)
	case BLS_RANDOM_PAIRING:
		return []byte{}
	case BLS_G1_ONE:
		g := bls12381.NewG1()
		x := g.One()
		return g.EncodePoint(x)
	case BLS_G1_ZERO:
		g := bls12381.NewG1()
		x := g.Zero()
		return g.EncodePoint(x)
	case BLS_G2_ONE:
		g := bls12381.NewG2()
		x := g.One()
		return g.EncodePoint(x)
	case BLS_G2_ZERO:
		g := bls12381.NewG2()
		x := g.Zero()
		return g.EncodePoint(x)
	case BLS_VALID_G1:
		return ValidEncodedG1Point
	case BLS_VALID_G2:
		return ValidEncodedG2Point
	case BLS_VALID_SCALAR:
		return bls12381.NewG1().EncodePoint(ValidScalar)
	case BLS_SCALAR_ONE:
		one := big.NewInt(1)
		return BigIntTo32Bytes(one)
	case BLS_INVALID_G1:
		g := bls12381.NewG1()
		x := g.CreateRandomG1()
		return g.ToBytes(x) // Not Encoded to make it invalid
	case BLS_INVALID_G2:
		g := bls12381.NewG2()
		x := g.CreateRandomG2()
		return g.ToBytes(x) // Not Encoded to make it invalid
	default:
		return []byte{}
	}
}

func compileInputs(input []BlsType) (fullInput [][]byte, merged []byte) {
	for _, t := range input {
		n := generateBLS(t)
		fullInput = append(fullInput, n)
		merged = append(merged, n...)
	}
	return
}

// compileOutputs compiles test outputs
func compileOutputs(t BlsOutputType, out []byte, inputs [][]byte) []byte {
	switch t {
	case BLS_OUTPUT_EXPECT_FAIL:
		return nil
	case BLS_OUTPUT_FIXED:
		return out
	case BLS_OUTPUT_IS_FIRST_INPUT:
		return inputs[0]
	case BLS_OUTPUT_G1_ONE:
		g := bls12381.NewG1()
		x := g.One()
		return g.EncodePoint(x)
	case BLS_OUTPUT_G1_ZERO:
		g := bls12381.NewG1()
		x := g.Zero()
		return g.EncodePoint(x)
	case BLS_OUTPUT_G2_ONE:
		g := bls12381.NewG2()
		x := g.One()
		return g.EncodePoint(x)
	case BLS_OUTPUT_G2_ZERO:
		g := bls12381.NewG2()
		x := g.Zero()
		return g.EncodePoint(x)
	default:
		return []byte{}
	}
}

// Test cases for all BLS12-381 operations
var bls12381TestCases = []struct {
	name              string
	operation         []byte
	input             []BlsType
	expectOutput      BlsOutputType
	expectFixedOutput []byte
	expectedGas       uint64
}{
	// G1Add tests
	{
		name:              "G1Add valid",
		operation:         blsG1AddSig,
		input:             []BlsType{BLS_G1_ONE, BLS_G1_ZERO},
		expectOutput:      BLS_OUTPUT_G1_ONE,
		expectFixedOutput: nil,
		expectedGas:       params.Bls12381G1AddGas,
	},
	{
		name:              "G1Add valid",
		operation:         blsG1AddSig,
		input:             []BlsType{BLS_RANDOM_G1, BLS_G1_ZERO},
		expectOutput:      BLS_OUTPUT_IS_FIRST_INPUT,
		expectFixedOutput: nil,
		expectedGas:       params.Bls12381G1AddGas,
	},
	{
		name:              "G1Add invalid point",
		operation:         blsG1AddSig,
		input:             []BlsType{BLS_INVALID_G1, BLS_INVALID_G1},
		expectOutput:      BLS_OUTPUT_EXPECT_FAIL,
		expectFixedOutput: nil,
		expectedGas:       params.Bls12381G1AddGas,
	},
	// G1MultiExp tests (simplified assumptions)
	{
		name:              "G1MultiExp valid single",
		operation:         blsG1MultiExpSig,
		input:             []BlsType{BLS_VALID_G1, BLS_SCALAR_ONE},
		expectOutput:      BLS_OUTPUT_IS_FIRST_INPUT,
		expectFixedOutput: nil,
		expectedGas:       params.Bls12381G1MulGas * params.Bls12381G1MultiExpDiscountTable[0] / 1000,
	},
	{
		name:              "G1MultiExp invalid point",
		operation:         blsG1MultiExpSig,
		input:             []BlsType{BLS_INVALID_G1, BLS_SCALAR_ONE},
		expectOutput:      BLS_OUTPUT_EXPECT_FAIL,
		expectFixedOutput: nil,
		expectedGas:       0,
	},
	// Pairing tests
	{
		name:              "Pairing valid single",
		operation:         blsPairingSig,
		input:             []BlsType{BLS_G1_ONE, BLS_G2_ONE},
		expectOutput:      BLS_OUTPUT_FIXED,
		expectFixedOutput: PairingResult,
		expectedGas:       params.Bls12381PairingBaseGas + params.Bls12381PairingPerPairGas,
	},
}

func printByteArrayWithCommas(arr []byte) string {
	var sb strings.Builder
	for i, b := range arr {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprint(&sb, b)
	}
	return sb.String()
}

func TestEIP2537Precompile(t *testing.T) {
	precompile := &eip2537Precompile{}
	evm := NewEVM(Context{}, nil, params.TestChainConfig, Config{})

	for ti, tc := range bls12381TestCases {
		inputs, fullInput := compileInputs(tc.input)
		fullInput = append(tc.operation, fullInput...)
		// Test RunWriteCapable
		out, err := precompile.RunWriteCapable(evm, nil, fullInput)
		if tc.expectOutput != BLS_OUTPUT_EXPECT_FAIL {
			require.NoError(t, err, "test %d: unexpected error from RunWriteCapable", ti)
			require.NotNil(t, out, "test %d: expected non-nil output", ti)
			expectOut := compileOutputs(tc.expectOutput, tc.expectFixedOutput, inputs)
			require.Equal(t, expectOut, out, "test %d: output is not as expected", ti)
		} else {
			require.Error(t, err, "test %d: expected error from RunWriteCapable but got none", ti)
		}
		// Test RequiredGas
		gas, err := precompile.RequiredGas(evm, nil, fullInput)
		require.NoError(t, err, "test %d: unexpected error from RequiredGas", ti)
		require.Equal(t, tc.expectedGas, gas, "test %d: gas mismatch", ti)
	}
}
