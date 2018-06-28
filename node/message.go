package node

import (
	"bytes"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/common"
)

// The specific types of message under NODE category
type NodeMessageType byte

const (
	TRANSACTION NodeMessageType = iota // TODO: Don't move this until the hack in client/message.go is resolved
	BLOCK
	CONTROL

	EXPERIMENT // Exist only for experiment setup
	// TODO: add more types
)

// The types of messages used for NODE/TRANSACTION
type TransactionMessageType int

const (
	SEND TransactionMessageType = iota
	REQUEST
	UNLOCK // The unlock to commit or abort message sent by the client to leaders.  TODO: Don't move this until the hack in client/message.go is resolved
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

// The types of messages used for NODE/EXPERIMENT
type ExperimentMessageType int

const (
	UTXO_REQUEST ExperimentMessageType = iota
	UTXO_RESPONSE
)

// Constructs serialized transactions
func ConstructTransactionListMessage(transactions []*blockchain.Transaction) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
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
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
	byteBuffer.WriteByte(byte(TRANSACTION))
	byteBuffer.WriteByte(byte(REQUEST))
	for _, txId := range transactionIds {
		byteBuffer.Write(txId)
	}
	return byteBuffer.Bytes()
}

// Constructs STOP message for node to stop
func ConstructStopMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
	byteBuffer.WriteByte(byte(CONTROL))
	byteBuffer.WriteByte(byte(STOP))
	return byteBuffer.Bytes()
}

// Constructs utxo response message with serialized utxoPool to return
func ConstructUtxoResponseMessage(pool blockchain.UTXOPool) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
	byteBuffer.WriteByte(byte(EXPERIMENT))
	byteBuffer.WriteByte(byte(UTXO_RESPONSE))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(pool)
	return byteBuffer.Bytes()
}

// Constructs utxo request message
func ConstructUtxoRequestMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
	byteBuffer.WriteByte(byte(EXPERIMENT))
	byteBuffer.WriteByte(byte(UTXO_REQUEST))
	return byteBuffer.Bytes()
}

// Constructs blocks sync message to send blocks to other nodes
func ConstructBlocksSyncMessage(blocks []blockchain.Block) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(common.NODE)})
	byteBuffer.WriteByte(byte(BLOCK))
	byteBuffer.WriteByte(byte(SYNC))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(blocks)
	return byteBuffer.Bytes()
}
