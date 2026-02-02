package vm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	"github.com/holiman/uint256"
)

const testValue = 5

// tests opExtCodeSize
func TestOpExtCodeSize(t *testing.T) {
	interpreter := func(v bool) *EVMInterpreter {
		return &EVMInterpreter{
			evm: &EVM{
				Context: BlockContext{
					ShardID: shard.BeaconChainShardID,
				},
				StateDB: &testdb{},
				chainRules: params.Rules{
					IsValidatorCodeFix: v,
				},
			},
		}
	}
	// test isValidatorCodeFix
	t.Run("isValidatorCodeFix-activated", func(t *testing.T) {
		stack := newstack()
		stack.push(uint256.NewInt(testValue))
		opExtCodeSize(nil, interpreter(true), &ScopeContext{Stack: stack})
		if stack.len() != 1 {
			t.Fatalf("expected stack length 1, got %d", stack.len())
		}
		res := stack.pop()
		if !res.IsZero() {
			t.Fatalf("expected 0, got %s", res.String())
		}
	})
	t.Run("isValidatorCodeFix-deactivated", func(t *testing.T) {
		stack := newstack()
		stack.push(uint256.NewInt(testValue))
		opExtCodeSize(nil, interpreter(false), &ScopeContext{Stack: stack})
		if stack.len() != 1 {
			t.Fatalf("expected stack length 1, got %d", stack.len())
		}
		res := stack.pop()
		if res.IsZero() {
			t.Fatalf("expected %d, got %s", testValue, res.String())
		}
	})

}

type testdb struct {
	state.DB
}

func (d testdb) IsValidator(address common.Address) bool {
	return true
}

// getcodesize
func (d testdb) GetCodeSize(address common.Address) int {
	return testValue
}
