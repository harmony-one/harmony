package client

import (
	"bytes"
	"encoding/gob"
	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/proto"
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
	PROOF_OF_LOCK TransactionMessageType = iota // The proof of accept or reject returned by the leader to the client tnat issued cross shard transactions.
	UTXO_RESPONSE
)

type FetchUtxoResponseMessage struct {
	UtxoMap blockchain.UtxoMap
	ShardId uint32
}

// [leader] Constructs the proof of accept or reject message that will be sent to client
func ConstructProofOfAcceptOrRejectMessage(proofs []blockchain.CrossShardTxProof) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.CLIENT)})
	byteBuffer.WriteByte(byte(TRANSACTION))
	byteBuffer.WriteByte(byte(PROOF_OF_LOCK))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(proofs)
	return byteBuffer.Bytes()
}

// Constructs the response message to fetch utxo message
func ConstructFetchUtxoResponseMessage(utxoMap *blockchain.UtxoMap, shardId uint32) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.CLIENT)})
	byteBuffer.WriteByte(byte(TRANSACTION))
	byteBuffer.WriteByte(byte(UTXO_RESPONSE))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(FetchUtxoResponseMessage{*utxoMap, shardId})
	return byteBuffer.Bytes()
}
