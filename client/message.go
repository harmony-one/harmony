package client

import (
	"bytes"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/common"
)

// The specific types of message under CLIENT category
type ClientMessageType byte

const (
	TRANSACTION ClientMessageType = iota
	// TODO: add more types
)

// The types of messages used for CLIENT/TRANSACTION
type TransactionMessageType int

const (
	CROSS_TX TransactionMessageType = iota // The proof of accept or reject returned by the leader to the cross shard transaction client.
)

// Used to aggregated proofs and unlock utxos in cross shard tx
type CrossShardTxAndProofs struct {
	Transaction blockchain.Transaction         // The cross shard tx
	Proofs      []blockchain.CrossShardTxProof // The proofs
}

//ConstructStopMessage is STOP message
func ConstructProofOfAcceptOrRejectMessage(proofs []blockchain.CrossShardTxProof) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.CLIENT)})
	byteBuffer.WriteByte(byte(TRANSACTION))
	byteBuffer.WriteByte(byte(CROSS_TX))
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(proofs)
	return byteBuffer.Bytes()
}
