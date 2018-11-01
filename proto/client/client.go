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
	Transaction ClientMessageType = iota
	// TODO: add more types
)

// The types of messages used for CLIENT/Transaction
type TransactionMessageType int

const (
	ProofOfLock TransactionMessageType = iota // The proof of accept or reject returned by the leader to the client tnat issued cross shard transactions.
	UtxoResponse
)

type FetchUtxoResponseMessage struct {
	UtxoMap blockchain.UtxoMap
	ShardID uint32
}

// [leader] Constructs the proof of accept or reject message that will be sent to client
func ConstructProofOfAcceptOrRejectMessage(proofs []blockchain.CrossShardTxProof) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.CLIENT)})
	byteBuffer.WriteByte(byte(Transaction))
	byteBuffer.WriteByte(byte(ProofOfLock))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(proofs)
	return byteBuffer.Bytes()
}

// Constructs the response message to fetch utxo message
func ConstructFetchUtxoResponseMessage(utxoMap *blockchain.UtxoMap, shardID uint32) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.CLIENT)})
	byteBuffer.WriteByte(byte(Transaction))
	byteBuffer.WriteByte(byte(UtxoResponse))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(FetchUtxoResponseMessage{*utxoMap, shardID})
	return byteBuffer.Bytes()
}
