package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	peer "github.com/libp2p/go-libp2p-peer"

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
	_    // used to be Control
	PING // node send ip/pki to register with leader
	PONG // node broadcast pubK
	ShardState
)

// BlockchainSyncMessage is a struct for blockchain sync message.
type BlockchainSyncMessage struct {
	BlockHeight int
	BlockHashes []common.Hash
}

// CXReceiptsMessage carrys the cross shard receipts and merkle proof
type CXReceiptsMessage struct {
	CXS types.CXReceipts
	MKP *types.CXMerkleProof
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

	Header  // used for crosslink from beacon chain to shard chain
	Receipt // cross-shard transaction receipts
)

// SerializeBlockchainSyncMessage serializes BlockchainSyncMessage.
func SerializeBlockchainSyncMessage(blockchainSyncMessage *BlockchainSyncMessage) []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(blockchainSyncMessage)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to serialize blockchain sync message")
	}
	return result.Bytes()
}

// DeserializeBlockchainSyncMessage deserializes BlockchainSyncMessage.
func DeserializeBlockchainSyncMessage(d []byte) (*BlockchainSyncMessage, error) {
	var blockchainSyncMessage BlockchainSyncMessage
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&blockchainSyncMessage)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to deserialize blockchain sync message")
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

// ConstructBlocksSyncMessage constructs blocks sync message to send blocks to other nodes
func ConstructBlocksSyncMessage(blocks []*types.Block) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Block))
	byteBuffer.WriteByte(byte(Sync))

	blocksData, _ := rlp.EncodeToBytes(blocks)
	byteBuffer.Write(blocksData)
	return byteBuffer.Bytes()
}

// ConstructCrossLinkHeadersMessage constructs cross link header message to send to beacon chain
func ConstructCrossLinkHeadersMessage(headers []*types.Header) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Block))
	byteBuffer.WriteByte(byte(Header))

	headersData, _ := rlp.EncodeToBytes(headers)
	byteBuffer.Write(headersData)
	return byteBuffer.Bytes()
}

// ConstructEpochShardStateMessage contructs epoch shard state message
func ConstructEpochShardStateMessage(epochShardState types.EpochShardState) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(ShardState))

	encoder := gob.NewEncoder(byteBuffer)
	err := encoder.Encode(epochShardState)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[ConstructEpochShardStateMessage] Encode")
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
		utils.Logger().Error().Err(err).Msg("[GetEpochShardStateFromMessage] Decode")
		return nil, fmt.Errorf("Decode epoch shard state Error")
	}

	return epochShardState, nil
}

// ConstructCXReceiptsMessage constructs cross shard receipts and merkle proof
func ConstructCXReceiptsMessage(cxs types.CXReceipts, mkp *types.CXMerkleProof) []byte {
	msg := &CXReceiptsMessage{CXS: cxs, MKP: mkp}

	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Block))
	byteBuffer.WriteByte(byte(Receipt))
	by, err := rlp.EncodeToBytes(msg)

	if err != nil {
		log.Fatal(err)
		return []byte{}
	}
	byteBuffer.Write(by)
	return byteBuffer.Bytes()
}
