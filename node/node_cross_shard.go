package node

import (
	"github.com/ethereum/go-ethereum/rlp"

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
	// TODO: add logic
}

// ProcessCrossShardTx verify and process cross shard transaction on destination shard
func (node *Node) ProcessCrossShardTx(blocks []*types.Block) {
	// TODO: add logic
}
