package node

import (
	"bytes"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/message"
)

type TransactionMessageType int

const (
	SEND TransactionMessageType = iota
)

type ControlMessageType int

const (
	STOP ControlMessageType = iota
)

//ConstructTransactionListMessage constructs serialized transactions
func ConstructTransactionListMessage(transactions []blockchain.Transaction) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(message.NODE)})
	byteBuffer.WriteByte(byte(message.TRANSACTION))
	byteBuffer.WriteByte(byte(SEND))
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(transactions)
	return byteBuffer.Bytes()
}

//ConstructStopMessage is  STOP message
func ConstructStopMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(message.NODE)})
	byteBuffer.WriteByte(byte(message.CONTROL))
	byteBuffer.WriteByte(byte(STOP))
	return byteBuffer.Bytes()
}
