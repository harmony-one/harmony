package vm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/stretchr/testify/require"
)

// TestValidatorAccountHasNoExecutableCode checks that a validator account is
// reached as an account without code from every call frame. Its code field holds
// the encoded validator wrapper rather than executable code, which Call and
// CallCode already account for.
func TestValidatorAccountHasNoExecutableCode(t *testing.T) {
	run := func(strict bool) (delegateErr error, staticErr error) {
		statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
		require.NoError(t, err)

		addr := common.HexToAddress("0x5741")
		// Stand in for an encoded validator wrapper: bytes that are not valid code.
		statedb.SetCode(addr, []byte{0xf9, 0x01, 0x02, 0x03}, false)

		cfg := *params.AllProtocolChanges
		if strict {
			cfg.StrictStateValidationEpoch = big.NewInt(0)
		} else {
			cfg.StrictStateValidationEpoch = params.EpochTBD
		}

		env := NewEVM(BlockContext{
			EpochNumber: big.NewInt(1),
			CanTransfer: func(_ StateDB, _ common.Address, _ *big.Int) bool { return true },
			Transfer: func(_ StateDB, _, _ common.Address, _ *big.Int, _ types.TransactionType) {
			},
			IsValidator: func(_ StateDB, a common.Address) bool { return a == addr },
		}, TxContext{}, statedb, &cfg, Config{})

		eoa := AccountRef(common.HexToAddress("0xEEEE"))
		// DELEGATECALL is only reachable from inside contract execution, so the
		// caller has to be a running frame rather than a bare account.
		callerFrame := NewContract(eoa, AccountRef(common.HexToAddress("0xCA11E4")), big.NewInt(0), 100000)
		_, _, delegateErr = env.DelegateCall(callerFrame, addr, nil, 100000)
		_, _, staticErr = env.StaticCall(eoa, addr, nil, 100000)
		return
	}

	dErr, sErr := run(true)
	require.NoError(t, dErr, "delegatecall to a validator account should behave as no code")
	require.NoError(t, sErr, "staticcall to a validator account should behave as no code")

	// Records the earlier behaviour: the wrapper bytes were run as code.
	dErrOld, sErrOld := run(false)
	require.Error(t, dErrOld)
	require.Error(t, sErrOld)
}
