package node

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/core"
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
		utils.Logger().Debug().
			Msgf("[ProcessingHeader] number of crosslink headers to propose %d, firstCrossLinkBlock %d", len(crossLinkHeadersToProcess), firstCrossLinkBlock)

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
						utils.Logger().Debug().Err(err).
							Msg("[ProcessingHeader] ReadCrossLink cannot read previousLink")
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

func (node *Node) verifyIncomingReceipts(block *types.Block) error {
	m := make(map[common.Hash]bool)
	cxps := block.IncomingReceipts()
	for _, cxp := range cxps {
		if err := cxp.IsValidCXReceiptsProof(); err != nil {
			return ctxerror.New("[verifyIncomingReceipts] verification failed").WithCause(err)
		}
		if node.Blockchain().IsSpent(cxp) {
			return ctxerror.New("[verifyIncomingReceipts] Double Spent!")
		}
		hash := cxp.MerkleProof.BlockHash
		// ignore duplicated receipts
		if _, ok := m[hash]; ok {
			return ctxerror.New("[verifyIncomingReceipts] Double Spent!")
		}
		m[hash] = true

		if err := node.compareCrosslinkWithReceipts(cxp); err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) compareCrosslinkWithReceipts(cxp *types.CXReceiptsProof) error {
	var hash, outgoingReceiptHash common.Hash

	shardID := cxp.MerkleProof.ShardID
	blockNum := cxp.MerkleProof.BlockNum.Uint64()
	beaconChain := node.Beaconchain()
	if shardID == 0 {
		block := beaconChain.GetBlockByNumber(blockNum)
		if block == nil {
			return ctxerror.New("[compareCrosslinkWithReceipts] Cannot get beaconchain header", "blockNum", blockNum, "shardID", shardID)
		}
		hash = block.Hash()
		outgoingReceiptHash = block.OutgoingReceiptHash()
	} else {
		crossLink, err := beaconChain.ReadCrossLink(shardID, blockNum, false)
		if err != nil {
			return ctxerror.New("[compareCrosslinkWithReceipts] Cannot get crosslink", "blockNum", blockNum, "shardID", shardID).WithCause(err)
		}
		hash = crossLink.ChainHeader.Hash()
		outgoingReceiptHash = crossLink.ChainHeader.OutgoingReceiptHash
	}
	// verify the source block hash is from a finalized block
	if hash == cxp.MerkleProof.BlockHash && outgoingReceiptHash == cxp.MerkleProof.CXReceiptHash {
		return nil
	}
	return ErrCrosslinkVerificationFail
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
				if lastLink == nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("[CrossLink] Haven't received the first cross link %d", link.BlockNum().Uint64())
				} else {
					err := node.VerifyCrosslinkHeader(lastLink.Header(), link.Header())
					if err != nil {
						utils.Logger().Debug().
							Err(err).
							Msgf("[CrossLink] Failed verifying temp cross link %d", link.BlockNum().Uint64())
						break
					}
				}
			}
			shardCrossLinks[i] = append(shardCrossLinks[i], *link)
			lastLink = link
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
	cxp := types.CXReceiptsProof{}
	if err := rlp.DecodeBytes(msgPayload, &cxp); err != nil {
		utils.Logger().Error().Err(err).Msg("[ProcessReceiptMessage] Unable to Decode message Payload")
		return
	}

	if err := cxp.IsValidCXReceiptsProof(); err != nil {
		utils.Logger().Error().Err(err).Msg("[ProcessReceiptMessage] Invalid CXReceiptsProof")
		return
	}

	// TODO: check message signature is from the nodes of source shard.

	// TODO: remove in future if not useful
	node.Blockchain().WriteCXReceipts(cxp.MerkleProof.ShardID, cxp.MerkleProof.BlockNum.Uint64(), cxp.MerkleProof.BlockHash, cxp.Receipts, true)

	utils.Logger().Debug().Msg("[ProcessReceiptMessage] Add CXReceiptsProof to pending Receipts")
	node.AddPendingReceipts(&cxp)
}
