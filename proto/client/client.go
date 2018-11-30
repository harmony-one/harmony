package client

import (
	"bytes"
	"encoding/gob"

	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/proto"
)

// MessageType is the specific types of message under Client category
type MessageType byte

// Message type supported by client
const (
	Transaction MessageType = iota
	// TODO: add more types
)

// TransactionMessageType defines the types of messages used for Client/Transaction
type TransactionMessageType int

// The proof of accept or reject returned by the leader to the client tnat issued cross shard transactions
const (
	ProofOfLock TransactionMessageType = iota
	UtxoResponse
)

// FetchUtxoResponseMessage is the data structure of UTXO map
type FetchUtxoResponseMessage struct {
	UtxoMap blockchain.UtxoMap
	ShardID uint32
}

// ConstructProofOfAcceptOrRejectMessage constructs the proof of accept or reject message that will be sent to client
func ConstructProofOfAcceptOrRejectMessage(proofs []blockchain.CrossShardTxProof) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Client)})
	byteBuffer.WriteByte(byte(Transaction))
	byteBuffer.WriteByte(byte(ProofOfLock))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(proofs)
	return byteBuffer.Bytes()
}

// ConstructFetchUtxoResponseMessage constructs the response message to fetch utxo message
func ConstructFetchUtxoResponseMessage(utxoMap *blockchain.UtxoMap, shardID uint32) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Client)})
	byteBuffer.WriteByte(byte(Transaction))
	byteBuffer.WriteByte(byte(UtxoResponse))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(FetchUtxoResponseMessage{*utxoMap, shardID})
	return byteBuffer.Bytes()
}
