package node

import (
	"bytes"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/common"
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
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
	byteBuffer.WriteByte(byte(common.TRANSACTION))
	byteBuffer.WriteByte(byte(SEND))
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(transactions)
	return byteBuffer.Bytes()
}

//ConstructStopMessage is  STOP message
func ConstructStopMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
	byteBuffer.WriteByte(byte(common.CONTROL))
	byteBuffer.WriteByte(byte(STOP))
	return byteBuffer.Bytes()
}
