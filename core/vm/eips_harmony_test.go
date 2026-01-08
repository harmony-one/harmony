package vm

import (
	"math/big"
	"testing"

	"github.com/harmony-one/harmony/internal/params"
	"github.com/stretchr/testify/require"
)

func TestOpChainID(t *testing.T) {
	const ethCompatibleChainID = 12345
	t.Run("disabled-chainid-fix", func(t *testing.T) {
		v := EVMInterpreter{
			evm: &EVM{
				Context: BlockContext{
					EpochNumber: big.NewInt(100),
				},
				chainConfig: &params.ChainConfig{
					ChainIdFixEpoch:      big.NewInt(1323),
					EthCompatibleChainID: big.NewInt(ethCompatibleChainID),
					ChainID:              big.NewInt(1),
				},
			},
		}

		stack := newstack()
		_, err := opChainID(nil, &v, &ScopeContext{Stack: stack})
		if err != nil {
			t.Fatalf("opChainID error: %v", err)
		}
		rs := stack.pop()
		require.Equal(t, rs.Uint64(), uint64(1))
	})

	t.Run("enabled-chainid-fix", func(t *testing.T) {
		v := EVMInterpreter{
			evm: &EVM{
				Context: BlockContext{
					EpochNumber: big.NewInt(1325),
				},
				chainConfig: &params.ChainConfig{
					ChainIdFixEpoch:      big.NewInt(1323),
					EthCompatibleChainID: big.NewInt(ethCompatibleChainID),
					ChainID:              big.NewInt(1),
				},
			},
		}

		stack := newstack()
		_, err := opChainID(nil, &v, &ScopeContext{Stack: stack})
		if err != nil {
			t.Fatalf("opChainID error: %v", err)
		}
		rs := stack.pop()
		require.Equal(t, rs.Uint64(), uint64(ethCompatibleChainID))
	})
}
