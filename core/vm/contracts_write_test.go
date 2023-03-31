package vm

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
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
		NumShards: 4,
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
