package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
)

// Transaction represents a Bitcoin transaction
type Transaction struct {
	ID       [32]byte // 32 byte hash
	TxInput  []TXInput
	TxOutput []TXOutput

	Proofs []CrossShardTxProof // The proofs for crossShard tx unlock-to-commit/abort
}

// TXOutput is the struct of transaction output in a transaction.
type TXOutput struct {
	Value   int
	Address string
	ShardId uint32 // The Id of the shard where this UTXO belongs
}

// TXInput is the struct of transaction input (a UTXO) in a transaction.
type TXInput struct {
	TxID          [32]byte
	TxOutputIndex int
	Address       string
	ShardId       uint32 // The Id of the shard where this UTXO belongs
}

type CrossShardTxProof struct {
	RejectOrAccept bool      // false means rejection, true means acceptance
	TxID           [32]byte  // Id of transaction whose utxo is related to this proof
	TxInput        []TXInput // The list of Utxo that this proof is referring to. They should be in the same shard.
	BlockHash      [32]byte  // The hash of the block where the proof is registered
	// Signatures
}

// This is a internal data structure that doesn't go across network
type CrossShardTxAndProof struct {
	Transaction *Transaction       // The cross shard tx
	Proof       *CrossShardTxProof // The proof
}

// DefaultCoinbaseValue is the default value of coinbase transaction.
const DefaultCoinbaseValue = 1000

// SetID sets ID of a transaction (32 byte hash of the whole transaction)
func (tx *Transaction) SetID() {
	var encoded bytes.Buffer
	var hash [32]byte

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	hash = sha256.Sum256(encoded.Bytes())
	tx.ID = hash
}

// NewCoinbaseTX creates a new coinbase transaction
func NewCoinbaseTX(to, data string, shardId uint32) *Transaction {
	if data == "" {
		data = fmt.Sprintf("Reward to '%s'", to)
	}

	txin := TXInput{[32]byte{}, -1, data, shardId}
	txout := TXOutput{DefaultCoinbaseValue, to, shardId}
	tx := Transaction{[32]byte{}, []TXInput{txin}, []TXOutput{txout}, nil}
	tx.SetID()
	return &tx
}

// Used for debuging.
func (txInput *TXInput) String() string {
	res := fmt.Sprintf("TxID: %v, ", hex.EncodeToString(txInput.TxID[:]))
	res += fmt.Sprintf("TxOutputIndex: %v, ", txInput.TxOutputIndex)
	res += fmt.Sprintf("Address: %v, ", txInput.Address)
	res += fmt.Sprintf("Shard Id: %v", txInput.ShardId)
	return res
}

// Used for debuging.
func (txOutput *TXOutput) String() string {
	res := fmt.Sprintf("Value: %v, ", txOutput.Value)
	res += fmt.Sprintf("Address: %v", txOutput.Address)
	return res
}

// Used for debuging.
func (proof *CrossShardTxProof) String() string {
	res := fmt.Sprintf("RejectOrAccept: %v, ", proof.RejectOrAccept)
	return res
}

// Used for debuging.
func (tx *Transaction) String() string {
	res := fmt.Sprintf("ID: %v\n", hex.EncodeToString(tx.ID[:]))
	res += fmt.Sprintf("TxInput:\n")
	for id, value := range tx.TxInput {
		res += fmt.Sprintf("%v: %v\n", id, value.String())
	}
	res += fmt.Sprintf("TxOutput:\n")
	for id, value := range tx.TxOutput {
		res += fmt.Sprintf("%v: %v\n", id, value.String())
	}
	for id, value := range tx.Proofs {
		res += fmt.Sprintf("%v: %v\n", id, value.String())
	}
	return res
}
