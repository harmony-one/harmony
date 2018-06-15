package node

import (
	"harmony-benchmark/blockchain"
	"bytes"
	"harmony-benchmark/message"
	"encoding/gob"
)

type TransactionMessageType int

const (
	SEND TransactionMessageType = iota
)

func ConstructTransactionListMessage(transactions []blockchain.Transaction) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(message.NODE)})
	byteBuffer.WriteByte(byte(message.TRANSACTION))
	byteBuffer.WriteByte(byte(SEND))
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(transactions)
	return byteBuffer.Bytes()
}