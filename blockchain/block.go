package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"log"
	"time"

	"github.com/simple-rules/harmony-benchmark/db"
	"github.com/simple-rules/harmony-benchmark/utils"
)

// A block in the blockchain that contains block headers, transactions and signature etc.
type Block struct {
	// Header
	Timestamp       int64
	PrevBlockHash   [32]byte
	NumTransactions int32
	TransactionIds  [][32]byte
	Transactions    []*Transaction // Transactions
	ShardId         uint32
	Hash            [32]byte
	MerkleRootData  []byte
	// Signature...
	Bitmap    []byte   // Contains which validator signed the block.
	Signature [66]byte // Schnorr collective signature
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

func (b *Block) generateMerkleRootData() {
	var data [][]byte

	for _, txId := range b.TransactionIds {
		data = append(data, txId[:])
	}
	merkleTre := NewMerkleTree(data)
	b.MerkleRootData = merkleTre.RootNode.Data
}

func (b *Block) Write(db db.Database, key string) error {
	return db.Put([]byte(key), b.Serialize())
}

// CalculateBlockHash returns a hash of the block
func (b *Block) CalculateBlockHash() []byte {
	var hashes [][]byte
	var blockHash [32]byte

	hashes = append(hashes, utils.ConvertFixedDataIntoByteArray(b.Timestamp))
	hashes = append(hashes, b.PrevBlockHash[:])
	for _, id := range b.TransactionIds {
		hashes = append(hashes, id[:])
	}
	b.generateMerkleRootData()

	hashes = append(hashes, b.MerkleRootData)
	hashes = append(hashes, utils.ConvertFixedDataIntoByteArray(b.ShardId))

	blockHash = sha256.Sum256(bytes.Join(hashes, []byte{}))
	return blockHash[:]
}

// NewBlock creates and returns a new block.
func NewBlock(transactions []*Transaction, prevBlockHash [32]byte, shardId uint32) *Block {
	numTxs := int32(len(transactions))
	var txIds [][32]byte

	for _, tx := range transactions {
		txIds = append(txIds, tx.ID)
	}
	block := &Block{Timestamp: time.Now().Unix(), PrevBlockHash: prevBlockHash, NumTransactions: numTxs, TransactionIds: txIds, Transactions: transactions, ShardId: shardId, Hash: [32]byte{}}
	copy(block.Hash[:], block.CalculateBlockHash()[:])

	return block
}

// NewGenesisBlock creates and returns genesis Block.
func NewGenesisBlock(coinbase *Transaction, shardId uint32) *Block {
	return NewBlock([]*Transaction{coinbase}, [32]byte{}, shardId)
}
