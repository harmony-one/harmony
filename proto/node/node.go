package node

import (
	"bytes"
	"encoding/gob"

	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/proto"
)

// The specific types of message under NODE category
type NodeMessageType byte

const (
	TRANSACTION NodeMessageType = iota
	BLOCK
	CONTROL
	// TODO: add more types
)

// The types of messages used for NODE/TRANSACTION
type TransactionMessageType int

const (
	SEND TransactionMessageType = iota
	REQUEST
	UNLOCK
)

// The types of messages used for NODE/BLOCK
type BlockMessageType int

const (
	SYNC BlockMessageType = iota
)

// The types of messages used for NODE/CONTROL
type ControlMessageType int

const (
	STOP ControlMessageType = iota
)

// Constructs serialized transactions
func ConstructTransactionListMessage(transactions []*blockchain.Transaction) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(TRANSACTION))
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

// Constructs serialized transactions
func ConstructRequestTransactionsMessage(transactionIds [][]byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(TRANSACTION))
	byteBuffer.WriteByte(byte(REQUEST))
	for _, txId := range transactionIds {
		byteBuffer.Write(txId)
	}
	return byteBuffer.Bytes()
}

// Constructs STOP message for node to stop
func ConstructStopMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(CONTROL))
	byteBuffer.WriteByte(byte(STOP))
	return byteBuffer.Bytes()
}

// Constructs blocks sync message to send blocks to other nodes
func ConstructBlocksSyncMessage(blocks []blockchain.Block) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(BLOCK))
	byteBuffer.WriteByte(byte(SYNC))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(blocks)
	return byteBuffer.Bytes()
}
