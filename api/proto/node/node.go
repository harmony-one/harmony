package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	peer "github.com/libp2p/go-libp2p-peer"
)

// MessageType is to indicate the specific type of message under Node category
type MessageType byte

// Constant of the top level Message Type exchanged among nodes
const (
	Transaction MessageType = iota
	Block
	Client
	_          // used to be Control
	PING       // node send ip/pki to register with leader
	ShardState // Deprecated
	Staking
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

	CrossLink      // used for crosslink from beacon chain to shard chain
	Receipt        // cross-shard transaction receipts
	SlashCandidate // A report of a double-signing event
)

var (
	// B suffix means Byte
	nodeB      = byte(proto.Node)
	blockB     = byte(Block)
	slashB     = byte(SlashCandidate)
	txnB       = byte(Transaction)
	sendB      = byte(Send)
	stakingB   = byte(Staking)
	syncB      = byte(Sync)
	crossLinkB = byte(CrossLink)
	receiptB   = byte(Receipt)
	// H suffix means header
	slashH           = []byte{nodeB, blockB, slashB}
	transactionListH = []byte{nodeB, txnB, sendB}
	stakingTxnListH  = []byte{nodeB, stakingB, sendB}
	syncH            = []byte{nodeB, blockB, syncB}
	crossLinkH       = []byte{nodeB, blockB, crossLinkB}
	cxReceiptH       = []byte{nodeB, blockB, receiptB}
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
	byteBuffer := bytes.NewBuffer(transactionListH)
	txs, err := rlp.EncodeToBytes(transactions)
	if err != nil {
		log.Fatal(err)
		return []byte{} // TODO(RJ): better handle of the error
	}
	byteBuffer.Write(txs)
	return byteBuffer.Bytes()
}

// ConstructStakingTransactionListMessageAccount constructs serialized staking transactions in account model
func ConstructStakingTransactionListMessageAccount(
	transactions staking.StakingTransactions,
) []byte {
	byteBuffer := bytes.NewBuffer(stakingTxnListH)
	txs, err := rlp.EncodeToBytes(transactions)
	if err != nil {
		log.Fatal(err)
		return []byte{} // TODO(RJ): better handle of the error
	}
	byteBuffer.Write(txs)
	return byteBuffer.Bytes()
}

// ConstructBlocksSyncMessage constructs blocks sync message to send blocks to other nodes
func ConstructBlocksSyncMessage(blocks []*types.Block) []byte {
	byteBuffer := bytes.NewBuffer(syncH)
	blocksData, _ := rlp.EncodeToBytes(blocks)
	byteBuffer.Write(blocksData)
	return byteBuffer.Bytes()
}

// ConstructSlashMessage ..
func ConstructSlashMessage(witnesses slash.Records) []byte {
	byteBuffer := bytes.NewBuffer(slashH)
	slashData, _ := rlp.EncodeToBytes(witnesses)
	byteBuffer.Write(slashData)
	return byteBuffer.Bytes()
}

// ConstructCrossLinkMessage constructs cross link message to send to beacon chain
func ConstructCrossLinkMessage(bc engine.ChainReader, headers []*block.Header) []byte {
	byteBuffer := bytes.NewBuffer(crossLinkH)
	crosslinks := []types.CrossLink{}
	for _, header := range headers {
		if header.Number().Uint64() <= 1 || !bc.Config().IsCrossLink(header.Epoch()) {
			continue
		}
		parentHeader := bc.GetHeaderByHash(header.ParentHash())
		if parentHeader == nil {
			continue
		}
		epoch := parentHeader.Epoch()
		crosslinks = append(crosslinks, types.NewCrossLink(header, epoch))
	}
	crosslinksData, _ := rlp.EncodeToBytes(crosslinks)
	byteBuffer.Write(crosslinksData)
	return byteBuffer.Bytes()
}

// ConstructCXReceiptsProof constructs cross shard receipts and related proof including
// merkle proof, blockHeader and  commitSignatures
func ConstructCXReceiptsProof(cxReceiptsProof *types.CXReceiptsProof) []byte {
	byteBuffer := bytes.NewBuffer(cxReceiptH)
	by, err := rlp.EncodeToBytes(cxReceiptsProof)
	if err != nil {
		const msg = "[ConstructCXReceiptsProof] Encode CXReceiptsProof Error"
		utils.Logger().Error().Err(err).Msg(msg)
		return []byte{}
	}
	byteBuffer.Write(by)
	return byteBuffer.Bytes()
}
