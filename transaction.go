package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"strconv"
	"strings"
)

// Transaction represents a Bitcoin transaction
type Transaction struct {
	ID            []byte
	inputAddresss []string
	outputAddress []string
	value         []int
}

func (tx *Transaction) Parse(data string) {
	items := strings.Split(data, ",")
	for _, value := range items {
		pair := strings.Split(value, " ")
		if len(pair) == 3 {
			intValue, err := strconv.Atoi(pair[2])
			if err != nil {
				tx.inputAddress = append(tx.inputAddresss, strings.Trim(pair[0]))
				tx.outputAddress = append(tx.outputAddress, strings.Trim(pair[1]))
				tx.value = append(tx.value, intValue)
			}
		}
	}
	return res
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
