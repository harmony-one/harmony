package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"log"
	"time"

	"github.com/harmony-one/harmony/db"
	"github.com/harmony-one/harmony/utils"
)

const (
	// TimeStampForGenesisBlock is the constant timestamp for the genesis block.
	TimeStampForGenesisBlock = 0
)

// Block is a block in the blockchain that contains block headers, transactions and signature etc.
type Block struct {
	// Header
	Timestamp       int64
	PrevBlockHash   [32]byte
	NumTransactions int32
	TransactionIds  [][32]byte
	Transactions    []*Transaction // Transactions.
	ShardID         uint32
	Hash            [32]byte
	MerkleRootData  []byte
	State           *State // If present, this block is state block.
	// Signature...
	Bitmap    []byte   // Contains which validator signed the block.
	Signature [66]byte // Schnorr collective signature.

	AccountBlock []byte // Temporary piggy-back.
}

// State is used in Block to indicate that block is a state block.
type State struct {
	NumBlocks       int32 // Total number of blocks.
	NumTransactions int32 // Total number of transactions.
}

// IsStateBlock is used to check if a block is a state block.
func (b *Block) IsStateBlock() bool {
	// TODO: think of a better indicator to check.
	return b.State != nil && bytes.Equal(b.PrevBlockHash[:], (&[32]byte{})[:])
}

// Serialize serializes the block.
func (b *Block) Serialize() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(b)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}

// DeserializeBlock deserializes a block.
func DeserializeBlock(d []byte) (*Block, error) {
	var block Block
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}
	return &block, err
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

	for _, txID := range b.TransactionIds {
		data = append(data, txID[:])
	}
	merkleTre := NewMerkleTree(data)
	b.MerkleRootData = merkleTre.RootNode.Data
}

func (b *Block) Write(db db.Database, key string) error {
	return db.Put([]byte(key), b.Serialize())
}

// Delete deletes the given key in the given databse.
func Delete(db db.Database, key string) error {
	return db.Delete([]byte(key))
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
	hashes = append(hashes, utils.ConvertFixedDataIntoByteArray(b.ShardID))

	blockHash = sha256.Sum256(bytes.Join(hashes, []byte{}))
	return blockHash[:]
}

// NewBlock creates and returns a new block.
func NewBlock(transactions []*Transaction, prevBlockHash [32]byte, shardID uint32, isGenesisBlock bool) *Block {
	numTxs := int32(len(transactions))
	var txIDs [][32]byte

	for _, tx := range transactions {
		txIDs = append(txIDs, tx.ID)
	}
	timestamp := time.Now().Unix()
	if isGenesisBlock {
		timestamp = TimeStampForGenesisBlock
	}
	block := &Block{Timestamp: timestamp, PrevBlockHash: prevBlockHash, NumTransactions: numTxs, TransactionIds: txIDs, Transactions: transactions, ShardID: shardID, Hash: [32]byte{}}
	copy(block.Hash[:], block.CalculateBlockHash()[:])

	return block
}

// NewGenesisBlock creates and returns genesis Block.
func NewGenesisBlock(coinbase *Transaction, shardID uint32) *Block {
	return NewBlock([]*Transaction{coinbase}, [32]byte{}, shardID, true)
}

// NewStateBlock creates and returns a state Block based on utxo pool.
// TODO(RJ): take care of dangling cross shard transaction
func NewStateBlock(utxoPool *UTXOPool, numBlocks, numTxs int32) *Block {
	stateTransactions := []*Transaction{}
	stateTransactionIds := [][32]byte{}
	for address, txHash2Vout2AmountMap := range utxoPool.UtxoMap {
		stateTransaction := &Transaction{}
		for txHash, vout2AmountMap := range txHash2Vout2AmountMap {
			for index, amount := range vout2AmountMap {
				txHashBytes, err := utils.Get32BytesFromString(txHash)
				if err == nil {
					stateTransaction.TxInput = append(stateTransaction.TxInput, *NewTXInput(NewOutPoint(&txHashBytes, index), address, utxoPool.ShardID))
					stateTransaction.TxOutput = append(stateTransaction.TxOutput, TXOutput{Amount: amount, Address: address, ShardID: utxoPool.ShardID})
				} else {
					return nil
				}
			}
		}
		if len(stateTransaction.TxOutput) != 0 {
			stateTransaction.SetID()
			stateTransactionIds = append(stateTransactionIds, stateTransaction.ID)
			stateTransactions = append(stateTransactions, stateTransaction)
		}
	}
	newBlock := NewBlock(stateTransactions, [32]byte{}, utxoPool.ShardID, false)
	newBlock.State = &State{NumBlocks: numBlocks, NumTransactions: numTxs}
	return newBlock
}
