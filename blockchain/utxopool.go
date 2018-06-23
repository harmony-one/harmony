package blockchain

import (
	"encoding/hex"
	"fmt"
	"sync"
)

// UTXOPool stores transactions and balance associated with each address.
type UTXOPool struct {
	// Mapping from address to a map of transaction id to a map of the index of output
	// array in that transaction to that balance.
	/*
	   The 3-d map's structure:
	     address - [
	                txId1 - [
	                        outputIndex1 - value1
	                        outputIndex2 - value2
	                       ]
	                txId2 - [
	                        outputIndex1 - value1
	                        outputIndex2 - value2
	                       ]
	               ]
	*/
	UtxoMap map[string]map[string]map[int]int

	ShardId uint32
	mutex   sync.Mutex
}

// VerifyTransactions verifies if a list of transactions valid for this shard.
func (utxoPool *UTXOPool) VerifyTransactions(transactions []*Transaction) bool {
	spentTXOs := make(map[string]map[string]map[int]bool)
	if utxoPool != nil {
		for _, tx := range transactions {
			if valid, _ := utxoPool.VerifyOneTransaction(tx, &spentTXOs); !valid {
				return false
			}
		}
	}
	return true
}

// VerifyOneTransaction verifies if a list of transactions valid.
func (utxoPool *UTXOPool) VerifyOneTransaction(tx *Transaction, spentTXOs *map[string]map[string]map[int]bool) (valid, crossShard bool) {
	if spentTXOs == nil {
		spentTXOs = &map[string]map[string]map[int]bool{}
	}
	inTotal := 0
	// Calculate the sum of TxInput
	for _, in := range tx.TxInput {
		// Only check the input for my own shard.
		if in.ShardId != utxoPool.ShardId {
			crossShard = true
			continue
		}

		inTxID := hex.EncodeToString(in.TxID)
		index := in.TxOutputIndex
		// Check if the transaction with the addres is spent or not.
		if val, ok := (*spentTXOs)[in.Address][inTxID][index]; ok {
			if val {
				return false, crossShard
			}
		}
		// Mark the transactions with the address and index spent.
		if _, ok := (*spentTXOs)[in.Address]; !ok {
			(*spentTXOs)[in.Address] = make(map[string]map[int]bool)
		}
		if _, ok := (*spentTXOs)[in.Address][inTxID]; !ok {
			(*spentTXOs)[in.Address][inTxID] = make(map[int]bool)
		}
		(*spentTXOs)[in.Address][inTxID][index] = true

		// Sum the balance up to the inTotal.
		utxoPool.mutex.Lock()
		if val, ok := utxoPool.UtxoMap[in.Address][inTxID][index]; ok {
			inTotal += val
		} else {
			utxoPool.mutex.Unlock()
			return false, crossShard
		}
		utxoPool.mutex.Unlock()
	}

	outTotal := 0
	// Calculate the sum of TxOutput
	for _, out := range tx.TxOutput {
		outTotal += out.Value
	}
	if inTotal != outTotal {
		return false, crossShard
	}

	return true, crossShard
}

// Update Utxo balances with a list of new transactions.
func (utxoPool *UTXOPool) Update(transactions []*Transaction) {
	if utxoPool != nil {
		for _, tx := range transactions {
			utxoPool.UpdateOneTransaction(tx)
		}
	}
}

// UpdateOneTransaction updates utxoPool in respect to the new Transaction.
func (utxoPool *UTXOPool) UpdateOneTransaction(tx *Transaction) {
	utxoPool.mutex.Lock()
	defer utxoPool.mutex.Unlock()
	if utxoPool != nil {
		txID := hex.EncodeToString(tx.ID[:])

		isCrossShard := false
		// Remove
		for _, in := range tx.TxInput {
			// Only check the input for my own shard.
			if in.ShardId != utxoPool.ShardId {
				isCrossShard = true
				continue
			}

			inTxID := hex.EncodeToString(in.TxID)
			utxoPool.DeleteOneBalanceItem(in.Address, inTxID, in.TxOutputIndex)
		}

		// Update
		if !isCrossShard {
			for index, out := range tx.TxOutput {
				if _, ok := utxoPool.UtxoMap[out.Address]; !ok {
					utxoPool.UtxoMap[out.Address] = make(map[string]map[int]int)
					utxoPool.UtxoMap[out.Address][txID] = make(map[int]int)
				}
				if _, ok := utxoPool.UtxoMap[out.Address][txID]; !ok {
					utxoPool.UtxoMap[out.Address][txID] = make(map[int]int)
				}
				utxoPool.UtxoMap[out.Address][txID][index] = out.Value
			}
		} // If it's a cross shard Tx, then don't update so the input UTXOs are locked (removed), and the money is not spendable until unlock-to-commit or unlock-to-abort

		// TODO: unlock-to-commit and unlock-to-abort
	}
}

// VerifyOneTransactionAndUpdate verifies and update a valid transaction.
// Return false if the transaction is not valid.
func (utxoPool *UTXOPool) VerifyOneTransactionAndUpdate(tx *Transaction) bool {
	if valid, crossShard := utxoPool.VerifyOneTransaction(tx, nil); valid {
		utxoPool.UpdateOneTransaction(tx)
		if crossShard {
			// TODO: send proof-of-accceptance
		}
		return true
	} else if crossShard {
		if crossShard {
			// TODO: send proof-of-rejection
		}
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

// CreateUTXOPoolFromTransaction a Utxo pool from a genesis transaction.
func CreateUTXOPoolFromTransaction(tx *Transaction, shardId uint32) *UTXOPool {
	var utxoPool UTXOPool
	txID := hex.EncodeToString(tx.ID[:])
	utxoPool.UtxoMap = make(map[string]map[string]map[int]int)
	for index, out := range tx.TxOutput {
		utxoPool.UtxoMap[out.Address] = make(map[string]map[int]int)
		utxoPool.UtxoMap[out.Address][txID] = make(map[int]int)
		utxoPool.UtxoMap[out.Address][txID][index] = out.Value
	}
	utxoPool.ShardId = shardId
	return &utxoPool
}

// CreateUTXOPoolFromGenesisBlockChain a Utxo pool from a genesis blockchain.
func CreateUTXOPoolFromGenesisBlockChain(bc *Blockchain) *UTXOPool {
	tx := bc.Blocks[0].Transactions[0]
	shardId := bc.Blocks[0].ShardId
	return CreateUTXOPoolFromTransaction(tx, shardId)
}

// SelectTransactionsForNewBlock returns a list of index of valid transactions for the new block.
func (utxoPool *UTXOPool) SelectTransactionsForNewBlock(transactions []*Transaction, maxNumTxs int) ([]*Transaction, []*Transaction) {
	selected, unselected, crossShardTxs := []*Transaction{}, []*Transaction{}, []*Transaction{}
	spentTXOs := make(map[string]map[string]map[int]bool)
	for _, tx := range transactions {
		valid, crossShard := utxoPool.VerifyOneTransaction(tx, &spentTXOs)

		if len(selected) < maxNumTxs && valid {
			selected = append(selected, tx)
			if crossShard {
				crossShardTxs = append(crossShardTxs, tx)
			}
		} else {
			unselected = append(unselected, tx)
		}
	}
	// TODO: return crossShardTxs and keep track of it at node level
	return selected, unselected
}

// DeleteOneBalanceItem deletes one balance item of UTXOPool and clean up if possible.
func (utxoPool *UTXOPool) DeleteOneBalanceItem(address, txID string, index int) {
	delete(utxoPool.UtxoMap[address][txID], index)
	if len(utxoPool.UtxoMap[address][txID]) == 0 {
		delete(utxoPool.UtxoMap[address], txID)
		if len(utxoPool.UtxoMap[address]) == 0 {
			delete(utxoPool.UtxoMap, address)
		}
	}
}

// CleanUp cleans up UTXOPool.
func (utxoPool *UTXOPool) CleanUp() {
	for address, txMap := range utxoPool.UtxoMap {
		for txid, outIndexes := range txMap {
			for index, value := range outIndexes {
				if value == 0 {
					delete(utxoPool.UtxoMap[address][txid], index)
				}
			}
			if len(utxoPool.UtxoMap[address][txid]) == 0 {
				delete(utxoPool.UtxoMap[address], txid)
			}
		}
		if len(utxoPool.UtxoMap[address]) == 0 {
			delete(utxoPool.UtxoMap, address)
		}
	}
}

// Used for debugging.
func (utxoPool *UTXOPool) String() string {
	res := ""
	for address, v1 := range utxoPool.UtxoMap {
		for txid, v2 := range v1 {
			for index, value := range v2 {
				res += fmt.Sprintf("address: %v, tx id: %v, index: %v, value: %v\n", address, txid, index, value)

			}
		}
	}
	return res
}
