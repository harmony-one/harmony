package client

import (
	"bytes"
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

//ConstructStopMessage is STOP message
func ConstructProofOfAcceptOrRejectMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.CLIENT)})
	byteBuffer.WriteByte(byte(TRANSACTION))
	byteBuffer.WriteByte(byte(CROSS_TX))
	return byteBuffer.Bytes()
}
