package node

import (
	"encoding/binary"

	"bytes"
	"encoding/base64"

	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"

	proto_node "github.com/harmony-one/harmony/api/proto/node"
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
				Msg("Crosslink Headers Broadcast Unable to Decode")
			return
		}

		utils.Logger().Debug().
			Msgf("[ProcessingHeader NUM] %d", len(headers))
		// Try to reprocess all the pending cross links
		node.pendingClMutex.Lock()
		crossLinkHeadersToProcess := node.pendingCrossLinks
		node.pendingCrossLinks = []*types.Header{}
		node.pendingClMutex.Unlock()

		crossLinkHeadersToProcess = append(crossLinkHeadersToProcess, headers...)

		headersToQuque := []*types.Header{}

		for _, header := range crossLinkHeadersToProcess {

			utils.Logger().Debug().
				Msgf("[ProcessingHeader] 1 shardID %d, blockNum %d", header.ShardID, header.Number.Uint64())
			exist, err := node.Blockchain().ReadCrossLink(header.ShardID, header.Number.Uint64(), false)
			if err == nil && exist != nil {
				// Cross link already exists, skip
				continue
			}

			if header.Number.Uint64() > 0 { // Blindly trust the first cross-link
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
						Msgf("Failed to verify new cross link header for shardID %d, blockNum %d", header.ShardID, header.Number)
					continue
				}
			}

			crossLink := types.NewCrossLink(header)
			utils.Logger().Debug().
				Msgf("[ProcessingHeader] committing shardID %d, blockNum %d", header.ShardID, header.Number.Uint64())
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
	if len(merkleProof.ShardID) == 0 {
		utils.Logger().Warn().Msg("[ProcessReceiptMessage] There is No non-empty destination shards")
		return
	}

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

	if !foundMyShard {
		utils.Logger().Warn().Msg("[ProcessReceiptMessage] Not Found My Shard in CXReceipt Message")
		return
	}

	hash := crypto.Keccak256Hash(byteBuffer.Bytes())
	utils.Logger().Debug().Interface("hash", hash).Msg("[ProcessReceiptMessage] RootHash of the CXReceipts")
	// TODO chao: use crosslink from beacon sync to verify the hash

	cxReceipts := cxmsg.Receipts
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
		tx := types.NewCrossShardTransaction(0, cx.To, cx.ShardID, cx.ToShardID, cx.Amount, gas, nil, inputData)
		txs = append(txs, tx)
	}
	node.addPendingTransactions(txs)
}

// ProcessCrossShardTx verify and process cross shard transaction on destination shard
func (node *Node) ProcessCrossShardTx(blocks []*types.Block) {
	// TODO: add logic
}
