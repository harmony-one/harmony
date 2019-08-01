package node

import (
	"encoding/binary"
	"sync"

	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

var once sync.Once

// ExplorerMessageHandler passes received message in node_handler to explorer service
func (node *Node) ExplorerMessageHandler(payload []byte) {
	if len(payload) == 0 {
		utils.Logger().Error().Err(err).Msg("Payload is empty")
		return
	}
	msg := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, msg)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to unmarshal message payload.")
		return
	}

	if msg.Type == msg_pb.MessageType_COMMITTED {
		recvMsg, err := consensus.ParsePbftMessage(msg)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("[Explorer] onCommitted unable to parse msg")
			return
		}

		aggSig, mask, err := node.Consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("[Explorer] readSignatureBitmapPayload failed")
			return
		}

		// check has 2f+1 signatures
		if count := utils.CountOneBits(mask.Bitmap); count < node.Consensus.Quorum() {
			utils.Logger().Error().Err(err).Str("need", node.Consensus.Quorum()).Str("have", count)("[Explorer] not have enough signature")
			return
		}

		blockNumHash := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumHash, recvMsg.BlockNum)
		commitPayload := append(blockNumHash, recvMsg.BlockHash[:]...)
		if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
			utils.Logger().Error().Err(err).Str("msgBlock", recvMsg.BlockNum).Msg("[Explorer] Failed to verify the multi signature for commit phase")
			return
		}

		block := node.Consensus.PbftLog.GetBlockByHash(recvMsg.BlockHash)

		if block == nil {
			utils.Logger().Info().Str("msgBlock", recvMsg.BlockNum).Msg("[Explorer] Haven't received the block before the committed msg")
			node.Consensus.PbftLog.AddMessage(recvMsg)
			return
		}

		node.AddNewBlockForExplorer()
		node.commitBlockForExplorer(block)
	} else if msg.Type == msg_pb.MessageType_PREPARED {

		recvMsg, err := consensus.ParsePbftMessage(msg)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("[Explorer] Unable to parse Prepared msg")
			return
		}
		block := recvMsg.Block

		blockObj := &types.Block{}
		err = rlp.DecodeBytes(block, blockObj)
		// Add the block into Pbft log.
		node.Consensus.PbftLog.AddBlock(blockObj)
		// Try to search for MessageType_COMMITTED message from pbft log.
		msgs := node.Consensus.PbftLog.GetMessagesByTypeSeqHash(msg_pb.MessageType_COMMITTED, blockObj.NumberU64(), blockObj.Hash())
		// If found, then add the new block into blockchain db.
		if len(msgs) > 0 {
			node.AddNewBlockForExplorer()
			node.commitBlockForExplorer(blockObj)
		}
	}
	return
}

// AddNewBlockForExplorer add new block for explorer.
func (node *Node) AddNewBlockForExplorer() {
	utils.Logger().Info().Msg("[Explorer] Add new block for explorer")
	// Search for the next block in PbftLog and commit the block into blockchain for explorer node.
	for {
		blocks := node.Consensus.PbftLog.GetBlocksByNumber(node.Blockchain().CurrentBlock().NumberU64() + 1)
		if len(blocks) == 0 {
			break
		} else {
			if len(blocks) > 1 {
				utils.Logger().Error().Err(err).Msg("[Explorer] We should have not received more than one block with the same block height.")
			}
			utils.Logger().Info().Uint64("blockHeight", blocks[0].NumberU64()).Msg("Adding new block for explorer node")
			if err := node.AddNewBlock(blocks[0]); err == nil {
				// Clean up the blocks to avoid OOM.
				node.Consensus.PbftLog.DeleteBlockByNumber(blocks[0].NumberU64())
				// Do dump all blocks from state sycning for explorer one time
				// TODO: some blocks can be dumped before state syncing finished.
				// And they would be dumped again here. Please fix it.
				once.Do(func() {
					utils.Logger().Info().Uint64("starting height", int64(blocks[0].NumberU64())-1).Msg("[Explorer] Populating explorer data from state synced blocks")
					go func() {
						for blockHeight := int64(blocks[0].NumberU64()) - 1; blockHeight >= 0; blockHeight-- {
							explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port, true).Dump(
								node.Blockchain().GetBlockByNumber(uint64(blockHeight)), uint64(blockHeight))
						}
					}()
				})
			} else {
				utils.Logger().Error().Err(err).Msg("[Explorer] Error when adding new block for explorer node")
			}
		}
	}
}

// ExplorerMessageHandler passes received message in node_handler to explorer service
func (node *Node) commitBlockForExplorer(block *types.Block) {
	if block.ShardID() != node.NodeConfig.ShardID {
		return
	}
	// Dump new block into level db.
	utils.Logger().Info().Uint64("blockNum", block.NumberU64()).Msg("[Explorer] Committing block into explorer DB")
	explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port, true).Dump(block, block.NumberU64())

	curNum := block.NumberU64()
	if curNum-100 > 0 {
		node.Consensus.PbftLog.DeleteBlocksLessThan(curNum - 100)
		node.Consensus.PbftLog.DeleteMessagesLessThan(curNum - 100)
	}
}
