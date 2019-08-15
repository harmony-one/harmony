package node

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/harmony-one/harmony/core"

	"bytes"

	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"

	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
)

// ProcessHeaderMessage verify and process Node/Header message into crosslink when it's valid
func (node *Node) ProcessHeaderMessage(msgPayload []byte) {
	if node.NodeConfig.ShardID == 0 {

		var headers []*types.Header
		err := rlp.DecodeBytes(msgPayload, &headers)
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Msg("[ProcessingHeader] Crosslink Headers Broadcast Unable to Decode")
			return
		}

		// Try to reprocess all the pending cross links
		node.pendingClMutex.Lock()
		crossLinkHeadersToProcess := node.pendingCrossLinks
		node.pendingCrossLinks = []*types.Header{}
		node.pendingClMutex.Unlock()

		firstCrossLinkBlock := core.ShardingSchedule.FirstCrossLinkBlock()
		for _, header := range headers {
			if header.Number.Uint64() >= firstCrossLinkBlock {
				// Only process cross link starting from FirstCrossLinkBlock
				crossLinkHeadersToProcess = append(crossLinkHeadersToProcess, header)
			}
		}

		headersToQuque := []*types.Header{}

		for _, header := range crossLinkHeadersToProcess {
			exist, err := node.Blockchain().ReadCrossLink(header.ShardID, header.Number.Uint64(), false)
			if err == nil && exist != nil {
				utils.Logger().Debug().
					Msgf("[ProcessingHeader] Cross Link already exists, pass. Block num: %d", header.Number)
				continue
			}

			if header.Number.Uint64() > firstCrossLinkBlock { // Directly trust the first cross-link
				// Sanity check on the previous link with the new link
				previousLink, err := node.Blockchain().ReadCrossLink(header.ShardID, header.Number.Uint64()-1, false)
				if err != nil {
					previousLink, err = node.Blockchain().ReadCrossLink(header.ShardID, header.Number.Uint64()-1, true)
					if err != nil {
						headersToQuque = append(headersToQuque, header)
						continue
					}
				}

				err = node.VerifyCrosslinkHeader(previousLink.Header(), header)
				if err != nil {
					utils.Logger().Warn().
						Err(err).
						Msgf("[ProcessingHeader] Failed to verify new cross link header for shardID %d, blockNum %d", header.ShardID, header.Number)
					continue
				}
			}

			crossLink := types.NewCrossLink(header)
			utils.Logger().Debug().
				Msgf("[ProcessingHeader] committing for shardID %d, blockNum %d", header.ShardID, header.Number.Uint64())
			node.Blockchain().WriteCrossLinks(types.CrossLinks{crossLink}, true)
		}

		// Queue up the cross links that's in the future
		node.pendingClMutex.Lock()
		node.pendingCrossLinks = append(node.pendingCrossLinks, headersToQuque...)
		node.pendingClMutex.Unlock()
	}
}

// VerifyCrosslinkHeader verifies the header is valid against the prevHeader.
func (node *Node) VerifyCrosslinkHeader(prevHeader, header *types.Header) error {

	// TODO: add fork choice rule
	if prevHeader.Hash() != header.ParentHash {
		return ctxerror.New("[CrossLink] Invalid cross link header - parent hash mismatch", "shardID", header.ShardID, "blockNum", header.Number)
	}

	// Verify signature of the new cross link header
	shardState, err := node.Blockchain().ReadShardState(prevHeader.Epoch)
	committee := shardState.FindCommitteeByID(prevHeader.ShardID)

	if err != nil || committee == nil {
		return ctxerror.New("[CrossLink] Failed to read shard state for cross link header", "shardID", header.ShardID, "blockNum", header.Number).WithCause(err)
	}
	var committerKeys []*bls.PublicKey

	parseKeysSuccess := true
	for _, member := range committee.NodeList {
		committerKey := new(bls.PublicKey)
		err = member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			parseKeysSuccess = false
			break
		}
		committerKeys = append(committerKeys, committerKey)
	}
	if !parseKeysSuccess {
		return ctxerror.New("[CrossLink] cannot convert BLS public key", "shardID", header.ShardID, "blockNum", header.Number).WithCause(err)
	}

	if header.Number.Uint64() > 1 { // First block doesn't have last sig
		mask, err := bls_cosi.NewMask(committerKeys, nil)
		if err != nil {
			return ctxerror.New("cannot create group sig mask", "shardID", header.ShardID, "blockNum", header.Number).WithCause(err)
		}
		if err := mask.SetMask(header.LastCommitBitmap); err != nil {
			return ctxerror.New("cannot set group sig mask bits", "shardID", header.ShardID, "blockNum", header.Number).WithCause(err)
		}

		aggSig := bls.Sign{}
		err = aggSig.Deserialize(header.LastCommitSignature[:])
		if err != nil {
			return ctxerror.New("unable to deserialize multi-signature from payload").WithCause(err)
		}

		blockNumBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumBytes, header.Number.Uint64()-1)
		commitPayload := append(blockNumBytes, header.ParentHash[:]...)
		if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
			return ctxerror.New("Failed to verify the signature for cross link header ", "shardID", header.ShardID, "blockNum", header.Number)
		}
	}
	return nil
}

// ProposeCrossLinkDataForBeaconchain propose cross links for beacon chain new block
func (node *Node) ProposeCrossLinkDataForBeaconchain() (types.CrossLinks, error) {
	utils.Logger().Info().
		Uint64("blockNum", node.Blockchain().CurrentBlock().NumberU64()+1).
		Msg("Proposing cross links ...")
	curBlock := node.Blockchain().CurrentBlock()
	numShards := core.ShardingSchedule.InstanceForEpoch(curBlock.Header().Epoch).NumShards()

	shardCrossLinks := make([]types.CrossLinks, numShards)

	firstCrossLinkBlock := core.ShardingSchedule.FirstCrossLinkBlock()

	for i := 0; i < int(numShards); i++ {
		curShardID := uint32(i)
		lastLink, err := node.Blockchain().ReadShardLastCrossLink(curShardID)

		lastLinkblockNum := big.NewInt(int64(firstCrossLinkBlock))
		blockNumoffset := 0
		if err == nil && lastLink != nil {
			blockNumoffset = 1
			lastLinkblockNum = lastLink.BlockNum()
		}

		for true {
			link, err := node.Blockchain().ReadCrossLink(curShardID, lastLinkblockNum.Uint64()+uint64(blockNumoffset), true)
			if err != nil || link == nil {
				break
			}

			if link.BlockNum().Uint64() > firstCrossLinkBlock {
				err := node.VerifyCrosslinkHeader(lastLink.Header(), link.Header())
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("[CrossLink] Failed verifying temp cross link %d", link.BlockNum().Uint64())
					break
				}
				lastLink = link
			}
			shardCrossLinks[i] = append(shardCrossLinks[i], *link)

			blockNumoffset++
		}
	}

	crossLinksToPropose := types.CrossLinks{}
	for _, crossLinks := range shardCrossLinks {
		crossLinksToPropose = append(crossLinksToPropose, crossLinks...)
	}
	if len(crossLinksToPropose) != 0 {
		crossLinksToPropose.Sort()

		return crossLinksToPropose, nil
	}
	return types.CrossLinks{}, errors.New("No cross link to propose")
}

// ProcessReceiptMessage store the receipts and merkle proof in local data store
func (node *Node) ProcessReceiptMessage(msgPayload []byte) {
	cxmsg := proto_node.CXReceiptsMessage{}
	if err := rlp.DecodeBytes(msgPayload, &cxmsg); err != nil {
		utils.Logger().Error().Err(err).Msg("[ProcessReceiptMessage] Unable to Decode message Payload")
		return
	}
	merkleProof := cxmsg.MerkleProof
	myShardRoot := common.Hash{}

	var foundMyShard bool
	byteBuffer := bytes.NewBuffer([]byte{})
	if len(merkleProof.ShardIDs) == 0 {
		utils.Logger().Warn().Msg("[ProcessReceiptMessage] There is No non-empty destination shards")
		return
	}

	// Find receipts with my shard as destination
	for j := 0; j < len(merkleProof.ShardIDs); j++ {
		sKey := make([]byte, 4)
		binary.BigEndian.PutUint32(sKey, merkleProof.ShardIDs[j])
		byteBuffer.Write(sKey)
		byteBuffer.Write(merkleProof.CXShardHashes[j][:])
		if merkleProof.ShardIDs[j] == node.Consensus.ShardID {
			foundMyShard = true
			myShardRoot = merkleProof.CXShardHashes[j]
		}
	}

	if !foundMyShard {
		utils.Logger().Warn().Msg("[ProcessReceiptMessage] Not Found My Shard in CXReceipt Message")
		return
	}

	// Check whether the receipts matches the receipt merkle root
	receiptsForMyShard := cxmsg.Receipts
	sha := types.DeriveSha(receiptsForMyShard)
	if sha != myShardRoot {
		utils.Logger().Warn().Interface("calculated", sha).Interface("got", myShardRoot).Msg("[ProcessReceiptMessage] Trie Root of ReadCXReceipts Not Match")
		return
	}

	if len(receiptsForMyShard) == 0 {
		return
	}

	sourceShardID := merkleProof.ShardID
	sourceBlockNum := merkleProof.BlockNum
	sourceBlockHash := merkleProof.BlockHash
	// TODO: check message signature is from the nodes of source shard.
	node.Blockchain().WriteCXReceipts(sourceShardID, sourceBlockNum.Uint64(), sourceBlockHash, receiptsForMyShard, true)

	// Check merkle proof with crosslink of the source shard
	hash := crypto.Keccak256Hash(byteBuffer.Bytes())
	utils.Logger().Debug().Interface("hash", hash).Msg("[ProcessReceiptMessage] RootHash of the CXReceipts")
	// TODO chao: use crosslink from beacon sync to verify the hash

	node.AddPendingReceipts(&cxmsg)
}

// ProcessCrossShardTx verify and process cross shard transaction on destination shard
func (node *Node) ProcessCrossShardTx(blocks []*types.Block) {
	// TODO: add logic
}
