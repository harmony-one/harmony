package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
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
	ShardState
	// TODO: add more types
)

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

// ConstructPingMessage contructs ping message from node to leader
func (p PingMessageType) ConstructPingMessage() []byte {
	siz := p.XXX_Size()
	b := make([]byte, 0, siz)
	ret, err := p.XXX_Marshal(b, true)
	if err != nil {
		return nil
	}
	return ret
}

// ConstructPongMessage contructs pong message from leader to node
func (p PongMessageType) ConstructPongMessage() []byte {
	siz := p.XXX_Size()
	b := make([]byte, 0, siz)
	ret, err := p.XXX_Marshal(b, true)
	if err != nil {
		return nil
	}
	return ret
}

// SerializeBlockchainSyncMessage serializes BlockchainSyncMessage.
func SerializeBlockchainSyncMessage(blockchainSyncMessage *BlockchainSyncMessage) []byte {
	siz := blockchainSyncMessage.XXX_Size()
	b := make([]byte, 0, siz)
	ret, err := blockchainSyncMessage.XXX_Marshal(b, true)
	if err != nil {
		return nil
	}
	return ret
}

// DeserializeBlockchainSyncMessage deserializes BlockchainSyncMessage.
func DeserializeBlockchainSyncMessage(d []byte) (*BlockchainSyncMessage, error) {
	bsm := BlockchainSyncMessage{}
	err := bsm.XXX_Unmarshal(d)
	if err != nil {
		return nil, err
	}
	return &bsm, nil
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

// ConstructEpochShardStateMessage contructs epoch shard state message
func ConstructEpochShardStateMessage(epochShardState types.EpochShardState) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(ShardState))

	encoder := gob.NewEncoder(byteBuffer)
	err := encoder.Encode(epochShardState)
	if err != nil {
		utils.GetLogInstance().Error("[ConstructEpochShardStateMessage] Encode", "error", err)
		return nil
	}
	return byteBuffer.Bytes()
}

// DeserializeEpochShardStateFromMessage deserializes the shard state Message from bytes payload
func DeserializeEpochShardStateFromMessage(payload []byte) (*types.EpochShardState, error) {
	epochShardState := new(types.EpochShardState)

	r := bytes.NewBuffer(payload)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(epochShardState)

	if err != nil {
		utils.GetLogInstance().Error("[GetEpochShardStateFromMessage] Decode", "error", err)
		return nil, fmt.Errorf("Decode epoch shard state Error")
	}

	return epochShardState, nil
}
