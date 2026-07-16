package vm

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/accounts/abi"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/stretchr/testify/require"
)

func TestWrapWritePrecompileErrorBeforeFork(t *testing.T) {
	cfg := *params.TestChainConfig
	cfg.StakingV2Epoch = big.NewInt(100)
	evm := NewEVM(
		BlockContext{EpochNumber: big.NewInt(1)},
		TxContext{},
		nil,
		&cfg,
		Config{},
	)

	origErr := errors.New("insufficient balance to stake")
	output, gas, err := wrapWritePrecompileError(evm, &stakingPrecompile{}, nil, 42_000, origErr)
	require.Equal(t, origErr, err)
	require.Equal(t, uint64(42_000), gas)
	require.Nil(t, output)
}

func TestWrapWritePrecompileErrorAfterFork(t *testing.T) {
	evm := NewEVM(
		BlockContext{EpochNumber: big.NewInt(6)},
		TxContext{},
		nil,
		params.LocalnetChainConfig,
		Config{},
	)

	origErr := errors.New("insufficient balance to stake")
	output, gas, err := wrapWritePrecompileError(evm, &stakingPrecompile{}, nil, 42_000, origErr)
	require.ErrorIs(t, err, ErrExecutionReverted)
	require.Equal(t, uint64(42_000), gas)

	reason, unpackErr := abi.UnpackRevert(output)
	require.NoError(t, unpackErr)
	require.Equal(t, origErr.Error(), reason)
}

func TestWrapWritePrecompileErrorSkipsNonStaking(t *testing.T) {
	evm := NewEVM(
		BlockContext{EpochNumber: big.NewInt(6)},
		TxContext{},
		nil,
		params.LocalnetChainConfig,
		Config{},
	)

	origErr := errors.New("cross shard transfer failed")
	output, gas, err := wrapWritePrecompileError(evm, &crossShardXferPrecompile{}, nil, 42_000, origErr)
	require.Equal(t, origErr, err)
	require.Equal(t, uint64(42_000), gas)
	require.Nil(t, output)
}

func TestStakingPrecompileAddressMismatchRevertsAfterFork(t *testing.T) {
	env := NewEVM(BlockContext{
		CollectRewards:        CollectRewardsFn(),
		Delegate:              DelegateFn(),
		Undelegate:            UndelegateFn(),
		CreateValidator:       CreateValidatorFn(),
		EditValidator:         EditValidatorFn(),
		ShardID:               0,
		EpochNumber:           big.NewInt(6),
		CalculateMigrationGas: CalculateMigrationGasFn(),
	}, TxContext{}, nil, params.LocalnetChainConfig, Config{})

	input := []byte{
		109, 107, 47, 119, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 56,
	}
	contract := NewContract(
		AccountRef(common.HexToAddress("0x1337")),
		AccountRef(common.HexToAddress("0x1338")),
		nil,
		1_000_000,
	)
	p := &stakingPrecompile{}
	gas, err := p.RequiredGas(env, contract, input)
	require.NoError(t, err)
	contract.Gas = gas + 1_000

	output, remainingGas, err := RunPrecompiledContract(p, env, contract, input, gas+1_000, false)
	require.ErrorIs(t, err, ErrExecutionReverted)
	require.Equal(t, uint64(1_000), remainingGas)

	reason, unpackErr := abi.UnpackRevert(output)
	require.NoError(t, unpackErr)
	require.NotEmpty(t, reason)
}
