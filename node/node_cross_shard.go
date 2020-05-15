package node

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protonode "github.com/harmony-one/harmony/api/proto/protonode"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

// BroadcastCXReceipts broadcasts cross shard receipts to correspoding
// destination shards
func (node *Node) BroadcastCXReceipts(newBlock *types.Block) {
	commitSigAndBitmap := newBlock.GetCurrentCommitSig()
	//#### Read payload data from committed msg
	if len(commitSigAndBitmap) <= 96 {
		utils.Logger().Debug().Int("commitSigAndBitmapLen", len(commitSigAndBitmap)).Msg("[BroadcastCXReceipts] commitSigAndBitmap Not Enough Length")
	}
	commitSig := make([]byte, 96)
	commitBitmap := make([]byte, len(commitSigAndBitmap)-96)
	offset := 0
	copy(commitSig[:], commitSigAndBitmap[offset:offset+96])
	offset += 96
	copy(commitBitmap[:], commitSigAndBitmap[offset:])
	//#### END Read payload data from committed msg

	epoch := newBlock.Header().Epoch()
	shardingConfig := shard.Schedule.InstanceForEpoch(epoch)
	shardNum := int(shardingConfig.NumShards())
	myShardID := node.Consensus.ShardID
	utils.Logger().Info().Int("shardNum", shardNum).Uint32("myShardID", myShardID).Uint64("blockNum", newBlock.NumberU64()).Msg("[BroadcastCXReceipts]")

	for i := 0; i < shardNum; i++ {
		if i == int(myShardID) {
			continue
		}
		node.BroadcastCXReceiptsWithShardID(newBlock, commitSig, commitBitmap, uint32(i))
	}
}

// BroadcastCXReceiptsWithShardID broadcasts cross shard receipts to given ToShardID
func (node *Node) BroadcastCXReceiptsWithShardID(block *types.Block, commitSig []byte, commitBitmap []byte, toShardID uint32) {
	myShardID := node.Consensus.ShardID
	utils.Logger().Info().
		Uint32("toShardID", toShardID).
		Uint32("myShardID", myShardID).
		Uint64("blockNum", block.NumberU64()).
		Msg("[BroadcastCXReceiptsWithShardID]")

	cxReceipts, err := node.Blockchain().ReadCXReceipts(toShardID, block.NumberU64(), block.Hash())
	if err != nil || len(cxReceipts) == 0 {
		utils.Logger().Info().Uint32("ToShardID", toShardID).
			Int("numCXReceipts", len(cxReceipts)).
			Msg("[CXMerkleProof] No receipts found for the destination shard")
		return
	}

	merkleProof, err := node.Blockchain().CXMerkleProof(toShardID, block)
	if err != nil {
		utils.Logger().Warn().
			Uint32("ToShardID", toShardID).
			Msg("[BroadcastCXReceiptsWithShardID] Unable to get merkleProof")
		return
	}

	cxReceiptsProof := &types.CXReceiptsProof{
		Receipts:     cxReceipts,
		MerkleProof:  merkleProof,
		Header:       block.Header(),
		CommitSig:    commitSig,
		CommitBitmap: commitBitmap,
	}

	groupID := nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(toShardID))
	utils.Logger().Info().Uint32("ToShardID", toShardID).
		Str("GroupID", string(groupID)).
		Interface("cxp", cxReceiptsProof).
		Msg("[BroadcastCXReceiptsWithShardID] ReadCXReceipts and MerkleProof ready. Sending CX receipts...")
	// TODO ek – limit concurrency
	go node.host.SendMessageToGroups([]nodeconfig.GroupID{groupID},
		p2p.ConstructMessage(protonode.ConstructCXReceiptsProof(cxReceiptsProof)),
	)
}

// BroadcastMissingCXReceipts broadcasts missing cross shard receipts per request
func (node *Node) BroadcastMissingCXReceipts() {
	sendNextTime := []core.CxEntry{}
	it := node.CxPool.Pool().Iterator()
	for entry := range it.C {
		cxEntry := entry.(core.CxEntry)
		toShardID := cxEntry.ToShardID
		blk := node.Blockchain().GetBlockByHash(cxEntry.BlockHash)
		if blk == nil {
			continue
		}
		blockNum := blk.NumberU64()
		nextHeader := node.Blockchain().GetHeaderByNumber(blockNum + 1)
		if nextHeader == nil {
			sendNextTime = append(sendNextTime, cxEntry)
			continue
		}
		sig := nextHeader.LastCommitSignature()
		bitmap := nextHeader.LastCommitBitmap()
		node.BroadcastCXReceiptsWithShardID(blk, sig[:], bitmap, toShardID)
	}
	node.CxPool.Clear()
	// this should not happen or maybe happen for impatient user
	for _, entry := range sendNextTime {
		node.CxPool.Add(entry)
	}
}

var (
	errDoubleSpent = errors.New("[verifyIncomingReceipts] Double Spent")
)

func (node *Node) verifyIncomingReceipts(block *types.Block) error {
	m := make(map[common.Hash]struct{})
	cxps := block.IncomingReceipts()
	for _, cxp := range cxps {
		// double spent
		if node.Blockchain().IsSpent(cxp) {
			return errDoubleSpent
		}
		hash := cxp.MerkleProof.BlockHash
		// duplicated receipts
		if _, ok := m[hash]; ok {
			return errDoubleSpent
		}
		m[hash] = struct{}{}

		for _, item := range cxp.Receipts {
			if s := node.Blockchain().ShardID(); item.ToShardID != s {
				return errors.Errorf(
					"[verifyIncomingReceipts] Invalid ToShardID %d expectShardID %d",
					s, item.ToShardID,
				)
			}
		}

		if err := node.Blockchain().Validator().ValidateCXReceiptsProof(cxp); err != nil {
			return errors.Wrapf(err, "[verifyIncomingReceipts] verification failed")
		}
	}

	incomingReceiptHash := types.EmptyRootHash
	if len(cxps) > 0 {
		incomingReceiptHash = types.DeriveSha(cxps)
	}
	if incomingReceiptHash != block.Header().IncomingReceiptHash() {
		return errors.New("[verifyIncomingReceipts] Invalid IncomingReceiptHash in block header")
	}

	return nil
}

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
