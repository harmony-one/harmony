package node

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/shard"
)

// BroadcastCXReceipts broadcasts cross shard receipts to correspoding
// destination shards
func (node *Node) BroadcastCXReceipts(newBlock *types.Block, lastCommits []byte) {

	//#### Read payload data from committed msg
	if len(lastCommits) <= 96 {
		utils.Logger().Debug().Int("lastCommitsLen", len(lastCommits)).Msg("[BroadcastCXReceipts] lastCommits Not Enough Length")
	}
	commitSig := make([]byte, 96)
	commitBitmap := make([]byte, len(lastCommits)-96)
	offset := 0
	copy(commitSig[:], lastCommits[offset:offset+96])
	offset += 96
	copy(commitBitmap[:], lastCommits[offset:])
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
		go node.BroadcastCXReceiptsWithShardID(newBlock, commitSig, commitBitmap, uint32(i))
	}
}

// BroadcastCXReceiptsWithShardID broadcasts cross shard receipts to given ToShardID
func (node *Node) BroadcastCXReceiptsWithShardID(block *types.Block, commitSig []byte, commitBitmap []byte, toShardID uint32) {
	myShardID := node.Consensus.ShardID
	utils.Logger().Info().Uint32("toShardID", toShardID).Uint32("myShardID", myShardID).Uint64("blockNum", block.NumberU64()).Msg("[BroadcastCXReceiptsWithShardID]")

	cxReceipts, err := node.Blockchain().ReadCXReceipts(toShardID, block.NumberU64(), block.Hash())
	if err != nil || len(cxReceipts) == 0 {
		utils.Logger().Info().Err(err).Uint32("ToShardID", toShardID).Int("numCXReceipts", len(cxReceipts)).Msg("[BroadcastCXReceiptsWithShardID] No ReadCXReceipts found")
		return
	}
	merkleProof, err := node.Blockchain().CXMerkleProof(toShardID, block)
	if err != nil {
		utils.Logger().Warn().Uint32("ToShardID", toShardID).Msg("[BroadcastCXReceiptsWithShardID] Unable to get merkleProof")
		return
	}

	groupID := nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(toShardID))
	utils.Logger().Info().Uint32("ToShardID", toShardID).Str("GroupID", string(groupID)).Msg("[BroadcastCXReceiptsWithShardID] ReadCXReceipts and MerkleProof Found")
	// TODO ek – limit concurrency
	go node.host.SendMessageToGroups([]nodeconfig.GroupID{groupID}, host.ConstructP2pMessage(byte(0), proto_node.ConstructCXReceiptsProof(cxReceipts, merkleProof, block.Header(), commitSig, commitBitmap)))
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

func (node *Node) verifyIncomingReceipts(block *types.Block) error {
	m := make(map[common.Hash]struct{})
	cxps := block.IncomingReceipts()
	for _, cxp := range cxps {
		// double spent
		if node.Blockchain().IsSpent(cxp) {
			return ctxerror.New("[verifyIncomingReceipts] Double Spent!")
		}
		hash := cxp.MerkleProof.BlockHash
		// duplicated receipts
		if _, ok := m[hash]; ok {
			return ctxerror.New("[verifyIncomingReceipts] Double Spent!")
		}
		m[hash] = struct{}{}

		for _, item := range cxp.Receipts {
			if item.ToShardID != node.Blockchain().ShardID() {
				return ctxerror.New("[verifyIncomingReceipts] Invalid ToShardID", "myShardID", node.Blockchain().ShardID(), "expectShardID", item.ToShardID)
			}
		}

		if err := node.Blockchain().Validator().ValidateCXReceiptsProof(cxp); err != nil {
			return ctxerror.New("[verifyIncomingReceipts] verification failed").WithCause(err)
		}
	}

	incomingReceiptHash := types.EmptyRootHash
	if len(cxps) > 0 {
		incomingReceiptHash = types.DeriveSha(cxps)
	}
	if incomingReceiptHash != block.Header().IncomingReceiptHash() {
		return ctxerror.New("[verifyIncomingReceipts] Invalid IncomingReceiptHash in block header")
	}

	return nil
}

// ProcessReceiptMessage store the receipts and merkle proof in local data store
func (node *Node) ProcessReceiptMessage(msgPayload []byte) {
	cxp := types.CXReceiptsProof{}
	if err := rlp.DecodeBytes(msgPayload, &cxp); err != nil {
		utils.Logger().Error().Err(err).Msg("[ProcessReceiptMessage] Unable to Decode message Payload")
		return
	}
	utils.Logger().Debug().Interface("cxp", cxp).Msg("[ProcessReceiptMessage] Add CXReceiptsProof to pending Receipts")
	// TODO: integrate with txpool
	node.AddPendingReceipts(&cxp)
}
