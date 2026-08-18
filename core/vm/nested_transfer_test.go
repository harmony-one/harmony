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

// forwardValueCode returns code that calls `to` forwarding `value` wei.
func forwardValueCode(to common.Address, value byte) []byte {
	code := []byte{
		byte(PUSH1), 0x00, // retSize
		byte(PUSH1), 0x00, // retOffset
		byte(PUSH1), 0x00, // argSize
		byte(PUSH1), 0x00, // argOffset
		byte(PUSH1), value, // value
		byte(PUSH20),
	}
	code = append(code, to.Bytes()...)
	code = append(code,
		byte(GAS),
		byte(CALL),
		byte(POP),
		byte(STOP),
	)
	return code
}

// TestNestedTransferDuringCrossShardTx checks that a value transfer made from
// inside a cross-shard transaction still credits its recipient. Only the
// transaction's own transfer moves value to another shard; calls made while it
// executes stay on this one.
func TestNestedTransferDuringCrossShardTx(t *testing.T) {
	run := func(strict bool) *big.Int {
		statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
		require.NoError(t, err)

		contractAddr := common.HexToAddress("0xC0DE")
		innerAddr := common.HexToAddress("0xDEED")
		statedb.SetCode(contractAddr, forwardValueCode(innerAddr, 100), false)
		statedb.AddBalance(contractAddr, big.NewInt(1000))

		cfg := *params.AllProtocolChanges
		if strict {
			cfg.StrictStateValidationEpoch = big.NewInt(0)
		} else {
			cfg.StrictStateValidationEpoch = params.EpochTBD
		}

		env := NewEVM(BlockContext{
			EpochNumber: big.NewInt(1),
			// The transaction itself is cross-shard.
			TxType: types.SubtractionOnly,
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
		}, TxContext{}, statedb, &cfg, Config{})

		_, _, err = env.Call(
			AccountRef(common.HexToAddress("0xEEEE")), contractAddr, nil, 1000000, big.NewInt(0),
		)
		require.NoError(t, err)
		return statedb.GetBalance(innerAddr)
	}

	require.Equal(t, big.NewInt(100), run(true),
		"a call made while a cross-shard transaction runs should credit its recipient")
	// Records the earlier behaviour, where the value was taken but never credited.
	require.Equal(t, big.NewInt(0), run(false))
}
