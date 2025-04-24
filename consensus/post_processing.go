package consensus

import (
	"math/rand"

	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/availability"
	"github.com/harmony-one/harmony/webhooks"
)

// postConsensusProcessing is called by consensus participants, after consensus is done, to:
// 1. [leader] send new block to the client
// 2. [leader] send cross shard tx receipts to destination shard
func (consensus *Consensus) postConsensusProcessing(newBlock *types.Block) error {
	if consensus.IsLeader() {
		if IsRunningBeaconChain(consensus) {
			// TODO: consider removing this and letting other nodes broadcast new blocks.
			// But need to make sure there is at least 1 node that will do the job.
			BroadcastNewBlock(consensus.host, newBlock, consensus.registry.GetNodeConfig())
		}
		BroadcastCXReceipts(newBlock, consensus)
	} else {
		if mode := consensus.mode(); mode != Listening {
			numSignatures := consensus.NumSignaturesIncludedInBlock(newBlock)
			consensus.getLogger().Info().
				Uint64("blockNum", newBlock.NumberU64()).
				Uint64("epochNum", newBlock.Epoch().Uint64()).
				Uint64("ViewId", newBlock.Header().ViewID().Uint64()).
				Str("blockHash", newBlock.Hash().String()).
				Int("numTxns", len(newBlock.Transactions())).
				Int("numStakingTxns", len(newBlock.StakingTransactions())).
				Uint32("numSignatures", numSignatures).
				Str("mode", mode.String()).
				Msg("BINGO !!! Reached Consensus")
			if consensus.mode() == Syncing {
				mode = consensus.updateConsensusInformation("consensus.mode() == Syncing")
				consensus.getLogger().Info().Msgf("Switching to mode %s", mode)
				consensus.setMode(mode)
			}

			consensus.UpdateValidatorMetrics(float64(numSignatures), float64(newBlock.NumberU64()))

			// 1% of the validator also need to do broadcasting
			rnd := rand.Intn(100)
			if rnd < 1 {
				// Beacon validators also broadcast new blocks to make sure beacon sync is strong.
				if IsRunningBeaconChain(consensus) {
					BroadcastNewBlock(consensus.host, newBlock, consensus.registry.GetNodeConfig())
				}
				BroadcastCXReceipts(newBlock, consensus)
			}
		}
	}

	// Broadcast client requested missing cross shard receipts if there is any
	BroadcastMissingCXReceipts(consensus)

	if h := consensus.registry.GetNodeConfig().WebHooks.Hooks; h != nil {
		if h.Availability != nil {
			shardState, err := consensus.Blockchain().ReadShardState(newBlock.Epoch())
			if err != nil {
				consensus.getLogger().Error().Err(err).
					Int64("epoch", newBlock.Epoch().Int64()).
					Uint32("shard-id", consensus.ShardID).
					Msg("failed to read shard state")
				return err
			}

			for _, addr := range consensus.Registry().GetAddressToBLSKey().GetAddresses(consensus.getPublicKeys(), shardState, newBlock.Epoch()) {
				wrapper, err := consensus.Beaconchain().ReadValidatorInformation(addr)
				if err != nil {
					consensus.getLogger().Err(err).Str("addr", addr.Hex()).Msg("failed reaching validator info")
					return nil
				}
				snapshot, err := consensus.Beaconchain().ReadValidatorSnapshot(addr)
				if err != nil {
					consensus.getLogger().Err(err).Str("addr", addr.Hex()).Msg("failed reaching validator snapshot")
					return nil
				}
				computed := availability.ComputeCurrentSigning(
					snapshot.Validator, wrapper, consensus.Blockchain().Config().IsHIP32(newBlock.Epoch()),
				)
				lastBlockOfEpoch := shard.Schedule.EpochLastBlock(consensus.Beaconchain().CurrentBlock().Header().Epoch().Uint64())

				computed.BlocksLeftInEpoch = lastBlockOfEpoch - consensus.Beaconchain().CurrentBlock().Header().Number().Uint64()

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

// BroadcastNewBlock is called by consensus leader to sync new blocks with other clients/nodes.
// NOTE: For now, just send to the client (basically not broadcasting)
// TODO (lc): broadcast the new blocks to new nodes doing state sync
func BroadcastNewBlock(host p2p.Host, newBlock *types.Block, nodeConfig *nodeconfig.ConfigType) {
	groups := []nodeconfig.GroupID{nodeConfig.GetClientGroupID()}
	utils.Logger().Info().
		Msgf(
			"broadcasting new block %d, group %s", newBlock.NumberU64(), groups[0],
		)
	msg := p2p.ConstructMessage(
		proto_node.ConstructBlocksSyncMessage([]*types.Block{newBlock}),
	)
	if err := host.SendMessageToGroups(groups, msg); err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot broadcast new block")
	}
}

func IsRunningBeaconChain(c *Consensus) bool {
	return c.ShardID == shard.BeaconChainShardID
}

// BroadcastCXReceipts broadcasts cross shard receipts to correspoding
// destination shards
func BroadcastCXReceipts(newBlock *types.Block, consensus *Consensus) {
	commitSigAndBitmap := newBlock.GetCurrentCommitSig()
	//#### Read payload data from committed msg
	if len(commitSigAndBitmap) <= 96 {
		utils.Logger().Debug().Int("commitSigAndBitmapLen", len(commitSigAndBitmap)).Msg("[BroadcastCXReceipts] commitSigAndBitmap Not Enough Length")
		return
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
	myShardID := consensus.ShardID
	utils.Logger().Info().Int("shardNum", shardNum).Uint32("myShardID", myShardID).Uint64("blockNum", newBlock.NumberU64()).Msg("[BroadcastCXReceipts]")

	for i := 0; i < shardNum; i++ {
		if i == int(myShardID) {
			continue
		}
		BroadcastCXReceiptsWithShardID(newBlock.Header(), commitSig, commitBitmap, uint32(i), consensus)
	}
}

// BroadcastCXReceiptsWithShardID broadcasts cross shard receipts to given ToShardID
func BroadcastCXReceiptsWithShardID(block *block.Header, commitSig []byte, commitBitmap []byte, toShardID uint32, consensus *Consensus) {
	myShardID := consensus.ShardID
	utils.Logger().Debug().
		Uint32("toShardID", toShardID).
		Uint32("myShardID", myShardID).
		Uint64("blockNum", block.NumberU64()).
		Msg("[BroadcastCXReceiptsWithShardID]")

	cxReceipts, err := consensus.Blockchain().ReadCXReceipts(toShardID, block.NumberU64(), block.Hash())
	if err != nil || len(cxReceipts) == 0 {
		utils.Logger().Debug().Uint32("ToShardID", toShardID).
			Int("numCXReceipts", len(cxReceipts)).
			Msg("[CXMerkleProof] No receipts found for the destination shard")
		return
	}

	merkleProof, err := consensus.Blockchain().CXMerkleProof(toShardID, block)
	if err != nil {
		utils.Logger().Warn().
			Uint32("ToShardID", toShardID).
			Msg("[BroadcastCXReceiptsWithShardID] Unable to get merkleProof")
		return
	}

	cxReceiptsProof := &types.CXReceiptsProof{
		Receipts:     cxReceipts,
		MerkleProof:  merkleProof,
		Header:       block,
		CommitSig:    commitSig,
		CommitBitmap: commitBitmap,
	}

	groupID := nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(toShardID))
	utils.Logger().Info().Uint32("ToShardID", toShardID).
		Str("GroupID", string(groupID)).
		Interface("cxp", cxReceiptsProof).
		Msg("[BroadcastCXReceiptsWithShardID] ReadCXReceipts and MerkleProof ready. Sending CX receipts...")
	// TODO ek – limit concurrency
	consensus.GetHost().SendMessageToGroups([]nodeconfig.GroupID{groupID},
		p2p.ConstructMessage(proto_node.ConstructCXReceiptsProof(cxReceiptsProof)),
	)
}

// BroadcastMissingCXReceipts broadcasts missing cross shard receipts per request
func BroadcastMissingCXReceipts(c *Consensus) {
	var (
		sendNextTime = make([]core.CxEntry, 0)
		cxPool       = c.Registry().GetCxPool()
		blockchain   = c.Blockchain()
	)
	it := cxPool.Pool().Iterator()
	for entry := range it.C {
		cxEntry := entry.(core.CxEntry)
		toShardID := cxEntry.ToShardID
		blk := blockchain.GetBlockByHash(cxEntry.BlockHash)
		if blk == nil {
			continue
		}
		blockNum := blk.NumberU64()
		nextHeader := blockchain.GetHeaderByNumber(blockNum + 1)
		if nextHeader == nil {
			sendNextTime = append(sendNextTime, cxEntry)
			continue
		}
		sig := nextHeader.LastCommitSignature()
		bitmap := nextHeader.LastCommitBitmap()
		BroadcastCXReceiptsWithShardID(blk.Header(), sig[:], bitmap, toShardID, c)
	}
	cxPool.Clear()
	// this should not happen or maybe happen for impatient user
	for _, entry := range sendNextTime {
		cxPool.Add(entry)
	}
}
