package vm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/params"
)

func TestEpochPrecompile(t *testing.T) {
	targetEpoch := big.NewInt(1)
	evm := NewEVM(Context{
		EpochNumber: targetEpoch,
	}, nil, params.TestChainConfig, Config{})
	input := []byte{}
	precompileAddr := common.BytesToAddress([]byte{250})
	contract := Contract{
		CodeAddr: &precompileAddr,
		Gas:      GasQuickStep,
	}
	result, err := run(evm,
		&contract,
		input,
		true,
	)
	if err != nil {
		t.Fatalf("Got error%v\n", err)
	}
	resultingEpoch := new(big.Int).SetBytes(result)
	if resultingEpoch.Cmp(targetEpoch) != 0 {
		t.Error("Epoch did not match")
	}
}
