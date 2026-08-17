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

// crossShardDirectCallEnv builds an EVM whose transfer rules mirror the chain's,
// with the given balance already held at the cross-shard precompile address.
func crossShardDirectCallEnv(t *testing.T, cfg *params.ChainConfig, precompileBalance *big.Int) (*EVM, *state.DB, common.Address) {
	t.Helper()
	statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	require.NoError(t, err)

	precompileAddr := common.BytesToAddress([]byte{249})
	statedb.AddBalance(precompileAddr, precompileBalance)

	env := NewEVM(BlockContext{
		EpochNumber: big.NewInt(1),
		NumShards:   2,
		ShardID:     0,
		CanTransfer: func(db StateDB, addr common.Address, amount *big.Int) bool {
			return db.GetBalance(addr).Cmp(amount) >= 0
		},
		Transfer: func(db StateDB, sender, recipient common.Address, amount *big.Int, txType types.TransactionType) {
			if txType == types.SameShardTx || txType == types.SubtractionOnly {
				db.SubBalance(sender, amount)
			}
			if txType == types.SameShardTx {
				db.AddBalance(recipient, amount)
			}
		},
		IsValidator: func(_ StateDB, _ common.Address) bool { return false },
	}, TxContext{}, statedb, cfg, Config{})
	return env, statedb, precompileAddr
}

// TestCrossShardXferRejectsIndirectCall checks that the cross-shard precompile
// is only usable through a plain CALL. CALLCODE and DELEGATECALL hand it a value
// taken from the calling frame without moving that value to the precompile, so
// the transfer it performs would come out of whatever balance happens to sit at
// the precompile address rather than out of the caller's.
func TestCrossShardXferRejectsIndirectCall(t *testing.T) {
	value := new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18))
	input := CrossShardXferPrecompileTests[0].input
	callerAddr := common.HexToAddress("0xCA11E4")

	strict := *params.AllProtocolChanges
	strict.StrictStateValidationEpoch = big.NewInt(0)

	env, statedb, precompileAddr := crossShardDirectCallEnv(t, &strict, value)
	// The frame shape evm.CallCode builds when the target is a precompile.
	contract := NewContract(AccountRef(callerAddr), AccountRef(precompileAddr), value, 100000)

	_, _, err := RunPrecompiledContract(
		&crossShardXferPrecompile{}, env, contract, input, 100000, false, false,
	)
	require.ErrorIs(t, err, ErrPrecompileRequiresDirectCall)
	require.Nil(t, env.CXReceipt, "no receipt should be produced for an indirect call")
	require.Equal(t, value, statedb.GetBalance(precompileAddr),
		"the precompile balance must be untouched")

	// A plain CALL, where the frame has moved the value in, still works.
	env2, _, precompileAddr2 := crossShardDirectCallEnv(t, &strict, value)
	contract2 := NewContract(AccountRef(callerAddr), AccountRef(precompileAddr2), value, 100000)
	_, _, err = RunPrecompiledContract(
		&crossShardXferPrecompile{}, env2, contract2, input, 100000, false, true,
	)
	require.NoError(t, err)
	require.NotNil(t, env2.CXReceipt)
}

// TestCrossShardXferIndirectCallBeforeFork records the behaviour left in place
// for blocks before the fork.
func TestCrossShardXferIndirectCallBeforeFork(t *testing.T) {
	value := new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18))
	callerAddr := common.HexToAddress("0xCA11E4")

	legacy := *params.AllProtocolChanges
	legacy.StrictStateValidationEpoch = params.EpochTBD

	env, _, precompileAddr := crossShardDirectCallEnv(t, &legacy, value)
	contract := NewContract(AccountRef(callerAddr), AccountRef(precompileAddr), value, 100000)

	_, _, err := RunPrecompiledContract(
		&crossShardXferPrecompile{}, env, contract,
		CrossShardXferPrecompileTests[0].input, 100000, false, false,
	)
	require.NoError(t, err)
	require.NotNil(t, env.CXReceipt)
}
