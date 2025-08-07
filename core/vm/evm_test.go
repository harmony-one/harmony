package vm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/params"
)

// this test is here so we can cover the input = epoch.bytes() line as well
func TestEpochPrecompile(t *testing.T) {
	targetEpoch := big.NewInt(1)
	evm := NewEVM(BlockContext{
		EpochNumber: targetEpoch,
	}, TxContext{}, nil, params.TestChainConfig, Config{})
	input := []byte{}
	precompileAddr := common.BytesToAddress([]byte{251})
	contract := Contract{
		CodeAddr: &precompileAddr,
		Gas:      GasQuickStep,
	}
	result, _, err := RunPrecompiledContract(&epoch{}, evm, &contract, input, GasQuickStep, true)
	if err != nil {
		t.Fatalf("Got error%v\n", err)
	}
	resultingEpoch := new(big.Int).SetBytes(result)
	if resultingEpoch.Cmp(targetEpoch) != 0 {
		t.Errorf("Epoch did not match, expected %d, got %d", targetEpoch.Int64(), resultingEpoch.Int64())
	}
}
