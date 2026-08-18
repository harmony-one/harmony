package vm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/stretchr/testify/require"
)

// delegateThenRevertInitCode returns constructor code that calls the staking
// precompile with the given payload and then reverts.
func delegateThenRevertInitCode(input []byte) []byte {
	code := make([]byte, 0, 256)
	for offset := 0; offset < len(input); offset += 32 {
		chunk := make([]byte, 32)
		copy(chunk, input[offset:])
		code = append(code, byte(PUSH32))
		code = append(code, chunk...)
		code = append(code, byte(PUSH1), byte(offset), byte(MSTORE))
	}
	code = append(code,
		byte(PUSH1), 0x00, // out size
		byte(PUSH1), 0x00, // out offset
		byte(PUSH1), byte(len(input)), // input size
		byte(PUSH1), 0x00, // input offset
		byte(PUSH1), 0x00, // value
		byte(PUSH1), 0xfc, // staking precompile
		byte(GAS), // forward all remaining gas
		byte(CALL),
		byte(POP),
		byte(PUSH1), 0x00,
		byte(PUSH1), 0x00,
		byte(REVERT),
	)
	return code
}

// delegateInput builds a Delegate payload naming the given delegator.
func delegateInput(delegator common.Address) []byte {
	in := []byte{0x51, 0x0b, 0x11, 0xbb} // Delegate(address,address,uint256)
	pad := func(b []byte) []byte {
		out := make([]byte, 32)
		copy(out[32-len(b):], b)
		return out
	}
	in = append(in, pad(delegator.Bytes())...)
	in = append(in, pad(common.HexToAddress("0x1338").Bytes())...)
	in = append(in, pad(new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)).Bytes())...)
	return in
}

// TestStakeMsgsDroppedOnRevert checks that stake messages recorded by the staking
// precompile do not outlive the frame that produced them. They are read after the
// transaction to index delegations, so a message left behind by a reverted frame
// would describe a delegation that is not in state.
func TestStakeMsgsDroppedOnRevert(t *testing.T) {
	run := func(strict bool) int {
		statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
		require.NoError(t, err)

		cfg := *params.AllProtocolChanges
		if strict {
			cfg.StrictStateValidationEpoch = big.NewInt(0)
		} else {
			cfg.StrictStateValidationEpoch = params.EpochTBD
		}

		env := NewEVM(BlockContext{
			EpochNumber:     big.NewInt(1),
			ShardID:         0,
			Delegate:        DelegateFn(),
			Undelegate:      UndelegateFn(),
			CollectRewards:  CollectRewardsFn(),
			CreateValidator: CreateValidatorFn(),
			EditValidator:   EditValidatorFn(),
			CanTransfer:     func(_ StateDB, _ common.Address, _ *big.Int) bool { return true },
			Transfer: func(_ StateDB, _, _ common.Address, _ *big.Int, _ types.TransactionType) {
			},
			IsValidator: func(_ StateDB, _ common.Address) bool { return false },
		}, TxContext{}, statedb, &cfg, Config{})

		caller := common.HexToAddress("0x1337")
		contractAddr := crypto.CreateAddress(caller, statedb.GetNonce(caller))

		_, _, _, err = env.Create(
			AccountRef(caller),
			delegateThenRevertInitCode(delegateInput(contractAddr)),
			10_000_000,
			big.NewInt(0),
		)
		require.ErrorIs(t, err, ErrExecutionReverted)
		return len(env.StakeMsgs)
	}

	require.Equal(t, 0, run(true), "a reverted frame must not leave stake messages behind")
	// Records the earlier behaviour so the change is visible.
	require.Equal(t, 1, run(false))
}
