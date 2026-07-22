package types

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/internal/params"
)

func TestExtraGasForStakingDirective(t *testing.T) {
	delegator := common.HexToAddress("0x1")
	validator := common.HexToAddress("0x2")

	batchDelegate := BatchDelegate{
		DelegatorAddress: delegator,
		Delegations: []DelegationAction{
			{ValidatorAddress: validator, Amount: big.NewInt(1000)},
			{ValidatorAddress: validator, Amount: big.NewInt(2000)},
		},
	}
	data, err := rlp.EncodeToBytes(batchDelegate)
	if err != nil {
		t.Fatal(err)
	}
	gas, err := ExtraGasForStakingDirective(DirectiveBatchDelegate, data)
	if err != nil {
		t.Fatal(err)
	}
	want := 2 * params.TxGasPerBatchStakingAction
	if gas != want {
		t.Fatalf("batch delegate gas = %d, want %d", gas, want)
	}

	batchUndelegate := BatchUndelegate{
		DelegatorAddress: delegator,
		Undelegations: []UndelegationAction{
			{ValidatorAddress: validator, Amount: big.NewInt(1000)},
		},
	}
	data, err = rlp.EncodeToBytes(batchUndelegate)
	if err != nil {
		t.Fatal(err)
	}
	gas, err = ExtraGasForStakingDirective(DirectiveBatchUndelegate, data)
	if err != nil {
		t.Fatal(err)
	}
	if gas != params.TxGasPerBatchStakingAction {
		t.Fatalf("batch undelegate gas = %d, want %d", gas, params.TxGasPerBatchStakingAction)
	}

	gas, err = ExtraGasForStakingDirective(DirectiveUndelegateAll, nil)
	if err != nil {
		t.Fatal(err)
	}
	if gas != params.TxGasUndelegateAll {
		t.Fatalf("undelegate all gas = %d, want %d", gas, params.TxGasUndelegateAll)
	}

	tooLarge := BatchDelegate{
		DelegatorAddress: delegator,
		Delegations:      make([]DelegationAction, MaxBatchStakingActions+1),
	}
	for i := range tooLarge.Delegations {
		tooLarge.Delegations[i] = DelegationAction{
			ValidatorAddress: validator,
			Amount:           big.NewInt(1),
		}
	}
	data, err = rlp.EncodeToBytes(tooLarge)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ExtraGasForStakingDirective(DirectiveBatchDelegate, data)
	if err != ErrBatchTooLarge {
		t.Fatalf("expected ErrBatchTooLarge, got %v", err)
	}
}
