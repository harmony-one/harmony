package hmy

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
)

// OverrideAccount specifies the fields of an account to override during the execution
// of a message call.
// The fields `state` and `stateDiff` cannot be specified simultaneously. If `state` is set,
// the message execution will use only the data provided in the given state. Conversely,
// if `stateDiff` is set, all the diffs will be applied first, and then the message call
// will be executed.
type OverrideAccount struct {
	Nonce            *hexutil.Uint64              `json:"nonce"`
	Code             *hexutil.Bytes               `json:"code"`
	Balance          **hexutil.Big                `json:"balance"`
	State            *map[common.Hash]common.Hash `json:"state"`
	StateDiff        *map[common.Hash]common.Hash `json:"stateDiff"`
	MovePrecompileTo *common.Address              `json:"movePrecompileTo"`
}

// StateOverride is the collection of overriden accounts.
type StateOverrides map[common.Address]OverrideAccount

func (s *StateOverrides) has(address common.Address) bool {
	_, ok := (*s)[address]
	return ok
}

// Apply overrides the fields of specified accounts into the given state.
func (s *StateOverrides) Apply(state *state.DB, precompiles map[common.Address]vm.PrecompiledContract) error {
	if s == nil {
		return nil
	}

	// track desintation of precompiles that were moved
	tracked := make(map[common.Address]struct{})
	for addr, account := range *s {
		// if a precompile was moved to this address already, it cannot be overridden
		if _, ok := tracked[addr]; ok {
			return fmt.Errorf("account %s has already been overridden by a precompile", addr.Hex())
		}

		p, isPrecompile := precompiles[addr]
		// if the target adddress is another precompile, the target code is lost for this session
		if account.MovePrecompileTo != nil {
			if !isPrecompile {
				return fmt.Errorf("account %s is not a precompile", addr.Hex())
			}

			// cannot move a precompile to an address that has been overriden already
			if s.has(*account.MovePrecompileTo) {
				return fmt.Errorf("account %s has already been overridden", account.MovePrecompileTo.Hex())
			}

			precompiles[*account.MovePrecompileTo] = p
			tracked[*account.MovePrecompileTo] = struct{}{}
		}

		// if the address is a precompile and has passed the MovePrecompileTo check, remove it as it has been moved
		if isPrecompile {
			delete(precompiles, addr)
		}

		// nonce
		if account.Nonce != nil {
			state.SetNonce(addr, uint64(*account.Nonce))
		}

		// account code
		if account.Code != nil {
			state.SetCode(addr, *account.Code, false)
		}

		// account balance
		if account.Balance != nil {
			state.SetBalance(addr, (*big.Int)(*account.Balance))
		}

		if account.State != nil && account.StateDiff != nil {
			return fmt.Errorf("account %s has both 'state' and 'stateDiff'", addr.Hex())
		}

		// replace entire state if caller requires
		if account.State != nil {
			state.SetStorage(addr, *account.State)
		}

		// apply state diff into specified accounts
		if account.StateDiff != nil {
			for k, v := range *account.StateDiff {
				state.SetState(addr, k, v)
			}
		}
	}

	// finalize is normally performed between transactions
	// by using finalize, the overrides are semantically behaving as if they were created in a transaction prior to trace call
	state.Finalise(false)

	return nil
}

// BlockOverrides is a set of header fields to override.
type BlockOverrides struct {
	Number        *hexutil.Big
	Difficulty    *hexutil.Big // no-op
	Time          *hexutil.Uint64
	GasLimit      *hexutil.Uint64
	FeeRecipient  *common.Address
	PrevRandao    *common.Hash // no-op
	BaseFeePerGas *hexutil.Big // EIP-1559 (not implemented)
	BlobBaseFee   *hexutil.Big // EIP-4844 (not implemented)
}

// apply overrides the given header fields into the given block context
// difficulty & random not part of vm.Context
func (b *BlockOverrides) Apply(blockCtx *vm.Context) {
	if b == nil {
		return
	}

	if b.Number != nil {
		blockCtx.BlockNumber = b.Number.ToInt()
	}
	if b.Time != nil {
		blockCtx.Time = big.NewInt(int64(*b.Time))
	}
	if b.GasLimit != nil {
		blockCtx.GasLimit = uint64(*b.GasLimit)
	}
	if b.FeeRecipient != nil {
		blockCtx.Coinbase = *b.FeeRecipient
	}
}
