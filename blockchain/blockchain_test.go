package blockchain

import (
	"testing"
)

func TestCreateBlockchain(t *testing.T) {
	if bc := CreateBlockchain("minh"); bc == nil {
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

func TestAddNewUserTransfer(t *testing.T) {
	bc := CreateBlockchain("minh")
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	if !bc.AddNewUserTransfer(utxoPool, "minh", "alok", 3) {
		t.Error("Failed to add new transfer to alok.")
	}

	if !bc.AddNewUserTransfer(utxoPool, "minh", "rj", 100) {
		t.Error("Failed to add new transfer to rj.")
	}

	if bc.AddNewUserTransfer(utxoPool, "minh", "stephen", DefaultCoinbaseValue-102) {
		t.Error("minh should not have enough fun to make the transfer.")
	}
}

func TestVerifyNewBlock(t *testing.T) {
	bc := CreateBlockchain("minh")
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	bc.AddNewUserTransfer(utxoPool, "minh", "alok", 3)
	bc.AddNewUserTransfer(utxoPool, "minh", "rj", 100)

	tx := bc.NewUTXOTransaction("minh", "mark", 10)
	if tx == nil {
		t.Error("failed to create a new transaction.")
	}
	newBlock := NewBlock([]*Transaction{tx}, bc.Blocks[len(bc.Blocks)-1].Hash)

	if !bc.VerifyNewBlockAndUpdate(utxoPool, newBlock) {
		t.Error("failed to add a new valid block.")
	}
}
