package node

import (
	"bytes"
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
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

// VerifyBlockCrossLinks verifies the cross links of the block
func (node *Node) VerifyBlockCrossLinks(block *types.Block) error {
	if len(block.Header().CrossLinks()) == 0 {
		utils.Logger().Debug().Msgf("[CrossLinkVerification] Zero CrossLinks in the header")
		return nil
	}
	crossLinks := &types.CrossLinks{}
	err := rlp.DecodeBytes(block.Header().CrossLinks(), crossLinks)
	if err != nil {
		return ctxerror.New("[CrossLinkVerification] failed to decode cross links",
			"blockHash", block.Hash(),
			"crossLinks", len(block.Header().CrossLinks()),
		).WithCause(err)
	}

	if !crossLinks.IsSorted() {
		return ctxerror.New("[CrossLinkVerification] cross links are not sorted",
			"blockHash", block.Hash(),
			"crossLinks", len(block.Header().CrossLinks()),
		)
	}

	for _, crossLink := range *crossLinks {
		cl, err := node.Blockchain().ReadCrossLink(crossLink.ShardID(), crossLink.BlockNum())
		if err == nil && cl != nil {
			if !bytes.Equal(cl.Serialize(), crossLink.Serialize()) {
				return ctxerror.New("[CrossLinkVerification] Double signed crossLink",
					"blockHash", block.Hash(),
					"Previous committed crossLink", cl,
					"crossLink", crossLink,
				)
			}
			continue
		}
		if err = node.VerifyCrossLink(crossLink); err != nil {
			return ctxerror.New("cannot VerifyBlockCrossLinks",
				"blockHash", block.Hash(),
				"blockNum", block.Number(),
				"crossLinkShard", crossLink.ShardID(),
				"crossLinkBlock", crossLink.BlockNum(),
				"numTx", len(block.Transactions()),
			).WithCause(err)
		}
	}
	return nil
}

// ProcessCrossLinkMessage verify and process Node/CrossLink message into crosslink when it's valid
func (node *Node) ProcessCrossLinkMessage(msgPayload []byte) {
	// TODO: non-leader in beaconchain doesn't need process crosslink message, but still need to monitor leader's behavior
	if node.NodeConfig.ShardID == 0 && node.Consensus.IsLeader() {
		utils.Logger().Debug().Msgf("[ProcessingCrossLink] leader is processing...")
		var crosslinks []types.CrossLink
		err := rlp.DecodeBytes(msgPayload, &crosslinks)
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Msg("[ProcessingCrossLink] Crosslink Message Broadcast Unable to Decode")
			return
		}

		candidates := []types.CrossLink{}
		utils.Logger().Debug().
			Msgf("[ProcessingCrossLink] Crosslink going to propose: %d", len(crosslinks))

		for i, cl := range crosslinks {
			if cl.Number() == nil || cl.Epoch().Cmp(node.Blockchain().Config().CrossLinkEpoch) < 0 {
				utils.Logger().Debug().
					Msgf("[ProcessingCrossLink] Crosslink %d skipped: %v", i, cl)
				continue
			}
			exist, err := node.Blockchain().ReadCrossLink(cl.ShardID(), cl.Number().Uint64())
			if err == nil && exist != nil {
				// TODO: leader add double sign checking
				utils.Logger().Debug().
					Msgf("[ProcessingCrossLink] Cross Link already exists, pass. Block num: %d, shardID %d", cl.Number(), cl.ShardID())
				continue
			}

			err = node.VerifyCrossLink(cl)
			if err != nil {
				utils.Logger().Error().
					Err(err).
					Msgf("[ProcessingCrossLink] Failed to verify new cross link for shardID %d, blockNum %d", cl.ShardID(), cl.Number())
				continue
			}
			candidates = append(candidates, cl)
			utils.Logger().Debug().
				Msgf("[ProcessingCrossLink] committing for shardID %d, blockNum %d", cl.ShardID(), cl.Number().Uint64())
		}
		Len, err := node.Blockchain().AddLastPendingCrossLinks(candidates)
		utils.Logger().Error().Err(err).
			Msgf("[ProcessingCrossLink] add pending crosslinks,  total pending: %d", Len)
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

// VerifyCrossLink verifies the header is valid against the prevHeader.
func (node *Node) VerifyCrossLink(cl types.CrossLink) error {

	// TODO: add fork choice rule

	// Verify signature of the new cross link header
	// TODO: check whether to recalculate shard state
	shardState, err := node.Blockchain().ReadShardState(cl.Epoch())
	committee := shardState.FindCommitteeByID(cl.ShardID())

	if err != nil || committee == nil {
		return ctxerror.New("[CrossLink] Failed to read shard state for cross link", "shardID", cl.ShardID(), "blockNum", cl.BlockNum()).WithCause(err)
	}
	var committerKeys []*bls.PublicKey

	parseKeysSuccess := true
	for _, member := range committee.Slots {
		committerKey := new(bls.PublicKey)
		err = member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			parseKeysSuccess = false
			break
		}
		committerKeys = append(committerKeys, committerKey)
	}
	if !parseKeysSuccess {
		return ctxerror.New("[CrossLink] cannot convert BLS public key", "shardID", cl.ShardID(), "blockNum", cl.BlockNum()).WithCause(err)
	}

	if cl.BlockNum() > 1 { // First block doesn't have last sig
		mask, err := bls_cosi.NewMask(committerKeys, nil)
		if err != nil {
			return ctxerror.New("cannot create group sig mask", "shardID", cl.ShardID(), "blockNum", cl.BlockNum()).WithCause(err)
		}
		if err := mask.SetMask(cl.Bitmap()); err != nil {
			return ctxerror.New("cannot set group sig mask bits", "shardID", cl.ShardID(), "blockNum", cl.BlockNum()).WithCause(err)
		}

		aggSig := bls.Sign{}
		sig := cl.Signature()
		err = aggSig.Deserialize(sig[:])
		if err != nil {
			return ctxerror.New("unable to deserialize multi-signature from payload").WithCause(err)
		}

		parentHash := cl.ParentHash()
		blockNumBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumBytes, cl.BlockNum())
		commitPayload := append(blockNumBytes, parentHash[:]...)
		if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
			return ctxerror.New("Failed to verify the signature for cross link", "shardID", cl.ShardID(), "blockNum", cl.BlockNum())
		}
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
