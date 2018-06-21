package node

import (
	"bytes"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/common"
)

// The types of messages used for NODE/TRANSACTION
type TransactionMessageType int
const (
	SEND TransactionMessageType = iota
	REQUEST
)

// The types of messages used for NODE/CONTROL
type ControlMessageType int
const (
	STOP ControlMessageType = iota
)

//ConstructTransactionListMessage constructs serialized transactions
func ConstructTransactionListMessage(transactions []*blockchain.Transaction) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
	byteBuffer.WriteByte(byte(common.TRANSACTION))
	byteBuffer.WriteByte(byte(SEND))
	encoder := gob.NewEncoder(byteBuffer)
	// Copy over the tx data
	txs := make([]blockchain.Transaction, len(transactions))
	for i := range txs {
		txs[i] = *transactions[i]
	}
	encoder.Encode(txs)
	return byteBuffer.Bytes()
}

//ConstructTransactionListMessage constructs serialized transactions
func ConstructRequestTransactionsMessage(transactionIds [][]byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
	byteBuffer.WriteByte(byte(common.TRANSACTION))
	byteBuffer.WriteByte(byte(REQUEST))
	for _, txId := range transactionIds {
		byteBuffer.Write(txId)
	}
	return byteBuffer.Bytes()
}

//ConstructStopMessage is  STOP message
func ConstructStopMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
	byteBuffer.WriteByte(byte(common.CONTROL))
	byteBuffer.WriteByte(byte(STOP))
	return byteBuffer.Bytes()
}
