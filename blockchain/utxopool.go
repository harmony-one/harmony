package blockchain

import (
	"encoding/hex"
	"fmt"
)

const (
	// MaxNumberOfTransactions is the max number of transaction per a block.
	MaxNumberOfTransactions = 100
)

// UTXOPool stores transactions and balance associated with each address.
type UTXOPool struct {
	// Mapping from address to a map of transaction id to a map of the index of output
	// array in that transaction to that balance.
	utxo map[string]map[string]map[int]int
}

// VerifyTransactions verifies if a list of transactions valid.
func (utxoPool *UTXOPool) VerifyTransactions(transactions []*Transaction) bool {
	spentTXOs := make(map[string]map[string]map[int]bool)
	if utxoPool != nil {
		for _, tx := range transactions {
			inTotal := 0
			// Calculate the sum of TxInput
			for _, in := range tx.TxInput {
				inTxID := hex.EncodeToString(in.TxID)
				index := in.TxOutputIndex
				// Check if the transaction with the addres is spent or not.
				if val, ok := spentTXOs[in.Address][inTxID][index]; ok {
					if val {
						return false
					}
				}
				// Mark the transactions with the address and index spent.
				if _, ok := spentTXOs[in.Address]; !ok {
					spentTXOs[in.Address] = make(map[string]map[int]bool)
				}
				if _, ok := spentTXOs[in.Address][inTxID]; !ok {
					spentTXOs[in.Address][inTxID] = make(map[int]bool)
				}
				spentTXOs[in.Address][inTxID][index] = true

				// Sum the balance up to the inTotal.
				if val, ok := utxoPool.utxo[in.Address][inTxID][index]; ok {
					inTotal += val
				} else {
					return false
				}
			}

			outTotal := 0
			// Calculate the sum of TxOutput
			for _, out := range tx.TxOutput {
				outTotal += out.Value
			}
			if inTotal != outTotal {
				return false
			}
		}
	}
	return true
}

// VerifyOneTransaction verifies if a list of transactions valid.
func (utxoPool *UTXOPool) VerifyOneTransaction(tx *Transaction) bool {
	spentTXOs := make(map[string]map[string]map[int]bool)
	txID := hex.EncodeToString(tx.ID)
	inTotal := 0
	// Calculate the sum of TxInput
	for _, in := range tx.TxInput {
		inTxID := hex.EncodeToString(in.TxID)
		index := in.TxOutputIndex
		// Check if the transaction with the addres is spent or not.
		if val, ok := utxoPool.utxo[in.Address][inTxID][index]; ok {
			inTotal += val
		} else {
			return false
		}
		// Mark the transactions with the address and index spent.
		if _, ok := spentTXOs[in.Address]; !ok {
			spentTXOs[in.Address] = make(map[string]map[int]bool)
		}
		if _, ok := spentTXOs[in.Address][inTxID]; !ok {
			spentTXOs[in.Address][inTxID] = make(map[int]bool)
		}
		if spentTXOs[in.Address][inTxID][index] {
			return false
		}
		spentTXOs[in.Address][inTxID][index] = true
	}

	outTotal := 0
	// Calculate the sum of TxOutput
	for index, out := range tx.TxOutput {
		if _, ok := spentTXOs[out.Address][txID][index]; ok {
			return false
		}
		outTotal += out.Value
	}
	if inTotal != outTotal {
		return false
	}
	return true
}

// UpdateOneTransaction updates utxoPool in respect to the new Transaction.
func (utxoPool *UTXOPool) UpdateOneTransaction(tx *Transaction) {
	if utxoPool != nil {
		txID := hex.EncodeToString(tx.ID)

		// Remove
		for _, in := range tx.TxInput {
			inTxID := hex.EncodeToString(in.TxID)
			delete(utxoPool.utxo[in.Address][inTxID], in.TxOutputIndex)
		}

		// Update
		for index, out := range tx.TxOutput {
			if _, ok := utxoPool.utxo[out.Address]; !ok {
				utxoPool.utxo[out.Address] = make(map[string]map[int]int)
				utxoPool.utxo[out.Address][txID] = make(map[int]int)
			}
			if _, ok := utxoPool.utxo[out.Address][txID]; !ok {
				utxoPool.utxo[out.Address][txID] = make(map[int]int)
			}
			utxoPool.utxo[out.Address][txID][index] = out.Value
		}
	}
}

// VerifyOneTransactionAndUpdate verifies and update a valid transaction.
// Return false if the transaction is not valid.
func (utxoPool *UTXOPool) VerifyOneTransactionAndUpdate(tx *Transaction) bool {
	if utxoPool.VerifyOneTransaction(tx) {
		utxoPool.UpdateOneTransaction(tx)
		return true
	}
	return false
}

// VerifyAndUpdate verifies a list of transactions and update utxoPool.
func (utxoPool *UTXOPool) VerifyAndUpdate(transactions []*Transaction) bool {
	if utxoPool.VerifyTransactions(transactions) {
		utxoPool.Update(transactions)
		return true
	}
	return false
}

// Update utxo balances with a list of new transactions.
func (utxoPool *UTXOPool) Update(transactions []*Transaction) {
	if utxoPool != nil {
		for _, tx := range transactions {
			curTxID := hex.EncodeToString(tx.ID)

			// Remove
			for _, in := range tx.TxInput {
				inTxID := hex.EncodeToString(in.TxID)
				delete(utxoPool.utxo[in.Address][inTxID], in.TxOutputIndex)
			}

			// Update
			for index, out := range tx.TxOutput {
				if _, ok := utxoPool.utxo[out.Address]; !ok {
					utxoPool.utxo[out.Address] = make(map[string]map[int]int)
					utxoPool.utxo[out.Address][curTxID] = make(map[int]int)
				}
				if _, ok := utxoPool.utxo[out.Address][curTxID]; !ok {
					utxoPool.utxo[out.Address][curTxID] = make(map[int]int)
				}
				utxoPool.utxo[out.Address][curTxID][index] = out.Value
			}
		}
	}
}

// CreateUTXOPoolFromTransaction a utxo pool from a genesis transaction.
func CreateUTXOPoolFromTransaction(tx *Transaction) *UTXOPool {
	var utxoPool UTXOPool
	txID := hex.EncodeToString(tx.ID)
	utxoPool.utxo = make(map[string]map[string]map[int]int)
	for index, out := range tx.TxOutput {
		utxoPool.utxo[out.Address] = make(map[string]map[int]int)
		utxoPool.utxo[out.Address][txID] = make(map[int]int)
		utxoPool.utxo[out.Address][txID][index] = out.Value
	}
	return &utxoPool
}

// CreateUTXOPoolFromGenesisBlockChain a utxo pool from a genesis blockchain.
func CreateUTXOPoolFromGenesisBlockChain(bc *Blockchain) *UTXOPool {
	tx := bc.Blocks[0].Transactions[0]
	return CreateUTXOPoolFromTransaction(tx)
}

// SelectTransactionsForNewBlock returns a list of index of valid transactions for the new block.
func (utxoPool *UTXOPool) SelectTransactionsForNewBlock(transactions []*Transaction) ([]*Transaction, []*Transaction) {
	selected, unselected := []*Transaction{}, []*Transaction{}
	for _, tx := range transactions {
		if len(selected) < MaxNumberOfTransactions && utxoPool.VerifyOneTransactionAndUpdate(tx) {
			selected = append(selected, tx)
		} else {
			unselected = append(unselected, tx)
		}
	}
	return selected, unselected
}

// Used for debugging.
func (utxoPool *UTXOPool) String() string {
	res := ""
	for address, v1 := range utxoPool.utxo {
		for txid, v2 := range v1 {
			for index, value := range v2 {
				res += fmt.Sprintf("address: %v, tx id: %v, index: %v, value: %v\n", address, txid, index, value)

			}
		}
	}
	return res
}
