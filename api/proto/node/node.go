package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"

	"github.com/harmony-one/harmony/api/proto"
	peer "github.com/libp2p/go-libp2p-peer"
)

// MessageType is to indicate the specific type of message under Node category
type MessageType byte

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
	BlockHashes []common.Hash
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

// RoleType defines the role of the node
type RoleType int

// Type of roles of a node
const (
	ValidatorRole RoleType = iota
	ClientRole
)

func (r RoleType) String() string {
	switch r {
	case ValidatorRole:
		return "Validator"
	case ClientRole:
		return "Client"
	}
	return "Unknown"
}

// Info refers to Peer struct in p2p/peer.go
// this is basically a simplified version of Peer
// for network transportation
type Info struct {
	IP     string
	Port   string
	PubKey []byte
	Role   RoleType
	PeerID peer.ID // Peerstore ID
}

func (info Info) String() string {
	return fmt.Sprintf("Info:%v/%v=>%v", info.IP, info.Port, info.PeerID.Pretty())
}

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
