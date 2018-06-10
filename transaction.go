package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
)

// Transaction represents a Bitcoin transaction
type Transaction struct {
	id       []byte
	txInput  []TXInput
	txOutput []TXOutput
}

type TXOutput struct {
	address string
	value   int
}

type TXInput struct {
	txId          []byte
	txOutputIndex int
	address       string
}

// SetID sets ID of a transaction
func (tx *Transaction) SetId() {
	var encoded bytes.Buffer
	var hash [32]byte

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	hash = sha256.Sum256(encoded.Bytes())
	tx.ID = hash[:]
}
