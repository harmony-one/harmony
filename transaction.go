package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"strings"
)

const subsidy = 10

// Transaction represents a Bitcoin transaction
type Transaction struct {
	ID   []byte
	data string
}

func (tx *Transaction) Parse() {
	strings.Split("a,b,c", ",")
}

// SetID sets ID of a transaction
func (tx *Transaction) SetID() {
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
