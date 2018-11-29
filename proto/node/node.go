package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"

	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/proto"
)

// NodeMessageType is to indicate the specific type of message under Node category
type NodeMessageType byte

const (
	PROTOCOL_VERSION = 1
)

const (
	Transaction NodeMessageType = iota
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

const (
	Done BlockchainSyncMessageType = iota
	GetLastBlockHashes
	GetBlock
)

// TransactionMessageType representa the types of messages used for Node/Transaction
type TransactionMessageType int

const (
	Send TransactionMessageType = iota
	Request
	Unlock
)

// BlockMessageType represents the types of messages used for Node/Block
type BlockMessageType int

const (
	Sync BlockMessageType = iota
)

// The types of messages used for Node/Block
type ClientMessageType int

const (
	LookupUtxo ClientMessageType = iota
)

// The types of messages used for Node/Control
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
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Transaction))
	byteBuffer.WriteByte(byte(Unlock))
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(txsAndProofs)
	return byteBuffer.Bytes()
}

// ConstructFetchUtxoMessage constructs the fetch utxo message that will be sent to Harmony network.
// this is for client.
func ConstructFetchUtxoMessage(sender p2p.Peer, addresses [][20]byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Client))
	byteBuffer.WriteByte(byte(LookupUtxo))

	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(FetchUtxoMessage{Addresses: addresses, Sender: sender})

	return byteBuffer.Bytes()
}

// ConstructTransactionListMessage constructs serialized transactions
func ConstructTransactionListMessage(transactions []*blockchain.Transaction) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Transaction))
	byteBuffer.WriteByte(byte(Send))
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

// ConstructTransactionListMessageAccount constructs serialized transactions in account model
func ConstructTransactionListMessageAccount(transactions types.Transactions) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Transaction))
	byteBuffer.WriteByte(byte(Send))

	txs, err := rlp.EncodeToBytes(transactions)
	if err != nil {
		fmt.Errorf("ERROR RLP %s", err)
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
func ConstructBlocksSyncMessage(blocks []blockchain.Block) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(Block))
	byteBuffer.WriteByte(byte(Sync))
	encoder := gob.NewEncoder(byteBuffer)

	encoder.Encode(blocks)
	return byteBuffer.Bytes()
}
