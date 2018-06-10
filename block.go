package main

import (
    "bytes"
    "crypto/sha256"
    "time"
)

// Block keeps block headers.
type Block struct {
    Timestamp     int64
    Transactions  []*Transaction
    PrevBlockHash []byte
    Hash          []byte
}

// HashTransactions returns a hash of the transactions in the block
func (b *Block) HashTransactions() []byte {
    var txHashes [][]byte
    var txHash [32]byte

    for _, tx := range b.Transactions {
        txHashes = append(txHashes, tx.ID)
    }
    txHash = sha256.Sum256(bytes.Join(txHashes, []byte{}))
    return txHash[:]
}

// NewBlock creates and returns Block.
func NewBlock(utxoPool []UTXOPool, prevBlockHash []byte) *Block {

    block := &Block{time.Now().Unix(), utxoPool, prevBlockHash, []byte{}}
    block.SetHash()
    return block
}

// NewGenesisBlock creates and returns genesis Block.
func NewGenesisBlock() *Block {
    genesisUTXOPool := UTXOPool{}
    genesisUTXOPool.utxos["genesis"] = TOTAL_COINS

    return NewBlock(genesisUTXOPool, []byte{})
}
