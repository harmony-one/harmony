package node

import (
	"bytes"
	"context"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/proto"
	protonode "github.com/harmony-one/harmony/api/proto/protonode"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	internal_bls "github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/availability"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/harmony-one/harmony/webhooks"
	"github.com/pkg/errors"
)

const p2pMsgPrefixSize = 5

// some messages have uninteresting fields in header, slash, receipt and crosslink are
// such messages. This function assumes that input bytes are a slice which already
// past those not relevant header bytes.
func (node *Node) processSkippedMsgTypeByteValue(
	cat protonode.BlockMessageType, content []byte,
) {
	switch cat {
	case protonode.SlashCandidate:
		node.processSlashCandidateMessage(content)
	case protonode.Receipt:
		utils.Logger().Debug().Msg("NET: received message: Node/Receipt")
		node.ProcessReceiptMessage(content)
	case protonode.CrossLink:
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

func (node *Node) handleClientMessage(
	ctx context.Context, isConsensusBound bool, msgPayload []byte,
) error {

	return nil

	switch protonode.Transaction {
	case protonode.Transaction:
		utils.Logger().Debug().Msg("NET: received message: Node/Transaction")
		node.transactionMessageHandler(msgPayload)
	case protonode.Staking:
		utils.Logger().Debug().Msg("NET: received message: Node/Staking")
		node.stakingMessageHandler(msgPayload)
	case protonode.Block:
		utils.Logger().Debug().Msg("NET: received message: Node/Block")
		if len(msgPayload) < 1 {
			utils.Logger().Debug().Msgf("Invalid block message size")
			return errors.New("invalid block message size")
		}

		switch blockMsgType := protonode.BlockMessageType(msgPayload[0]); blockMsgType {
		case protonode.Sync:
			utils.Logger().Debug().Msg("NET: received message: Node/Sync")
			blocks := []*types.Block{}
			if err := rlp.DecodeBytes(msgPayload[1:], &blocks); err != nil {
				utils.Logger().Error().
					Err(err).
					Msg("block sync")
			} else {
				// for non-beaconchain node, subscribe to beacon block broadcast
				if node.Blockchain().ShardID() != shard.BeaconChainShardID &&
					node.NodeConfig.Role() != nodeconfig.ExplorerNode {
					for _, block := range blocks {
						if block.ShardID() == 0 {
							utils.Logger().Info().
								Uint64("block", blocks[0].NumberU64()).
								Msgf("Beacon block being handled by block channel: %d", block.NumberU64())
							go func(blk *types.Block) {
								node.BeaconBlockChannel <- blk
							}(block)
						}
					}
				}
				if node.Client != nil && node.Client.UpdateBlocks != nil && blocks != nil {
					utils.Logger().Info().Msg("Block being handled by client")
					node.Client.UpdateBlocks(blocks)
				}
			}
		case
			protonode.SlashCandidate,
			protonode.Receipt,
			protonode.CrossLink:
			// skip first byte which is blockMsgType
			node.processSkippedMsgTypeByteValue(blockMsgType, msgPayload[1:])
		}
	}

	return nil
}

var (
	errWrongBlockMsgSize = errors.New("invalid block message size")
)

// HandleNodeMessage parses the message and dispatch the actions.
func (node *Node) HandleNodeMessage(
	ctx context.Context,
	payload []byte,
) error {

	msgPayload := payload[proto.MessageCategoryBytes+proto.MessageTypeBytes:]
	msgType := byte(payload[proto.MessageCategoryBytes+proto.MessageTypeBytes-1])
	actionType := protonode.MessageType(msgType)

	switch actionType {
	case protonode.Transaction:
		utils.Logger().Debug().Msg("NET: received message: Node/Transaction")
		node.transactionMessageHandler(msgPayload)
	case protonode.Staking:
		utils.Logger().Debug().Msg("NET: received message: Node/Staking")
		node.stakingMessageHandler(msgPayload)
	case protonode.Block:
		utils.Logger().Debug().Msg("NET: received message: Node/Block")
		if len(msgPayload) < 1 {
			utils.Logger().Debug().Msgf("Invalid block message size")
			return errors.WithStack(errWrongBlockMsgSize)
		}

		switch blockMsgType := protonode.BlockMessageType(msgPayload[0]); blockMsgType {
		case protonode.Sync:
			utils.Logger().Debug().Msg("NET: received message: Node/Sync")
			blocks := []*types.Block{}
			if err := rlp.DecodeBytes(msgPayload[1:], &blocks); err != nil {
				return err
			}
			// for non-beaconchain node, subscribe to beacon block broadcast
			if node.Blockchain().ShardID() != shard.BeaconChainShardID &&
				node.NodeConfig.Role() != nodeconfig.ExplorerNode {
				for _, block := range blocks {
					if block.ShardID() == 0 {
						utils.Logger().Info().
							Uint64("block", blocks[0].NumberU64()).
							Msgf("Beacon block being handled by block channel: %d", block.NumberU64())
						go func(blk *types.Block) {
							node.BeaconBlockChannel <- blk
						}(block)
					}
				}
			}
			if node.Client != nil && node.Client.UpdateBlocks != nil && blocks != nil {
				utils.Logger().Info().Msg("Block being handled by client")
				node.Client.UpdateBlocks(blocks)
			}

		case
			protonode.SlashCandidate,
			protonode.Receipt,
			protonode.CrossLink:
			// skip first byte which is blockMsgType
			node.processSkippedMsgTypeByteValue(blockMsgType, msgPayload[1:])
		}
	}

	return nil
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
	txMessageType := protonode.TransactionMessageType(msgPayload[0])

	switch txMessageType {
	case protonode.Send:
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
	txMessageType := protonode.TransactionMessageType(msgPayload[0])

	switch txMessageType {
	case protonode.Send:
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

// BroadcastNewBlock is called by consensus leader to sync new blocks with other clients/nodes.
// NOTE: For now, just send to the client (basically not broadcasting)
// TODO (lc): broadcast the new blocks to new nodes doing state sync
func (node *Node) BroadcastNewBlock(newBlock *types.Block) {
	groups := []nodeconfig.GroupID{node.NodeConfig.GetClientGroupID()}
	utils.Logger().Info().
		Msgf(
			"broadcasting new block %d, group %s", newBlock.NumberU64(), groups[0],
		)
	msg := p2p.ConstructMessage(
		protonode.ConstructBlocksSyncMessage([]*types.Block{newBlock}),
	)
	if err := node.host.SendMessageToGroups(groups, msg); err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot broadcast new block")
	}
}

// BroadcastSlash ..
func (node *Node) BroadcastSlash(witness *slash.Record) {
	if err := node.host.SendMessageToGroups(
		[]nodeconfig.GroupID{nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID)},
		p2p.ConstructMessage(
			protonode.ConstructSlashMessage(slash.Records{*witness})),
	); err != nil {
		utils.Logger().Err(err).
			RawJSON("record", []byte(witness.String())).
			Msg("could not send slash record to beaconchain")
	}
	utils.Logger().Info().Msg("broadcast the double sign record")
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

	utils.Logger().Info().Msgf("[BroadcastCrossLink] Broadcasting Block Headers, latestBlockNum %d, currentBlockNum %d, Number of Headers %d", latestBlockNum, newBlock.NumberU64(), len(headers))
	for _, header := range headers {
		utils.Logger().Debug().Msgf(
			"[BroadcastCrossLink] Broadcasting %d",
			header.Number().Uint64(),
		)
	}
	node.host.SendMessageToGroups(
		[]nodeconfig.GroupID{nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID)},
		p2p.ConstructMessage(
			protonode.ConstructCrossLinkMessage(node.Consensus.ChainReader, headers)),
	)
}

// VerifyNewBlock is called by consensus participants to verify the block (account model) they are
// running consensus on
func (node *Node) VerifyNewBlock(newBlock *types.Block) error {
	if newBlock == nil || newBlock.Header() == nil {
		return errors.New("nil header or block asked to verify")
	}
	if err := node.Blockchain().Validator().ValidateHeader(newBlock, true); err != nil {
		utils.Logger().Error().
			Str("blockHash", newBlock.Hash().Hex()).
			Err(err).
			Msg("[VerifyNewBlock] Cannot validate header for the new block")
		return err
	}

	if newBlock.ShardID() != node.Blockchain().ShardID() {
		utils.Logger().Error().
			Uint32("my shard ID", node.Blockchain().ShardID()).
			Uint32("new block's shard ID", newBlock.ShardID()).
			Msg("[VerifyNewBlock] Wrong shard ID of the new block")
		return errors.New("[VerifyNewBlock] Wrong shard ID of the new block")
	}

	if err := node.Blockchain().Engine().VerifyShardState(
		node.Blockchain(), node.Beaconchain(), newBlock.Header(),
	); err != nil {
		utils.Logger().Error().
			Str("blockHash", newBlock.Hash().Hex()).
			Err(err).
			Msg("[VerifyNewBlock] Cannot verify shard state for the new block")
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
		utils.Logger().Error().
			Str("blockHash", newBlock.Hash().Hex()).
			Int("numTx", len(newBlock.Transactions())).
			Int("numStakingTx", len(newBlock.StakingTransactions())).
			Err(err).
			Msg("[VerifyNewBlock] Cannot Verify New Block!!!")
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
		utils.Logger().Error().
			Str("blockHash", newBlock.Hash().Hex()).
			Int("numIncomingReceipts", len(newBlock.IncomingReceipts())).
			Err(err).
			Msg("[VerifyNewBlock] Cannot ValidateNewBlock")
		return errors.Wrapf(
			err, "[VerifyNewBlock] Cannot ValidateNewBlock",
		)
	}
	return nil
}

func (node *Node) numSignaturesIncludedInBlock(block *types.Block) uint32 {
	count := uint32(0)
	pubkeys := node.Consensus.Decider.Participants()
	mask, err := internal_bls.NewMask(pubkeys, nil)
	if err != nil {
		return count
	}
	err = mask.SetMask(block.Header().LastCommitBitmap())
	if err != nil {
		return count
	}
	for _, key := range node.Consensus.PubKey.PublicKey {
		if ok, err := mask.KeyEnabled(key); err == nil && ok {
			count++
		}
	}
	return count
}

// PostConsensusProcessing is called by consensus participants, after consensus is done, to:
// 1. add the new block to blockchain
// 2. [leader] send new block to the client
// 3. [leader] send cross shard tx receipts to destination shard
func (node *Node) PostConsensusProcessing(
	newBlock *types.Block,
) {
	if _, err := node.Blockchain().InsertChain([]*types.Block{newBlock}, true); err != nil {
		utils.Logger().Error().
			Err(err).
			Uint64("blockNum", newBlock.NumberU64()).
			Str("parentHash", newBlock.Header().ParentHash().Hex()).
			Str("hash", newBlock.Header().Hash().Hex()).
			Msg("Error Adding new block to blockchain")
		return
	}
	utils.Logger().Info().
		Uint64("blockNum", newBlock.NumberU64()).
		Str("hash", newBlock.Header().Hash().Hex()).
		Msg("Added New Block to Blockchain!!!")

	if node.Consensus.IsLeader() {
		if node.NodeConfig.ShardID == shard.BeaconChainShardID {
			node.BroadcastNewBlock(newBlock)
		}
		if node.NodeConfig.ShardID != shard.BeaconChainShardID &&
			node.Blockchain().Config().IsCrossLink(newBlock.Epoch()) {
			node.BroadcastCrossLink(newBlock)
		}
		node.BroadcastCXReceipts(newBlock)
	} else {
		if node.Consensus.Mode() != consensus.Listening {
			utils.Logger().Info().
				Uint64("blockNum", newBlock.NumberU64()).
				Uint64("epochNum", newBlock.Epoch().Uint64()).
				Uint64("ViewId", newBlock.Header().ViewID().Uint64()).
				Str("blockHash", newBlock.Hash().String()).
				Int("numTxns", len(newBlock.Transactions())).
				Int("numStakingTxns", len(newBlock.StakingTransactions())).
				Uint32("numSignatures", node.numSignaturesIncludedInBlock(newBlock)).
				Msg("BINGO !!! Reached Consensus")
			// 1% of the validator also need to do broadcasting
			rand.Seed(time.Now().UTC().UnixNano())
			rnd := rand.Intn(100)
			if rnd < 1 {
				// Beacon validators also broadcast new blocks to make sure beacon sync is strong.
				if node.NodeConfig.ShardID == shard.BeaconChainShardID {
					node.BroadcastNewBlock(newBlock)
				}
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
	if h := node.NodeConfig.WebHooks.Hooks; h != nil {
		if h.Availability != nil {
			for _, addr := range node.GetAddresses(newBlock.Epoch()) {
				wrapper, err := node.Beaconchain().ReadValidatorInformation(addr)
				if err != nil {
					return
				}
				snapshot, err := node.Beaconchain().ReadValidatorSnapshot(addr)
				if err != nil {
					return
				}
				computed := availability.ComputeCurrentSigning(
					snapshot.Validator, wrapper,
				)
				beaconChainBlocks := uint64(
					node.Beaconchain().CurrentBlock().Header().Number().Int64(),
				) % shard.Schedule.BlocksPerEpoch()
				computed.BlocksLeftInEpoch = shard.Schedule.BlocksPerEpoch() - beaconChainBlocks

				if err != nil && computed.IsBelowThreshold {
					url := h.Availability.OnDroppedBelowThreshold
					go func() {
						webhooks.DoPost(url, computed)
					}()
				}
			}
		}
	}
}

// BootstrapConsensus is the a goroutine to check number of peers and start the consensus
func (node *Node) BootstrapConsensus() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	min := node.Consensus.MinPeers
	needFriends := make(chan struct{})
	const checkEvery = 3 * time.Second
	go func() {
		for {
			<-time.After(checkEvery)
			numPeersNow := node.host.GetPeerCount()
			if numPeersNow >= min {
				utils.Logger().Info().Msg("[bootstrap] StartConsensus")
				needFriends <- struct{}{}
				return
			}
			utils.Logger().Info().
				Int("numPeersNow", numPeersNow).
				Int("targetNumPeers", min).
				Dur("next-peer-count-check-in-seconds", checkEvery).
				Msg("do not have enough min peers yet in bootstrap of consensus")
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-needFriends:
		go func() {
			node.startConsensus <- struct{}{}
		}()
		return nil
	}
}
