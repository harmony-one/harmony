package blockchain

import (
	"testing"
)

var (
	TestAddressOne   = [20]byte{1}
	TestAddressTwo   = [20]byte{2}
	TestAddressThree = [20]byte{3}
	TestAddressFour  = [20]byte{4}
)

func TestCreateBlockchain(t *testing.T) {
	if bc := CreateBlockchain(TestAddressOne, 0); bc == nil {
		t.Errorf("failed to create a blockchain")
	}
}

func TestFindSpendableOutputs(t *testing.T) {
	requestAmount := 3
	bc := CreateBlockchain(TestAddressOne, 0)
	accumulated, unspentOutputs := bc.FindSpendableOutputs(TestAddressOne, requestAmount)
	if accumulated < DefaultCoinbaseValue {
		t.Error("Failed to find enough unspent ouptuts")
	}

	if len(unspentOutputs) <= 0 {
		t.Error("Failed to find enough unspent ouptuts")
	}
}

func TestFindUTXO(t *testing.T) {
	bc := CreateBlockchain(TestAddressOne, 0)
	utxo := bc.FindUTXO(TestAddressOne)

	total := 0
	for _, value := range utxo {
		total += value.Amount
		if value.Address != TestAddressOne {
			t.Error("FindUTXO failed.")
		}
	}

	if total != DefaultCoinbaseValue {
		t.Error("FindUTXO failed.")
	}
}

func TestAddNewUserTransfer(t *testing.T) {
	bc := CreateBlockchain(TestAddressOne, 0)
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	if !bc.AddNewUserTransfer(utxoPool, TestAddressOne, TestAddressThree, 3, 0) {
		t.Error("Failed to add new transfer to alok.")
	}

	if !bc.AddNewUserTransfer(utxoPool, TestAddressOne, TestAddressTwo, 100, 0) {
		t.Error("Failed to add new transfer to rj.")
	}

	if bc.AddNewUserTransfer(utxoPool, TestAddressOne, TestAddressFour, DefaultCoinbaseValue-102, 0) {
		t.Error("minh should not have enough fun to make the transfer.")
	}
}

func TestVerifyNewBlock(t *testing.T) {
	bc := CreateBlockchain(TestAddressOne, 0)
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	bc.AddNewUserTransfer(utxoPool, TestAddressOne, TestAddressThree, 3, 0)
	bc.AddNewUserTransfer(utxoPool, TestAddressOne, TestAddressTwo, 100, 0)

	tx := bc.NewUTXOTransaction(TestAddressOne, TestAddressFour, 10, 0)
	if tx == nil {
		t.Error("failed to create a new transaction.")
	}
	newBlock := NewBlock([]*Transaction{tx}, bc.Blocks[len(bc.Blocks)-1].Hash, 0)

	if !bc.VerifyNewBlockAndUpdate(utxoPool, newBlock) {
		t.Error("failed to add a new valid block.")
	}
}
