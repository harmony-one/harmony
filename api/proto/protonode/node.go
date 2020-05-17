package protonode

import (
	"bytes"
	"log"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
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

// TransactionMessageType representa the types of messages used for Node/Transaction
type TransactionMessageType int

// Constant of transaction message subtype
const (
	Send TransactionMessageType = iota
	Unlock
)

// BlockMessageType represents the type of messages used for Node/Block
type BlockMessageType int

// Block sync message subtype
const (
	Sync           BlockMessageType = iota
	CrossLink                       // used for crosslink from beacon chain to shard chain
	Receipt                         // cross-shard transaction receipts
	SlashCandidate                  // A report of a double-signing event
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
	crosslinks := []*types.CrossLink{}
	for _, header := range headers {
		if header.Number().Uint64() <= 1 || !bc.Config().IsCrossLink(header.Epoch()) {
			continue
		}
		parentHeader := bc.GetHeaderByHash(header.ParentHash())
		if parentHeader == nil {
			continue
		}
		crosslinks = append(crosslinks, types.NewCrossLink(header, parentHeader))
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
