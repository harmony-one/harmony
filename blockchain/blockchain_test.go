package blockchain

import (
	"testing"

	"github.com/simple-rules/harmony-benchmark/crypto/pki"
)

var (
	PriIntOne        = 111
	PriIntTwo        = 2
	PriIntThree      = 3
	PriIntFour       = 4
	PriKeyOne        = pki.GetPrivateKeyScalarFromInt(PriIntOne)
	PriKeyTwo        = pki.GetPrivateKeyScalarFromInt(PriIntTwo)
	PriKeyThree      = pki.GetPrivateKeyScalarFromInt(PriIntThree)
	PriKeyFour       = pki.GetPrivateKeyScalarFromInt(PriIntFour)
	TestAddressOne   = pki.GetAddressFromInt(PriIntOne)
	TestAddressTwo   = pki.GetAddressFromInt(PriIntTwo)
	TestAddressThree = pki.GetAddressFromInt(PriIntThree)
	TestAddressFour  = pki.GetAddressFromInt(PriIntFour)
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
	utxoPool := CreateUTXOPoolFromGenesisBlock(bc.Blocks[0])

	if !bc.AddNewUserTransfer(utxoPool, PriKeyOne, TestAddressOne, TestAddressThree, 3, 0) {
		t.Error("Failed to add new transfer to alok.")
	}
	if !bc.AddNewUserTransfer(utxoPool, PriKeyOne, TestAddressOne, TestAddressTwo, 3, 0) {
		t.Error("Failed to add new transfer to rj.")
	}

	if bc.AddNewUserTransfer(utxoPool, PriKeyOne, TestAddressOne, TestAddressFour, 100, 0) {
		t.Error("minh should not have enough fun to make the transfer.")
	}
}

func TestVerifyNewBlock(t *testing.T) {
	bc := CreateBlockchain(TestAddressOne, 0)
	utxoPool := CreateUTXOPoolFromGenesisBlock(bc.Blocks[0])

	bc.AddNewUserTransfer(utxoPool, PriKeyOne, TestAddressOne, TestAddressThree, 3, 0)
	bc.AddNewUserTransfer(utxoPool, PriKeyOne, TestAddressOne, TestAddressTwo, 10, 0)

	tx := bc.NewUTXOTransaction(PriKeyOne, TestAddressOne, TestAddressFour, 10, 0)
	if tx == nil {
		t.Error("failed to create a new transaction.")
	}
	newBlock := NewBlock([]*Transaction{tx}, bc.Blocks[len(bc.Blocks)-1].Hash, 0)

	if !bc.VerifyNewBlockAndUpdate(utxoPool, newBlock) {
		t.Error("failed to add a new valid block.")
	}
}
