package node

import (
	"bytes"
	"fmt"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/proto"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/harmony-one/harmony/webhooks"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
)

const p2pMsgPrefixSize = 5

// HandleConsensusMessageProcessing ..
func (node *Node) HandleConsensusMessageProcessing() error {
	i := 0
	for msg := range node.Consensus.IncomingConsensusMessage {

		if err := node.Consensus.HandleMessageUpdate(msg); err != nil {
			fmt.Println("some visibility into consensus messages", err.Error())
			return err
		}
		i++

		// fmt.Println("handling ith consensus message", i, "on shard", node.Consensus.ShardID)
	}

	return nil
}

// HandleMessage parses the message and dispatch the actions.
func (node *Node) HandleMessage(
	content []byte, sender libp2p_peer.ID, topic string,
) {
	msgCategory, err := proto.GetMessageCategory(content)
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("HandleMessage get message category failed")
		return
	}
	msgType, err := proto.GetMessageType(content)
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("HandleMessage get message type failed")
		return
	}

	msgPayload, err := proto.GetMessagePayload(content)
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("HandleMessage get message payload failed")
		return
	}

	switch msgCategory {
	case proto.Consensus:
		msgPayload, _ := proto.GetConsensusMessagePayload(content)
		if node.NodeConfig.Role() == nodeconfig.ExplorerNode {
			node.ExplorerMessageHandler(msgPayload)
		} else {
			// TODO handle this higher up, give conesnsus a weight of 1, others a weight of 2
			node.Consensus.IncomingConsensusMessage <- msgPayload
		}
	case proto.Node:
		actionType := proto_node.MessageType(msgType)
		switch actionType {
		case proto_node.Transaction:
			node.transactionMessageHandler(msgPayload)
		case proto_node.Staking:
			node.stakingMessageHandler(msgPayload)
		case proto_node.Block:
			if len(msgPayload) < 1 {
				utils.Logger().Debug().Msgf("Invalid block message size")
				return
			}
			switch blockMsgType := proto_node.BlockMessageType(msgPayload[0]); blockMsgType {
			case proto_node.Sync:
				var blocks []*types.Block
				if err := rlp.DecodeBytes(msgPayload[1:], &blocks); err != nil {
					return
				}

				isValidatorNode := node.NodeConfig.Role() == nodeconfig.Validator

				if isValidatorNode {
					for _, block := range blocks {
						// for the closure
						_ = block

						go func() {
							// node.IncomingBlocksClient <- blk
							// fmt.Println("check myself-> is beaconchain node?", blk.String(), topic)
						}()
					}

				}

			case
				proto_node.SlashCandidate,
				proto_node.Receipt,
				proto_node.CrossLink:
				// skip first byte which is blockMsgType
				node.processSkippedMsgTypeByteValue(blockMsgType, msgPayload[1:])
			}
		}
	default:
		utils.Logger().Error().
			Str("Unknown MsgCateogry", string(msgCategory))
	}
}

func (node *Node) transactionMessageHandler(msgPayload []byte) {
	if len(msgPayload) >= types.MaxEncodedPoolTransactionSize {
		utils.Logger().Warn().Err(core.ErrOversizedData).Msgf("encoded tx size: %d", len(msgPayload))
		return
	}
	if len(msgPayload) < 1 {
		utils.Logger().Debug().Msgf("Invalid transaction message size")
		return
	}
	txMessageType := proto_node.TransactionMessageType(msgPayload[0])

	switch txMessageType {
	case proto_node.Send:
		txs := types.Transactions{}
		err := rlp.Decode(bytes.NewReader(msgPayload[1:]), &txs) // skip the Send messge type
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Msg("Failed to deserialize transaction list")
			return
		}
		node.addPendingTransactions(txs)
	}
}

func (node *Node) stakingMessageHandler(msgPayload []byte) {
	if len(msgPayload) >= types.MaxEncodedPoolTransactionSize {
		utils.Logger().Warn().Err(core.ErrOversizedData).Msgf("encoded tx size: %d", len(msgPayload))
		return
	}
	if len(msgPayload) < 1 {
		utils.Logger().Debug().Msgf("Invalid staking transaction message size")
		return
	}
	txMessageType := proto_node.TransactionMessageType(msgPayload[0])

	switch txMessageType {
	case proto_node.Send:
		txs := staking.StakingTransactions{}
		err := rlp.Decode(bytes.NewReader(msgPayload[1:]), &txs) // skip the Send messge type
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Msg("Failed to deserialize staking transaction list")
			return
		}
		node.addPendingStakingTransactions(txs)
	}
}

// verifyNewBlock is called by consensus participants
// to verify the block (account model) they are
// running consensus on
func (node *Node) verifyBlock(newBlock *types.Block) error {

	if err := node.Blockchain().Validator().ValidateHeader(newBlock, true); err != nil {
		utils.Logger().Error().
			Str("blockHash", newBlock.Hash().Hex()).
			Err(err).
			Msg("[VerifyNewBlock] Cannot validate header for the new block")
		return err
	}

	if newBlock.ShardID() != node.Blockchain().ShardID() {
		return errors.New("[VerifyNewBlock] Wrong shard ID of the new block")
	}

	if err := node.Blockchain().Engine().VerifyShardState(
		node.Blockchain(), node.Beaconchain(), newBlock.Header(),
	); err != nil {
		return errors.New(
			"[VerifyNewBlock] Cannot verify shard state for the new block",
		)
	}

	if err := node.Blockchain().ValidateNewBlock(newBlock); err != nil {
		if hooks := node.NodeConfig.WebHooks.Hooks; hooks != nil {
			if p := hooks.ProtocolIssues; p != nil {
				url := p.OnCannotCommit
				go func() {
					webhooks.DoPost(url, map[string]interface{}{
						"bad-header": newBlock.Header(),
						"reason":     err.Error(),
					})
				}()
			}
		}
		return errors.Errorf(
			"[VerifyNewBlock] Cannot Verify New Block!!! block-hash %s txn-count %d",
			newBlock.Hash().Hex(),
			len(newBlock.Transactions()),
		)
	}

	// Verify cross links
	// TODO: move into ValidateNewBlock
	if node.NodeConfig.ShardID == shard.BeaconChainShardID {
		err := node.VerifyBlockCrossLinks(newBlock)
		if err != nil {
			utils.Logger().Debug().Err(err).Msg("ops2 VerifyBlockCrossLinks Failed")
			return err
		}
	}

	// TODO: move into ValidateNewBlock
	if err := node.verifyIncomingReceipts(newBlock); err != nil {
		return errors.Wrapf(
			err, "cannot verify incoming receipts",
		)
	}
	return nil
}

// postConsensusProcessing is called by consensus participants, after consensus is done, to:
// 1. add the new block to blockchain
// 2. [leader] send new block to the client
// 3. [leader] send cross shard tx receipts to destination shard
func (node *Node) postConsensusProcessing(
	newBlock *types.Block,
) error {

	if node.Consensus.IsLeader() {

		// if err := node.Gossiper.AcceptedBlock(
		// 	node.Consensus.ShardID, newBlock,
		// ); err != nil {
		// 	return err
		// }

		// node.Gossiper.NewBeaconChainBlock(newBlock)

		// if node.Consensus.ShardID == shard.BeaconChainShardID {
		// 	// node.Gossiper.NewBeaconChainBlock(newBlock)
		// }

		// if node.Consensus.ShardID != shard.BeaconChainShardID {
		// 	node.Gossiper.NewShardChainBlock(newBlock)
		// }

		if node.NodeConfig.ShardID != shard.BeaconChainShardID &&
			node.Blockchain().Config().IsCrossLink(newBlock.Epoch()) {
			node.BroadcastCrossLink(newBlock)
		}

		node.BroadcastCXReceipts(newBlock)

	} else {
		if node.Consensus.Mode() != consensus.Listening {
			// 1% of the validator also need to do broadcasting
			rand.Seed(time.Now().UTC().UnixNano())
			rnd := rand.Intn(100)
			if rnd < 1 {
				// Beacon validators also broadcast new blocks to make sure beacon sync is strong.
				// if node.NodeConfig.ShardID == shard.BeaconChainShardID {
				// 	node.Gossiper.NewBeaconChainBlock(newBlock)
				// }

				node.BroadcastCXReceipts(newBlock)
			}
		}
	}

	// Broadcast client requested missing cross shard receipts if there is any
	node.BroadcastMissingCXReceipts()

	// Update consensus keys at last so the change of leader status doesn't mess up normal flow
	if len(newBlock.Header().ShardState()) > 0 {
		node.Consensus.SetMode(node.Consensus.UpdateConsensusInformation())
	}

	return nil
}

// BroadcastCrossLink is called by consensus leader to
// send the new header as cross link to beacon chain.
func (node *Node) BroadcastCrossLink(newBlock *types.Block) {
	// no point to broadcast the crosslink if we aren't even in the right epoch yet
	if !node.Blockchain().Config().IsCrossLink(
		node.Blockchain().CurrentHeader().Epoch(),
	) {
		return
	}

	utils.Logger().Info().Msgf(
		"Construct and Broadcasting new crosslink to beacon chain groupID %s",
		nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID),
	)
	headers := []*block.Header{}
	lastLink, err := node.Beaconchain().ReadShardLastCrossLink(newBlock.ShardID())
	var latestBlockNum uint64

	// TODO chao: record the missing crosslink in local database instead of using latest crosslink
	// if cannot find latest crosslink, broadcast latest 3 block headers
	if err != nil {
		utils.Logger().Debug().Err(err).Msg("[BroadcastCrossLink] ReadShardLastCrossLink Failed")
		header := node.Blockchain().GetHeaderByNumber(newBlock.NumberU64() - 2)
		if header != nil && node.Blockchain().Config().IsCrossLink(header.Epoch()) {
			headers = append(headers, header)
		}
		header = node.Blockchain().GetHeaderByNumber(newBlock.NumberU64() - 1)
		if header != nil && node.Blockchain().Config().IsCrossLink(header.Epoch()) {
			headers = append(headers, header)
		}
		headers = append(headers, newBlock.Header())
	} else {
		latestBlockNum = lastLink.BlockNum()
		for blockNum := latestBlockNum + 1; blockNum <= newBlock.NumberU64(); blockNum++ {
			header := node.Blockchain().GetHeaderByNumber(blockNum)
			if header != nil && node.Blockchain().Config().IsCrossLink(header.Epoch()) {
				headers = append(headers, header)
				if len(headers) == crossLinkBatchSize {
					break
				}
			}
		}
	}

	for _, header := range headers {
		utils.Logger().Debug().
			Msgf("[BroadcastCrossLink] Broadcasting %d", header.Number().Uint64())
	}

	node.host.SendMessageToGroups(
		[]nodeconfig.GroupID{nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID)},
		p2p.ConstructMessage(
			proto_node.ConstructCrossLinkMessage(node.Consensus.ChainReader, headers)),
	)
}

// some messages have uninteresting fields in header, slash, receipt and crosslink are
// such messages. This function assumes that input bytes are a slice which already
// past those not relevant header bytes.
func (node *Node) processSkippedMsgTypeByteValue(
	cat proto_node.BlockMessageType, content []byte,
) {
	switch cat {
	case proto_node.SlashCandidate:
		node.processSlashCandidateMessage(content)
	case proto_node.Receipt:
		utils.Logger().Debug().Msg("NET: received message: Node/Receipt")
		node.ProcessReceiptMessage(content)
	case proto_node.CrossLink:
		// only beacon chain will accept the header from other shards
		utils.Logger().Debug().
			Uint32("shardID", node.NodeConfig.ShardID).
			Msg("NET: received message: Node/CrossLink")
		if node.NodeConfig.ShardID != shard.BeaconChainShardID {
			return
		}
		node.ProcessCrossLinkMessage(content)
	default:
		utils.Logger().Error().
			Int("message-iota-value", int(cat)).
			Msg("Invariant usage of processSkippedMsgTypeByteValue violated")
	}
}
