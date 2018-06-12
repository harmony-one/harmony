package blockchain

import (
	"testing"
)

func TestCreateBlockchain(t *testing.T) {
	CreateBlockchain("minh")
}

func TestFindSpendableOutputs(t *testing.T) {
	requestAmount := 3
	bc := CreateBlockchain("minh")
	accumulated, unspentOutputs := bc.FindSpendableOutputs("minh", requestAmount)
	if accumulated < DefaultCoinbaseValue {
		t.Error("Failed to find enough unspent ouptuts")
	}

	if len(unspentOutputs) <= 0 {
		t.Error("Failed to find enough unspent ouptuts")
	}
}

func TestFindUTXO(t *testing.T) {
	bc := CreateBlockchain("minh")
	utxo := bc.FindUTXO("minh")

	total := 0
	for _, value := range utxo {
		total += value.Value
		if value.Address != "minh" {
			t.Error("FindUTXO failed.")
		}
	}

	if total != DefaultCoinbaseValue {
		t.Error("FindUTXO failed.")
	}
}
