package node

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"

	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

// ProcessHeaderMessage verify and process Node/Header message into crosslink when it's valid
func (node *Node) ProcessHeaderMessage(msgPayload []byte) {
	var headers []*types.Header
	err := rlp.DecodeBytes(msgPayload, &headers)
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("Crosslink Headers Broadcast Unable to Decode")
		return
	}
	// TODO: add actual logic
}

// ProcessReceiptMessage store the receipts and merkle proof in local data store
func (node *Node) ProcessReceiptMessage(msgPayload []byte) {
	cxmsg := proto_node.CXReceiptsMessage{}
	if err := rlp.DecodeBytes(msgPayload, &cxmsg); err != nil {
		utils.Logger().Error().Err(err).Msg("[ProcessReceiptMessage] Unable to Decode message Payload")
		return
	}
	merkleProof := cxmsg.MKP
	myShardRoot := common.Hash{}

	var foundMyShard bool
	byteBuffer := bytes.NewBuffer([]byte{})
	if len(merkleProof.ShardID) == 0 {
		utils.Logger().Warn().Msg("[ProcessReceiptMessage] There is No non-empty destination shards")
		return
	} else {
		for j := 0; j < len(merkleProof.ShardID); j++ {
			sKey := make([]byte, 4)
			binary.BigEndian.PutUint32(sKey, merkleProof.ShardID[j])
			byteBuffer.Write(sKey)
			byteBuffer.Write(merkleProof.CXShardHash[j][:])
			if merkleProof.ShardID[j] == node.Consensus.ShardID {
				foundMyShard = true
				myShardRoot = merkleProof.CXShardHash[j]
			}
		}
	}

	if !foundMyShard {
		utils.Logger().Warn().Msg("[ProcessReceiptMessage] Not Found My Shard in CXReceipt Message")
		return
	}

	hash := crypto.Keccak256Hash(byteBuffer.Bytes())
	utils.Logger().Debug().Interface("hash", hash).Msg("[ProcessReceiptMessage] RootHash of the CXReceipts")
	// TODO chao: use crosslink from beacon sync to verify the hash

	cxReceipts := cxmsg.CXS
	sha := types.DeriveSha(cxReceipts)
	if sha != myShardRoot {
		utils.Logger().Warn().Interface("calculated", sha).Interface("got", myShardRoot).Msg("[ProcessReceiptMessage] Trie Root of CXReceipts Not Match")
		return
	}

	txs := types.Transactions{}
	inputData, _ := base64.StdEncoding.DecodeString("")
	gas, err := core.IntrinsicGas(inputData, false, true)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot calculate required gas")
		return
	}
	for _, cx := range cxReceipts {
		// TODO chao: add gas fee to incentivize
		tx := types.NewCrossShardTransaction(0, cx.To, cx.ToShardID, cx.ToShardID, cx.Amount, gas, nil, inputData, types.AdditionOnly)
		txs = append(txs, tx)
	}
	node.addPendingTransactions(txs)
}

// ProcessCrossShardTx verify and process cross shard transaction on destination shard
func (node *Node) ProcessCrossShardTx(blocks []*types.Block) {
	// TODO: add logic
}
