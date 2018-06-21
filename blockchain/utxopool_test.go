package blockchain

import (
	"fmt"
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

func TestDeleteOneBalanceItem(t *testing.T) {
	bc := CreateBlockchain("minh")
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	bc.AddNewUserTransfer(utxoPool, "minh", "alok", 3)
	fmt.Println("first\n", utxoPool.String())
	bc.AddNewUserTransfer(utxoPool, "alok", "rj", 3)
	fmt.Println("second\n", utxoPool.String())

	fmt.Println(utxoPool.String())
	if _, ok := utxoPool.UtxoMap["alok"]; ok {
		t.Errorf("alok should not be contained in the balance map")
	}
}
