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
	PROOF_OF_LOCK TransactionMessageType = iota // The proof of accept or reject returned by the leader to the client tnat issued cross shard transactions.
)

// [leader] Constructs the proof of accept or reject message that will be sent to client
func ConstructProofOfAcceptOrRejectMessage(proofs []blockchain.CrossShardTxProof) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.CLIENT)})
	byteBuffer.WriteByte(byte(TRANSACTION))
	byteBuffer.WriteByte(byte(PROOF_OF_LOCK))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(proofs)
	return byteBuffer.Bytes()
}

// [client] Constructs the unlock to commit or abort message that will be sent to leaders
func ConstructUnlockToCommitOrAbortMessage(txsAndProofs []blockchain.Transaction) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
	byteBuffer.WriteByte(byte(0)) // A temporary hack to represent node.TRANSACTION, to avoid cyclical import. TODO: Potentially solution is to refactor all the message enums into a common package
	byteBuffer.WriteByte(byte(2)) // A temporary hack to represent node.UNLOCK, to avoid cyclical import. TODO: Potentially solution is to refactor all the message enums into a common package
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(txsAndProofs)
	return byteBuffer.Bytes()
}
