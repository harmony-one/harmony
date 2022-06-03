package node

import (
	"bytes"
	"context"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/proto"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
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
const p2pNodeMsgPrefixSize = proto.MessageTypeBytes + proto.MessageCategoryBytes

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
		node.ProcessReceiptMessage(content)
	case proto_node.CrossLink:
		node.ProcessCrossLinkMessage(content)
	default:
		utils.Logger().Error().
			Int("message-iota-value", int(cat)).
			Msg("Invariant usage of processSkippedMsgTypeByteValue violated")
	}
}

var (
	errInvalidPayloadSize         = errors.New("invalid payload size")
	errWrongBlockMsgSize          = errors.New("invalid block message size")
	latestSentCrosslinkNum uint64 = 0
)

// HandleNodeMessage parses the message and dispatch the actions.
func (node *Node) HandleNodeMessage(
	ctx context.Context,
	msgPayload []byte,
	actionType proto_node.MessageType,
) error {
	switch actionType {
	case proto_node.Transaction:
		node.transactionMessageHandler(msgPayload)
	case proto_node.Staking:
		node.stakingMessageHandler(msgPayload)
	case proto_node.Block:
		switch blockMsgType := proto_node.BlockMessageType(msgPayload[0]); blockMsgType {
		case proto_node.Sync:
			blocks := []*types.Block{}
			if err := rlp.DecodeBytes(msgPayload[1:], &blocks); err != nil {
				utils.Logger().Error().
					Err(err).
					Msg("block sync")
			} else {
				// for non-beaconchain node, subscribe to beacon block broadcast
				if node.Blockchain().ShardID() != shard.BeaconChainShardID {
					for _, block := range blocks {
						if block.ShardID() == 0 {
							utils.Logger().Info().
								Msgf("Beacon block being handled by block channel: %d", block.NumberU64())
							go func(blk *types.Block) {
								node.BeaconBlockChannel <- blk
							}(block)
						}
					}
				}
			}
		case
			proto_node.SlashCandidate,
			proto_node.Receipt,
			proto_node.CrossLink:
			// skip first byte which is blockMsgType
			node.processSkippedMsgTypeByteValue(blockMsgType, msgPayload[1:])
		}
	default:
		utils.Logger().Error().
			Str("Unknown actionType", string(actionType))
	}
	return nil
}

func (node *Node) transactionMessageHandler(msgPayload []byte) {
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
		proto_node.ConstructBlocksSyncMessage([]*types.Block{newBlock}),
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
			proto_node.ConstructSlashMessage(slash.Records{*witness})),
	); err != nil {
		utils.Logger().Err(err).
			RawJSON("record", []byte(witness.String())).
			Msg("could not send slash record to beaconchain")
	}
	utils.Logger().Info().Msg("broadcast the double sign record")
}

// BroadcastCrossLink is called by consensus leader to
// send the new header as cross link to beacon chain.
func (node *Node) BroadcastCrossLink() {
	curBlock := node.Blockchain().CurrentBlock()
	if curBlock == nil {
		return
	}

	if node.IsRunningBeaconChain() ||
		!node.Blockchain().Config().IsCrossLink(curBlock.Epoch()) {
		// no need to broadcast crosslink if it's beacon chain or it's not crosslink epoch
		return
	}

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
	lastLink, err := node.Beaconchain().ReadShardLastCrossLink(curBlock.ShardID())
	var latestBlockNum uint64

	// TODO chao: record the missing crosslink in local database instead of using latest crosslink
	// if cannot find latest crosslink, broadcast latest 3 block headers
	if err != nil {
		utils.Logger().Debug().Err(err).Msg("[BroadcastCrossLink] ReadShardLastCrossLink Failed")
		header := node.Blockchain().GetHeaderByNumber(curBlock.NumberU64() - 2)
		if header != nil && node.Blockchain().Config().IsCrossLink(header.Epoch()) {
			headers = append(headers, header)
		}
		header = node.Blockchain().GetHeaderByNumber(curBlock.NumberU64() - 1)
		if header != nil && node.Blockchain().Config().IsCrossLink(header.Epoch()) {
			headers = append(headers, header)
		}
		headers = append(headers, curBlock.Header())
	} else {
		latestBlockNum = lastLink.BlockNum()
		if latestSentCrosslinkNum > latestBlockNum && latestSentCrosslinkNum <= latestBlockNum+crossLinkBatchSize*6 {
			latestBlockNum = latestSentCrosslinkNum
		}

		batchSize := crossLinkBatchSize
		diff := curBlock.Number().Uint64() - latestBlockNum

		// Increase batch size by 1 for every 5 blocks behind
		batchSize += int(diff) / 5

		// Cap at a sane size to avoid overload network
		if batchSize > crossLinkBatchSize*2 {
			batchSize = crossLinkBatchSize * 2
		}

		for blockNum := latestBlockNum + 1; blockNum <= curBlock.NumberU64(); blockNum++ {
			header := node.Blockchain().GetHeaderByNumber(blockNum)
			if header != nil && node.Blockchain().Config().IsCrossLink(header.Epoch()) {
				headers = append(headers, header)
				if len(headers) == batchSize {
					break
				}
			}
		}
	}

	utils.Logger().Info().Msgf("[BroadcastCrossLink] Broadcasting Block Headers, latestBlockNum %d, currentBlockNum %d, Number of Headers %d", latestBlockNum, curBlock.NumberU64(), len(headers))
	for i, header := range headers {
		utils.Logger().Debug().Msgf(
			"[BroadcastCrossLink] Broadcasting %d",
			header.Number().Uint64(),
		)
		if i == len(headers)-1 {
			latestSentCrosslinkNum = header.Number().Uint64()
		}
	}
	node.host.SendMessageToGroups(
		[]nodeconfig.GroupID{nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID)},
		p2p.ConstructMessage(
			proto_node.ConstructCrossLinkMessage(node.Consensus.Blockchain, headers)),
	)
}

// VerifyNewBlock is called by consensus participants to verify the block (account model) they are
// running consensus on
func (node *Node) VerifyNewBlock(newBlock *types.Block) error {
	if newBlock == nil || newBlock.Header() == nil {
		return errors.New("nil header or block asked to verify")
	}

	if newBlock.ShardID() != node.Blockchain().ShardID() {
		utils.Logger().Error().
			Uint32("my shard ID", node.Blockchain().ShardID()).
			Uint32("new block's shard ID", newBlock.ShardID()).
			Msg("[VerifyNewBlock] Wrong shard ID of the new block")
		return errors.New("[VerifyNewBlock] Wrong shard ID of the new block")
	}

	if newBlock.NumberU64() <= node.Blockchain().CurrentBlock().NumberU64() {
		return errors.Errorf("block with the same block number is already committed: %d", newBlock.NumberU64())
	}
	if err := node.Blockchain().Validator().ValidateHeader(newBlock, true); err != nil {
		utils.Logger().Error().
			Str("blockHash", newBlock.Hash().Hex()).
			Err(err).
			Msg("[VerifyNewBlock] Cannot validate header for the new block")
		return err
	}

	if err := node.Blockchain().Engine().VerifyVRF(
		node.Blockchain(), newBlock.Header(),
	); err != nil {
		utils.Logger().Error().
			Str("blockHash", newBlock.Hash().Hex()).
			Err(err).
			Msg("[VerifyNewBlock] Cannot verify vrf for the new block")
		return errors.Wrap(err,
			"[VerifyNewBlock] Cannot verify vrf for the new block",
		)
	}

	if err := node.Blockchain().Engine().VerifyShardState(
		node.Blockchain(), node.Beaconchain(), newBlock.Header(),
	); err != nil {
		utils.Logger().Error().
			Str("blockHash", newBlock.Hash().Hex()).
			Err(err).
			Msg("[VerifyNewBlock] Cannot verify shard state for the new block")
		return errors.Wrap(err,
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
	if node.IsRunningBeaconChain() {
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

// PostConsensusProcessing is called by consensus participants, after consensus is done, to:
// 1. add the new block to blockchain
// 2. [leader] send new block to the client
// 3. [leader] send cross shard tx receipts to destination shard
func (node *Node) PostConsensusProcessing(newBlock *types.Block) error {
	if node.Consensus.IsLeader() {
		if node.IsRunningBeaconChain() {
			// TODO: consider removing this and letting other nodes broadcast new blocks.
			// But need to make sure there is at least 1 node that will do the job.
			node.BroadcastNewBlock(newBlock)
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
				Uint32("numSignatures", node.Consensus.NumSignaturesIncludedInBlock(newBlock)).
				Msg("BINGO !!! Reached Consensus")

			numSig := float64(node.Consensus.NumSignaturesIncludedInBlock(newBlock))
			node.Consensus.UpdateValidatorMetrics(numSig, float64(newBlock.NumberU64()))

			// 1% of the validator also need to do broadcasting
			rnd := rand.Intn(100)
			if rnd < 1 {
				// Beacon validators also broadcast new blocks to make sure beacon sync is strong.
				if node.IsRunningBeaconChain() {
					node.BroadcastNewBlock(newBlock)
				}
				node.BroadcastCXReceipts(newBlock)
			}
		}
	}

	// Broadcast client requested missing cross shard receipts if there is any
	node.BroadcastMissingCXReceipts()

	if h := node.NodeConfig.WebHooks.Hooks; h != nil {
		if h.Availability != nil {
			for _, addr := range node.GetAddresses(newBlock.Epoch()) {
				wrapper, err := node.Beaconchain().ReadValidatorInformation(addr)
				if err != nil {
					utils.Logger().Err(err).Str("addr", addr.Hex()).Msg("failed reaching validator info")
					return nil
				}
				snapshot, err := node.Beaconchain().ReadValidatorSnapshot(addr)
				if err != nil {
					utils.Logger().Err(err).Str("addr", addr.Hex()).Msg("failed reaching validator snapshot")
					return nil
				}
				computed := availability.ComputeCurrentSigning(
					snapshot.Validator, wrapper,
				)
				lastBlockOfEpoch := shard.Schedule.EpochLastBlock(node.Beaconchain().CurrentBlock().Header().Epoch().Uint64())

				computed.BlocksLeftInEpoch = lastBlockOfEpoch - node.Beaconchain().CurrentBlock().Header().Number().Uint64()

				if err != nil && computed.IsBelowThreshold {
					url := h.Availability.OnDroppedBelowThreshold
					go func() {
						webhooks.DoPost(url, computed)
					}()
				}
			}
		}
	}
	return nil
}

// BootstrapConsensus is the a goroutine to check number of peers and start the consensus
func (node *Node) BootstrapConsensus() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	min := node.Consensus.MinPeers
	enoughMinPeers := make(chan struct{})
	const checkEvery = 3 * time.Second
	go func() {
		for {
			<-time.After(checkEvery)
			numPeersNow := node.host.GetPeerCount()
			if numPeersNow >= min {
				utils.Logger().Info().Msg("[bootstrap] StartConsensus")
				enoughMinPeers <- struct{}{}
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
	case <-enoughMinPeers:
		go func() {
			node.startConsensus <- struct{}{}
		}()
		return nil
	}
}
