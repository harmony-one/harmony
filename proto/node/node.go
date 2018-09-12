package node

import (
	"bytes"
	"encoding/gob"

	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/proto"
)

// The specific types of message under NODE category
type NodeMessageType byte

const (
	TRANSACTION NodeMessageType = iota
	BLOCK
	CLIENT
	CONTROL
	BLOCKCHAIN_SYNC
	// TODO: add more types
)

type BlockchainSyncMessage struct {
	msgType   BlockchainSyncMessageType
	blockHash [32]byte
	block     *blockchain.Block
}
type BlockchainSyncMessageType int

const (
	DONE BlockchainSyncMessageType = iota
	GET_LAST_BLOCK_HASH
	GET_BLOCK
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

// The types of messages used for NODE/BLOCK
type ClientMessageType int

const (
	LOOKUP_UTXO ClientMessageType = iota
)

// The types of messages used for NODE/CONTROL
type ControlMessageType int

const (
	STOP ControlMessageType = iota
)

// The wrapper struct FetchUtxoMessage sent from client wallet
type FetchUtxoMessage struct {
	Addresses [][20]byte
	Sender    p2p.Peer
}

// [client] Constructs the unlock to commit or abort message that will be sent to leaders
func ConstructUnlockToCommitOrAbortMessage(txsAndProofs []*blockchain.Transaction) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(TRANSACTION))
	byteBuffer.WriteByte(byte(UNLOCK))
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(txsAndProofs)
	return byteBuffer.Bytes()
}

// [client] Constructs the fetch utxo message that will be sent to Harmony network
func ConstructFetchUtxoMessage(sender p2p.Peer, addresses [][20]byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(CLIENT))
	byteBuffer.WriteByte(byte(LOOKUP_UTXO))

	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(FetchUtxoMessage{Addresses: addresses, Sender: sender})

	return byteBuffer.Bytes()
}

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
	err := encoder.Encode(txs)
	if err != nil {
		return []byte{} // TODO(RJ): better handle of the error
	}
	return byteBuffer.Bytes()
}

// Constructs Blockchain Sync Message.
func ConstructBlockchainSyncMessage(msgType BlockchainSyncMessageType, blockHash [32]byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(BLOCKCHAIN_SYNC))
	byteBuffer.WriteByte(byte(msgType))
	byteBuffer.Write(blockHash[:])
	return byteBuffer.Bytes()
}

func GenerateBlockchainSyncMessage(payload []byte) *BlockchainSyncMessage {
	dec := gob.NewDecoder(bytes.NewBuffer(payload))
	var res BlockchainSyncMessage
	dec.Decode(&res)
	return &res
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
