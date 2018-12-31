package node

import (
	"bytes"
	"encoding/gob"
	"log"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"

	"github.com/harmony-one/harmony-public/pkg/p2p"
	"github.com/harmony-one/harmony/proto"
)

// MessageType is to indicate the specific type of message under Node category
type MessageType byte

// ProtocolVersion is a constant defined as the version of the Harmony protocol
const (
	ProtocolVersion = 1
)

// Constant of the top level Message Type exchanged among nodes
const (
	Transaction MessageType = iota
	Block
	Client
	Control
	PING // node send ip/pki to register with leader
	PONG // node broadcast pubK
	// TODO: add more types
)

// BlockchainSyncMessage is a struct for blockchain sync message.
type BlockchainSyncMessage struct {
	BlockHeight int
	BlockHashes [][32]byte
}

// BlockchainSyncMessageType represents BlockchainSyncMessageType type.
type BlockchainSyncMessageType int

// Constant of blockchain sync-up message subtype
const (
	Done BlockchainSyncMessageType = iota
	GetLastBlockHashes
	GetBlock
)

// TransactionMessageType representa the types of messages used for Node/Transaction
type TransactionMessageType int

// Constant of transaction message subtype
const (
	Send TransactionMessageType = iota
	Request
	Unlock
)

// BlockMessageType represents the type of messages used for Node/Block
type BlockMessageType int

// Block sync message subtype
const (
	Sync BlockMessageType = iota
)

// ControlMessageType is the type of messages used for Node/Control
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

// ConstructTransactionListMessageAccount constructs serialized transactions in account model
func ConstructTransactionListMessageAccount(transactions types.Transactions) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Transaction))
	byteBuffer.WriteByte(byte(Send))

	txs, err := rlp.EncodeToBytes(transactions)
	if err != nil {
		log.Fatal(err)
		return []byte{} // TODO(RJ): better handle of the error
	}
	byteBuffer.Write(txs)
	return byteBuffer.Bytes()
}

// ConstructRequestTransactionsMessage constructs serialized transactions
func ConstructRequestTransactionsMessage(transactionIds [][]byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Transaction))
	byteBuffer.WriteByte(byte(Request))
	for _, txID := range transactionIds {
		byteBuffer.Write(txID)
	}
	return byteBuffer.Bytes()
}

// ConstructStopMessage constructs STOP message for node to stop
func ConstructStopMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Control))
	byteBuffer.WriteByte(byte(STOP))
	return byteBuffer.Bytes()
}

// ConstructBlocksSyncMessage constructs blocks sync message to send blocks to other nodes
func ConstructBlocksSyncMessage(blocks []*types.Block) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Block))
	byteBuffer.WriteByte(byte(Sync))

	blocksData, _ := rlp.EncodeToBytes(blocks)
	byteBuffer.Write(blocksData)
	return byteBuffer.Bytes()
}
