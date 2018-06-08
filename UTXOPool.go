package main

// UTXOPool is the data structure to store the current balance.
type UTXOPool struct {
	utxos map[string]int
}

// func (utxoPool *UTXOPool) handleTransaction(transaction Transaction, receiver string) {
// 	if !isValidTransaction(transaction) {
// 		return
// 	}
// 	// utxoPool[]
// }

// func (utxoPool *UTXOPool) isValidTransaction(transaction Transaction) {
// 	const { inputPublicKey, amount, fee } = transaction
// 	const utxo = this.utxos[inputPublicKey]
// 	return utxo !== undefined && utxo.amount >= (amount + fee) && amount > 0
// }

// func (utxoPool *UTXOPool) handleTransaction(transaction, feeReceiver) {
//     if (!this.isValidTransaction(transaction))
//       return
//     const inputUTXO = this.utxos[transaction.inputPublicKey];
//     inputUTXO.amount -= transaction.amount
//     inputUTXO.amount -= transaction.fee
//     if (inputUTXO.amount === 0)
//       delete this.utxos[transaction.inputPublicKey]
//     this.addUTXO(transaction.outputPublicKey, transaction.amount)
//     this.addUTXO(feeReceiver, transaction.fee)
//   }

//   func (utxoPool *UTXOPool) isValidTransaction(transaction Transaction) {
//     const { inputPublicKey, amount, fee } = transaction
//     const utxo = utxoPool.utxos[inputPublicKey]
//     return utxo !== undefined && utxo.amount >= (amount + fee) && amount > 0
//   }
