package blockchain

import (
	"testing"
)

func TestCreateBlockchain(t *testing.T) {
	if CreateBlockchain("minh") == nil {
		t.Errorf("failed to create a blockchain")
	}
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

func TestAddNewTransferAmount(t *testing.T) {
	bc := CreateBlockchain("minh")

	bc = bc.AddNewTransferAmount("minh", "alok", 3)

	if bc == nil {
		t.Error("Failed to add new transfer to alok")
	}

	bc = bc.AddNewTransferAmount("minh", "rj", 100)

	if bc == nil {
		t.Error("Failed to add new transfer to rj")
	}

	bc = bc.AddNewTransferAmount("minh", "stephen", DefaultCoinbaseValue-102)

	if bc != nil {
		t.Error("minh should not have enough fun to make the transfer")
	}
}

func TestVerifyNewBlock(t *testing.T) {
	bc := CreateBlockchain("minh")
	bc = bc.AddNewTransferAmount("minh", "alok", 3)
	bc = bc.AddNewTransferAmount("minh", "rj", 100)

	tx := bc.NewUTXOTransaction("minh", "mark", 10)
	if tx == nil {
		t.Error("failed to create a new transaction.")
	}
	newBlock := NewBlock([]*Transaction{tx}, bc.blocks[len(bc.blocks)-1].Hash)

	if !bc.VerifyNewBlock(newBlock) {
		t.Error("failed to add a new valid block.")
	}
}
