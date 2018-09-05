package blockchain

import (
	"testing"
)

func TestVerifyOneTransactionAndUpdate(t *testing.T) {
	bc := CreateBlockchain(TestAddressOne, 0)
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	bc.AddNewUserTransfer(utxoPool, PriKeyOne, TestAddressOne, TestAddressThree, 3, 0)
	bc.AddNewUserTransfer(utxoPool, PriKeyOne, TestAddressOne, TestAddressTwo, 100, 0)

	tx := bc.NewUTXOTransaction(PriKeyOne, TestAddressOne, TestAddressFour, 10, 0)
	if tx == nil {
		t.Error("failed to create a new transaction.")
	}

	if err, _ := utxoPool.VerifyOneTransaction(tx, nil); err != nil {
		t.Error("failed to verify a valid transaction.")
	}
	utxoPool.VerifyOneTransactionAndUpdate(tx)
}

func TestVerifyOneTransactionFail(t *testing.T) {
	bc := CreateBlockchain(TestAddressOne, 0)
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	bc.AddNewUserTransfer(utxoPool, PriKeyOne, TestAddressOne, TestAddressThree, 3, 0)
	bc.AddNewUserTransfer(utxoPool, PriKeyOne, TestAddressOne, TestAddressTwo, 100, 0)

	tx := bc.NewUTXOTransaction(PriKeyOne, TestAddressOne, TestAddressFour, 10, 0)
	if tx == nil {
		t.Error("failed to create a new transaction.")
	}

	tx.TxInput = append(tx.TxInput, tx.TxInput[0])
	if err, _ := utxoPool.VerifyOneTransaction(tx, nil); err == nil {
		t.Error("Tx with multiple identical TxInput shouldn't be valid")
	}
}

func TestDeleteOneBalanceItem(t *testing.T) {
	bc := CreateBlockchain(TestAddressOne, 0)
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	bc.AddNewUserTransfer(utxoPool, PriKeyOne, TestAddressOne, TestAddressThree, 3, 0)
	bc.AddNewUserTransfer(utxoPool, PriKeyThree, TestAddressThree, TestAddressTwo, 3, 0)

	if _, ok := utxoPool.UtxoMap[TestAddressThree]; ok {
		t.Errorf("alok should not be contained in the balance map")
	}
}

func TestCleanUp(t *testing.T) {
	var utxoPool UTXOPool
	utxoPool.UtxoMap = make(UtxoMap)
	utxoPool.UtxoMap[TestAddressOne] = make(TXHash2Vout2AmountMap)
	utxoPool.UtxoMap[TestAddressTwo] = TXHash2Vout2AmountMap{
		"abcd": {
			0: 1,
		},
	}
	utxoPool.CleanUp()
	if _, ok := utxoPool.UtxoMap[TestAddressOne]; ok {
		t.Errorf("minh should not be contained in the balance map")
	}
}
