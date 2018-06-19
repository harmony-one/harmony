package blockchain

import (
	"encoding/hex"
)

const (
	MaxNumberOfTransactions = 100
)

// UTXOPool stores transactions and balance associated with each address.
type UTXOPool struct {
	// Mapping from address to a map of transaction id to a map of the index of output
	// array in that transaction to that balance.
	utxo map[string]map[string]map[int]int
}

// VerifyTransactions verifies if a list of transactions valid.
func (utxopool *UTXOPool) VerifyTransactions(transactions []*Transaction) bool {
	spentTXOs := make(map[string]map[string]map[int]bool)
	if utxopool != nil {
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
				if val, ok := utxopool.utxo[in.Address][inTxID][index]; ok {
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

// VerifyOneTransactionAndUpdate verifies if a list of transactions valid.
func (utxopool *UTXOPool) VerifyOneTransaction(tx *Transaction) bool {
    spentTXOs := make(map[string]map[string]map[int]bool)
    txID := hex.EncodeToString(tx.ID)
    inTotal := 0
    // Calculate the sum of TxInput
    for _, in := range tx.TxInput {
        inTxID := hex.EncodeToString(in.TxID)
        index := in.TxOutputIndex
        // Check if the transaction with the addres is spent or not.
        if val, ok := utxopool.utxo[in.Address][inTxID][index]; ok {
            if spentTXOs[in.Address][inTxID][index] {
                return false
            }
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
        spentTXOs[in.Address][inTxID][index] = true
    }

    outTotal := 0
    // Calculate the sum of TxOutput
    for index, out := range tx.TxOutput {
        if val, ok := spentTXOs[out.Address][txID][index]; ok {
            return false
        }
        outTotal += out.Value
    }
    if inTotal != outTotal {
        return false
    }
	return true
}

func (utxopool *UTXOPool) Update(tx* Transaction) {
	if utxopool != nil {
        txID := hex.EncodeToString(tx.ID)

        // Remove
        for _, in := range tx.TxInput {
            inTxID := hex.EncodeToString(in.TxID)
            delete(utxopool.utxo[in.Address][inTxID], in.TxOutputIndex)
        }

        // Update
        for index, out := range tx.TxOutput {
            if _, ok := utxopool.utxo[out.Address]; !ok {
                utxopool.utxo[out.Address] = make(map[string]map[int]int)
                utxopool.utxo[out.Address][txID] = make(map[int]int)
            }
            if _, ok := utxopool.utxo[out.Address][txID]; !ok {
                utxopool.utxo[out.Address][txID] = make(map[int]int)
            }
            utxopool.utxo[out.Address][txID][index] = out.Value
        }
    }
}

func (utxopool *UTXOPool) VerifyOneTransactionAndUpdate(tx *Transaction) bool {
    if utxopool.VerifyOneTransaction(tx) {
        utxopool.Update(tx)
        return true
    }
    retur false
}

// VerifyAndUpdate verifies a list of transactions and update utxoPool.
func (utxopool *UTXOPool) VerifyAndUpdate(transactions []*Transaction) bool {
	if utxopool.VerifyTransactions(transactions) {
		utxopool.Update(transactions)
		return true
	}
	return false
}

// Update utxo balances with a list of new transactions.
func (utxopool *UTXOPool) Update(transactions []*Transaction) {
	if utxopool != nil {
		for _, tx := range transactions {
			curTxID := hex.EncodeToString(tx.ID)

			// Remove
			for _, in := range tx.TxInput {
				inTxID := hex.EncodeToString(in.TxID)
				delete(utxopool.utxo[in.Address][inTxID], in.TxOutputIndex)
			}

			// Update
			for index, out := range tx.TxOutput {
				if _, ok := utxopool.utxo[out.Address]; !ok {
					utxopool.utxo[out.Address] = make(map[string]map[int]int)
					utxopool.utxo[out.Address][curTxID] = make(map[int]int)
				}
				if _, ok := utxopool.utxo[out.Address][curTxID]; !ok {
					utxopool.utxo[out.Address][curTxID] = make(map[int]int)
				}
				utxopool.utxo[out.Address][curTxID][index] = out.Value
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
	tx := bc.blocks[0].Transactions[0]
	return CreateUTXOPoolFromTransaction(tx)
}

// SelectTransactionsForNewBlock returns a list of index of valid transactions for the new block.
func (utxoPool *UTXOPool) SelectTransactionsForNewBlock(transactions []*Transaction) (selected, unselected []*Transaction) {
	selected, unselected = []*Transaction{}, []*Transaction{}
	for id, tx := range transactions {
        if len(selected) < MaxNumberOfTransactions && utxoPool.VerifyOneTransactionAndUpdate(tx) {
            append(selected, tx)
        } else {
            append(unselected, tx)
        }
	}
}
