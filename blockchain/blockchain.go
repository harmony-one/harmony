package blockchain

import (
	"bytes"
	"encoding/hex"

	"github.com/dedis/kyber"
	"github.com/simple-rules/harmony-benchmark/crypto/pki"
)

// Blockchain keeps a sequence of Blocks
type Blockchain struct {
	Blocks []*Block
}

const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

// blockHash should have size of 32.
func (bc *Blockchain) FindBlock(blockHash []byte) *Block {
	if len(blockHash) != 32 {
		return nil
	}
	for _, block := range bc.Blocks {
		if bytes.Compare(block.Hash[:], blockHash[:]) == 0 {
			return block
		}
	}
	return nil
}

func (bc *Blockchain) FindBlockWithPrevHash(prevHash []byte) *Block {
	if len(prevHash) != 32 {
		return nil
	}
	for _, block := range bc.Blocks {
		if bytes.Compare(block.PrevBlockHash[:], prevHash[:]) == 0 {
			return block
		}
	}
	return nil
}

// GetLatestBlock gests the latest block at the end of the chain
func (bc *Blockchain) GetLatestBlock() *Block {
	if len(bc.Blocks) == 0 {
		return nil
	}
	return bc.Blocks[len(bc.Blocks)-1]
}

// FindUnspentTransactions returns a list of transactions containing unspent outputs
func (bc *Blockchain) FindUnspentTransactions(address [20]byte) []Transaction {
	var unspentTXs []Transaction
	spentTXOs := make(map[string][]uint32)

	for index := len(bc.Blocks) - 1; index >= 0; index-- {
		block := bc.Blocks[index]

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID[:])

			idx := -1
			// TODO(minhdoan): Optimize this.
			if spentTXOs[txID] != nil {
				idx = 0
			}
			for outIdx, txOutput := range tx.TxOutput {
				if idx >= 0 && spentTXOs[txID][idx] == uint32(outIdx) {
					idx++
					continue
				}

				if txOutput.Address == address {
					unspentTXs = append(unspentTXs, *tx)
					// Break because of the assumption that each address appears once in output.
					break
				}
			}

			for _, txInput := range tx.TxInput {
				if address == txInput.Address {
					ID := hex.EncodeToString(txInput.PreviousOutPoint.TxID[:])
					spentTXOs[ID] = append(spentTXOs[ID], txInput.PreviousOutPoint.Index)
				}
			}
		}
	}
	return unspentTXs
}

// FindUTXO finds and returns all unspent transaction outputs
func (bc *Blockchain) FindUTXO(address [20]byte) []TXOutput {
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
func (bc *Blockchain) FindSpendableOutputs(address [20]byte, amount int) (int, map[string][]uint32) {
	unspentOutputs := make(map[string][]uint32)
	unspentTXs := bc.FindUnspentTransactions(address)
	accumulated := 0

Work:
	for _, tx := range unspentTXs {
		txID := hex.EncodeToString(tx.ID[:])

		for outIdx, txOutput := range tx.TxOutput {
			if txOutput.Address == address && accumulated < amount {
				accumulated += txOutput.Amount
				unspentOutputs[txID] = append(unspentOutputs[txID], uint32(outIdx))

				if accumulated >= amount {
					break Work
				}
			}
		}
	}

	return accumulated, unspentOutputs
}

// NewUTXOTransaction creates a new transaction
func (bc *Blockchain) NewUTXOTransaction(priKey kyber.Scalar, from, to [20]byte, amount int, shardID uint32) *Transaction {
	var inputs []TXInput
	var outputs []TXOutput

	acc, validOutputs := bc.FindSpendableOutputs(from, amount)

	if acc < amount {
		return nil
	}

	// Build a list of inputs
	for txid, outs := range validOutputs {
		id, err := hex.DecodeString(txid)
		if err != nil {
			return nil
		}

		txID := TxID{}
		copy(txID[:], id[:])
		for _, out := range outs {
			input := NewTXInput(NewOutPoint(&txID, out), from, shardID)
			inputs = append(inputs, *input)
		}
	}

	// Build a list of outputs
	outputs = append(outputs, TXOutput{amount, to, shardID})
	if acc > amount {
		outputs = append(outputs, TXOutput{acc - amount, from, shardID}) // a change
	}

	tx := Transaction{ID: [32]byte{}, TxInput: inputs, TxOutput: outputs, Proofs: nil}
	tx.SetID()

	pubKey := pki.GetPublicKeyFromScalar(priKey)
	bytes, err := pubKey.MarshalBinary()
	if err == nil {
		copy(tx.PublicKey[:], bytes)
	} else {
		panic("Failed to serialize public key")
	}
	tx.SetID() // TODO(RJ): figure out the correct way to set Tx ID.
	tx.Sign(priKey)

	return &tx
}

// AddNewUserTransfer creates a new transaction and a block of that transaction.
// Mostly used for testing.
func (bc *Blockchain) AddNewUserTransfer(utxoPool *UTXOPool, priKey kyber.Scalar, from, to [20]byte, amount int, shardId uint32) bool {
	tx := bc.NewUTXOTransaction(priKey, from, to, amount, shardId)
	if tx != nil {
		newBlock := NewBlock([]*Transaction{tx}, bc.Blocks[len(bc.Blocks)-1].Hash, shardId)
		if bc.VerifyNewBlockAndUpdate(utxoPool, newBlock) {
			return true
		}
	}
	return false
}

// VerifyNewBlockAndUpdate verifies if the new coming block is valid for the current blockchain.
func (bc *Blockchain) VerifyNewBlockAndUpdate(utxopool *UTXOPool, block *Block) bool {
	length := len(bc.Blocks)
	if bytes.Compare(block.PrevBlockHash[:], bc.Blocks[length-1].Hash[:]) != 0 {
		return false
	}
	if block.Timestamp < bc.Blocks[length-1].Timestamp {
		return false
	}

	if utxopool != nil && !utxopool.VerifyAndUpdate(block.Transactions) {
		return false
	}
	bc.Blocks = append(bc.Blocks, block)
	return true
}

// CreateBlockchain creates a new blockchain DB
// TODO(minhdoan): This func is not used, consider to remove.
func CreateBlockchain(address [20]byte, shardId uint32) *Blockchain {
	// TODO: We assume we have not created any blockchain before.
	// In current bitcoin, we can check if we created a blockchain before accessing local db.
	cbtx := NewCoinbaseTX(address, genesisCoinbaseData, shardId)
	genesis := NewGenesisBlock(cbtx, shardId)

	bc := Blockchain{[]*Block{genesis}}

	return &bc
}

// Create state block based on the utxos.
func (bc *Blockchain) CreateStateBlock(utxoPool *UTXOPool) *Block {
	var numBlocks int32 = 0
	var numTxs int32 = 0
	for _, block := range bc.Blocks {
		if block.IsStateBlock() {
			numBlocks += block.State.NumBlocks
			numTxs += block.State.NumTransactions
		} else {
			numBlocks += 1
			numTxs += block.NumTransactions
		}
	}
	return NewStateBlock(utxoPool, numBlocks, numTxs)
}
