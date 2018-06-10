package main

import (
    "encoding/hex"
)

// Blockchain keeps a sequence of Blocks
type Blockchain struct {
    blocks []*Block
}

// AddBlock saves provided data as a block in the blockchain
func (bc *Blockchain) AddBlock(data string) {
    prevBlock := bc.blocks[len(bc.blocks)-1]

    // TODO(minhdoan): Parse data.
    newBlock := NewBlock({}, prevBlock.Hash)
    bc.blocks = append(bc.blocks, newBlock)
}

// NewBlockchain creates a new Blockchain with genesis Block
func NewBlockchain() *Blockchain {
    return &Blockchain{[]*Block{NewGenesisBlock()}}
}

// FindUnspentTransactions returns a list of transactions containing unspent outputs
func (bc *Blockchain) FindUnspentTransactions(address string) []Transaction {
    var unspentTXs []Transaction
    spentTXOs := make(map[string][]int)

    for index := len(bc.blocks) - 1; index >= 0; index-- {
        block := bc.blocks[index];

        BreakTransaction:
        for _, tx := range block.Transactions {
            txId := hex.EncodeToString(tx.Id)

            idx := -1
            if spentTXOs[txId] != nil {
                idx = 0
            }
            for outIdx, txOutput  := range tx.txOutput {
                if idx >= 0 && spentTXOs[txId][idx] == outIdx {
                    idx++
                    continue
                }

                if txOutput.address == address {
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
        for _, txOutput := range tx.txOutput {
            if txOutput.address == address {
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

        for outIdx, txOutput := range tx.txOutput {
            if txOutput.address == address && accumulated < amount {
                accumulated += txOutput.value
                unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)

                if accumulated >= amount {
                    break Work
                }
            }
        }
    }

    return accumulated, unspentOutputs
}
