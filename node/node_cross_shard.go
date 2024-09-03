package node

import (
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

// ProcessReceiptMessage store the receipts and merkle proof in local data store
func (node *Node) ProcessReceiptMessage(msgPayload []byte) {
	cxp := types.CXReceiptsProof{}
	if err := rlp.DecodeBytes(msgPayload, &cxp); err != nil {
		utils.Logger().Error().Err(err).
			Msg("[ProcessReceiptMessage] Unable to Decode message Payload")
		return
	}
	utils.Logger().Debug().Interface("cxp", cxp).
		Msg("[ProcessReceiptMessage] Add CXReceiptsProof to pending Receipts")
	// TODO: integrate with txpool
	node.AddPendingReceipts(&cxp)
}
