package node

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

// Process is called by consensus participants, after consensus is done, to:
// 1. add the new block to blockchain
// 2. [leader] send new block to the client
// 3. [leader] send cross shard tx receipts to destination shard
func (node *Node) Process(newBlock *types.Block) error {
	if _, err := node.Blockchain().InsertChain(
		[]*types.Block{newBlock}, true,
	); err != nil {
		utils.Logger().Error().
			Err(err).
			Uint64("blockNum", newBlock.NumberU64()).
			Str("parentHash", newBlock.Header().ParentHash().Hex()).
			Str("hash", newBlock.Header().Hash().Hex()).
			Msg("Error Adding new block to blockchain")
		return err
	}
	// grab lock for insertion of block?

	if node.Consensus.IsLeader() {

		// if err := node.Gossiper.AcceptedBlockForShardGroup(
		// 	node.Consensus.ShardID, newBlock,
		// ); err != nil {
		// 	return err
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
				if node.NodeConfig.ShardID == shard.BeaconChainShardID {
					// node.Gossiper.NewBeaconChainBlockForClient(newBlock)
				}
				node.BroadcastCXReceipts(newBlock)
			}
		}
	}

	// Broadcast client requested missing cross shard receipts if there is any
	node.BroadcastMissingCXReceipts()

	// Update consensus keys at last so the change of leader status doesn't mess up normal flow
	if len(newBlock.Header().ShardState()) > 0 {
		node.Consensus.SetMode(
			node.Consensus.UpdateConsensusInformation(),
		)
	}

	return nil
}

// HandleBlockProcessing ..
func (node *Node) HandleBlockProcessing() error {

	for blk := range node.IncomingBlocks {
		fmt.Println("before insert", blk.String())
		_, err := node.Blockchain().InsertChain(
			types.Blocks{blk}, true,
		)

		fmt.Println("afer insert", blk.String(), err)

	}
	return nil

}
