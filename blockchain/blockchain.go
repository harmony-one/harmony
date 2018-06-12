package blockchain

import (
	"encoding/hex"
)

// Blockchain keeps a sequence of Blocks
type Blockchain struct {
	blocks []*Block
}

const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

// FindUnspentTransactions returns a list of transactions containing unspent outputs
func (bc *Blockchain) FindUnspentTransactions(address string) []Transaction {
	var unspentTXs []Transaction
	spentTXOs := make(map[string][]int)

	for index := len(bc.blocks) - 1; index >= 0; index-- {
		block := bc.blocks[index]

	BreakTransaction:
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

			idx := -1
			if spentTXOs[txID] != nil {
				idx = 0
			}
			for outIdx, txOutput := range tx.TxOutput {
				if idx >= 0 && spentTXOs[txID][idx] == outIdx {
					idx++
					continue
				}

				if txOutput.Address == address {
					unspentTXs = append(unspentTXs, *tx)
					continue BreakTransaction
				}
			}
		}
	}
	return unspentTXs
}

// FindUTXO finds and returns all unspent transaction outputs
func (bc *Blockchain) FindUTXO(address string) []TXOutput {
	var UTXOs []TXOutput
	unspentTXs := bc.FindUnspentTransactions(address)

	for _, tx := range unspentTXs {
		for _, txOutput := range tx.TxOutput {
			if txOutput.Address == address {
				UTXOs = append(UTXOs, txOutput)
				break
			}
		}
	}

	return UTXOs
}

// FindSpendableOutputs finds and returns unspent outputs to reference in inputs
func (bc *Blockchain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOutputs := make(map[string][]int)
	unspentTXs := bc.FindUnspentTransactions(address)
	accumulated := 0

Work:
	for _, tx := range unspentTXs {
		txID := hex.EncodeToString(tx.ID)

		for outIdx, txOutput := range tx.TxOutput {
			if txOutput.Address == address && accumulated < amount {
				accumulated += txOutput.Value
				unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)

				if accumulated >= amount {
					break Work
				}
			}
		}
	}

	return accumulated, unspentOutputs
}

// NewUTXOTransaction creates a new transaction
func (bc *Blockchain) NewUTXOTransaction(from, to string, amount int) *Transaction {
	var inputs []TXInput
	var outputs []TXOutput

	acc, validOutputs := bc.FindSpendableOutputs(from, amount)

	if acc < amount {
		return nil
	}

	// Build a list of inputs
	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			return nil
		}

		for _, out := range outs {
			input := TXInput{txID, out, from}
			inputs = append(inputs, input)
		}
	}

	// Build a list of outputs
	outputs = append(outputs, TXOutput{amount, to})
	if acc > amount {
		outputs = append(outputs, TXOutput{acc - amount, from}) // a change
	}

	tx := Transaction{nil, inputs, outputs}
	tx.SetID()

	return &tx
}

// AddNewTransferAmount creates a new transaction and a block of that transaction.
// Mostly used for testing.
func (bc *Blockchain) AddNewTransferAmount(from, to string, amount int) *Blockchain {
	tx := bc.NewUTXOTransaction(from, to, amount)
	if tx != nil {
		newBlock := NewBlock([]*Transaction{tx}, bc.blocks[len(bc.blocks)-1].PrevBlockHash)
		bc.blocks = append(bc.blocks, newBlock)
		return bc
	}
	return nil
}

// CreateBlockchain creates a new blockchain DB
func CreateBlockchain(address string) *Blockchain {
	// TODO: We assume we have not created any blockchain before.
	// In current bitcoin, we can check if we created a blockchain before accessing local db.
	cbtx := NewCoinbaseTX(address, genesisCoinbaseData)
	genesis := NewGenesisBlock(cbtx)

	bc := Blockchain{[]*Block{genesis}}

	return &bc
}
