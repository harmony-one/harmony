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

func TestDeleteOneBalanceItem(t *testing.T) {
	bc := CreateBlockchain("minh")
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	bc.AddNewUserTransfer(utxoPool, "minh", "alok", 3)
	bc.AddNewUserTransfer(utxoPool, "alok", "rj", 3)

	if _, ok := utxoPool.UtxoMap["alok"]; ok {
		t.Errorf("alok should not be contained in the balance map")
	}
}

func TestCleanUp(t *testing.T) {
	var utxoPool UTXOPool
	utxoPool.UtxoMap = make(map[string]map[string]map[int]int)
	utxoPool.UtxoMap["minh"] = make(map[string]map[int]int)
	utxoPool.UtxoMap["rj"] = map[string]map[int]int{
		"abcd": {
			0: 1,
		},
	}
	utxoPool.CleanUp()
	if _, ok := utxoPool.UtxoMap["minh"]; ok {
		t.Errorf("minh should not be contained in the balance map")
	}
}
