package node

import (
	"bytes"
	"encoding/gob"
	"log"

	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/proto"
)

// NodeMessageType is to indicate the specific type of message under NODE category
type NodeMessageType byte

const (
	Transaction NodeMessageType = iota
	BLOCK
	CLIENT
	CONTROL
	BlockchainSync
	// TODO: add more types
)

// BlockchainSyncMessage is a struct for blockchain sync message.
type BlockchainSyncMessage struct {
	BlockHeight int
	BlockHashes [][32]byte
}

// BlockchainSyncMessageType represents BlockchainSyncMessageType type.
type BlockchainSyncMessageType int

const (
	DONE BlockchainSyncMessageType = iota
	GetLastBlockHashes
	GetBlock
)

// TransactionMessageType representa the types of messages used for NODE/Transaction
type TransactionMessageType int

const (
	SEND TransactionMessageType = iota
	REQUEST
	UNLOCK
)

// BlockMessageType represents the types of messages used for NODE/BLOCK
type BlockMessageType int

const (
	SYNC BlockMessageType = iota
)

// The types of messages used for NODE/BLOCK
type ClientMessageType int

const (
	LookupUtxo ClientMessageType = iota
)

// The types of messages used for NODE/CONTROL
type ControlMessageType int

// ControlMessageType
const (
	STOP ControlMessageType = iota
)

// FetchUtxoMessage is the wrapper struct FetchUtxoMessage sent from client wallet.
type FetchUtxoMessage struct {
	Addresses [][20]byte
	Sender    p2p.Peer
}

// SerializeBlockchainSyncMessage serializes BlockchainSyncMessage.
func SerializeBlockchainSyncMessage(blockchainSyncMessage *BlockchainSyncMessage) []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(blockchainSyncMessage)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}

// DeserializeBlockchainSyncMessage deserializes BlockchainSyncMessage.
func DeserializeBlockchainSyncMessage(d []byte) (*BlockchainSyncMessage, error) {
	var blockchainSyncMessage BlockchainSyncMessage
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&blockchainSyncMessage)
	if err != nil {
		log.Panic(err)
	}
	return &blockchainSyncMessage, err
}

// ConstructUnlockToCommitOrAbortMessage constructs the unlock to commit or abort message that will be sent to leaders.
// This is for client.
func ConstructUnlockToCommitOrAbortMessage(txsAndProofs []*blockchain.Transaction) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(Transaction))
	byteBuffer.WriteByte(byte(UNLOCK))
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(txsAndProofs)
	return byteBuffer.Bytes()
}

// ConstructFetchUtxoMessage constructs the fetch utxo message that will be sent to Harmony network.
// this is for client.
func ConstructFetchUtxoMessage(sender p2p.Peer, addresses [][20]byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(CLIENT))
	byteBuffer.WriteByte(byte(LookupUtxo))

	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(FetchUtxoMessage{Addresses: addresses, Sender: sender})

	return byteBuffer.Bytes()
}

// ConstructTransactionListMessage constructs serialized transactions
func ConstructTransactionListMessage(transactions []*blockchain.Transaction) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(Transaction))
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

// ConstructBlockchainSyncMessage constructs Blockchain Sync Message.
func ConstructBlockchainSyncMessage(msgType BlockchainSyncMessageType, blockHash [32]byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(BlockchainSync))
	byteBuffer.WriteByte(byte(msgType))
	if msgType != GetLastBlockHashes {
		byteBuffer.Write(blockHash[:])
	}
	return byteBuffer.Bytes()
}

// GenerateBlockchainSyncMessage generates blockchain sync message.
func GenerateBlockchainSyncMessage(payload []byte) *BlockchainSyncMessage {
	dec := gob.NewDecoder(bytes.NewBuffer(payload))
	var res BlockchainSyncMessage
	dec.Decode(&res)
	return &res
}

// ConstructRequestTransactionsMessage constructs serialized transactions
func ConstructRequestTransactionsMessage(transactionIds [][]byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(Transaction))
	byteBuffer.WriteByte(byte(REQUEST))
	for _, txID := range transactionIds {
		byteBuffer.Write(txID)
	}
	return byteBuffer.Bytes()
}

// ConstructStopMessage constructs STOP message for node to stop
func ConstructStopMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(CONTROL))
	byteBuffer.WriteByte(byte(STOP))
	return byteBuffer.Bytes()
}

// ConstructBlocksSyncMessage constructs blocks sync message to send blocks to other nodes
func ConstructBlocksSyncMessage(blocks []blockchain.Block) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.NODE)})
	byteBuffer.WriteByte(byte(BLOCK))
	byteBuffer.WriteByte(byte(SYNC))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(blocks)
	return byteBuffer.Bytes()
}
