package blockchain

import (
	"testing"
)

func TestVerifyOneTransactionAndUpdate(t *testing.T) {
	bc := CreateBlockchain("minh")
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	bc.AddNewUserTransfer(utxoPool, "minh", "alok", 3)
	bc.AddNewUserTransfer(utxoPool, "minh", "rj", 100)

	tx := bc.NewUTXOTransaction("minh", "mark", 10)
	if tx == nil {
		t.Error("failed to create a new transaction.")
	}

	if !utxoPool.VerifyOneTransaction(tx) {
		t.Error("failed to verify a valid transaction.")
	}
	utxoPool.VerifyOneTransactionAndUpdate(tx)
}
