package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"log"
	"time"
)

// Block keeps block headers, transactions and signature.
type Block struct {
	// Header
	Timestamp       int64
	PrevBlockHash   [32]byte
	Hash            [32]byte
	NumTransactions int32
	TransactionIds  [][32]byte
	Transactions    []*Transaction // Transactions
	ShardId         uint32
	// Signature...
}

// Serialize serializes the block
func (b *Block) Serialize() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(b)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}

// DeserializeBlock deserializes a block
func DeserializeBlock(d []byte) *Block {
	var block Block
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}
	return &block
}

// Used for debuging.
func (b *Block) String() string {
	res := fmt.Sprintf("Block created at %v\n", b.Timestamp)
	for id, tx := range b.Transactions {
		res += fmt.Sprintf("Transaction %v: %v\n", id, *tx)
	}
	res += fmt.Sprintf("previous blockhash: %v\n", b.PrevBlockHash)
	res += fmt.Sprintf("hash: %v\n", b.Hash)
	return res
}

// HashTransactions returns a hash of the transactions in the block
func (b *Block) HashTransactions() []byte {
	var txHashes [][]byte
	var txHash [32]byte

	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.ID[:])
	}
	txHash = sha256.Sum256(bytes.Join(txHashes, []byte{}))
	return txHash[:]
}

// NewBlock creates and returns a neew block.
func NewBlock(transactions []*Transaction, prevBlockHash [32]byte, shardId uint32) *Block {
	numTxs := int32(len(transactions))
	var txIds [][32]byte

	for _, tx := range transactions {
		txIds = append(txIds, tx.ID)
	}
	block := &Block{time.Now().Unix(), prevBlockHash, [32]byte{}, numTxs, txIds, transactions, shardId}
	copy(block.Hash[:], block.HashTransactions()[:]) // TODO(Minh): the blockhash should be a hash of everything in the block

	return block
}

// NewGenesisBlock creates and returns genesis Block.
func NewGenesisBlock(coinbase *Transaction, shardId uint32) *Block {
	return NewBlock([]*Transaction{coinbase}, [32]byte{}, shardId)
}
