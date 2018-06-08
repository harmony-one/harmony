package main

import (
	"bytes"
	"crypto/sha256"
	"strconv"
	"time"
)

// Constants
const (
	TOTAL_COINS = 21000000
)

// Block keeps block headers.
type Block struct {
	Timestamp     int64
	utxoPool      []UTXOPool
	PrevBlockHash []byte
	Hash          []byte
}

//SetHash calculates and sets block hash.
func (b *Block) SetHash() {
	timestamp := []byte(strconv.FormatInt(b.Timestamp, 10))
	// headers := bytes.Join([][]byte{b.PrevBlockHash, b.Data, timestamp}, []byte{})
	headers := bytes.Join([][]byte{b.PrevBlockHash, timestamp}, []byte{})
	hash := sha256.Sum256(headers)

	b.Hash = hash[:]
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
