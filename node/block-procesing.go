package node

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/shard"
	"golang.org/x/sync/errgroup"
)

// postConsensusProcessing is called by consensus participants, after consensus is done, to:
// 1. add the new block to blockchain
// 2. [leader] send new block to the client
// 3. [leader] send cross shard tx receipts to destination shard
func (node *Node) postConsensusProcessing(
	newBlock *types.Block, leader string,
) error {

	if node.Consensus.IsLeader() {

		if err := node.Gossiper.AcceptedBlock(
			node.Consensus.ShardID, newBlock,
		); err != nil {
			return err
		}

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
		node.Consensus.SetMode(
			node.Consensus.UpdateConsensusInformation(leader),
		)
	}

	return nil
}

// HandleConsensusBlockProcessing ..
func (node *Node) HandleConsensusBlockProcessing() error {
	var g errgroup.Group

	g.Go(func() error {
		for accepted := range node.Consensus.RoundCompleted.Request {

			if accepted.Blk.ParentHash() == node.Blockchain().CurrentHeader().Hash() {
				if _, err := node.Blockchain().InsertChain(
					types.Blocks{accepted.Blk}, true,
				); err != nil {
					accepted.Err <- err
					continue
				}

				if len(accepted.Blk.Header().ShardState()) > 0 {
					// fmt.Println("before post consensus on new shard state header")
				}
				// fmt.Println("WHAT is leader public key now?",
				// 	node.Consensus.LeaderPubKey().SerializeToHexStr(),
				// 	"should be SOMETHING",
				// )
				accepted.Err <- node.postConsensusProcessing(
					accepted.Blk, node.Consensus.LeaderPubKey().SerializeToHexStr(),
				)

				if len(accepted.Blk.Header().ShardState()) > 0 {
					// fmt.Println("after post consensus on new shard state header")

				}
			} else {
				accepted.Err <- nil
			}

		}
		return nil
	})

	g.Go(func() error {
		for verify := range node.Consensus.Verify.Request {
			// fmt.Println("received block verify process", verify.Blk.String())
			verify.Err <- node.verifyBlock(verify.Blk)
			// fmt.Println("received block verify process", verify.Blk.String())
		}
		return nil
	})

	return g.Wait()

}

// HandleIncomingBlock ..
func (node *Node) HandleIncomingBlock() error {
	var g errgroup.Group
	chans := []chan *types.Block{
		make(chan *types.Block), make(chan *types.Block),
	}

	g.Go(func() error {
		for acceptedBlock := range chans[0] {
			if acceptedBlock != nil {
				if _, err := node.Beaconchain().InsertChain(
					types.Blocks{acceptedBlock}, true,
				); err != nil {
					return err
				}
			}
			fmt.Println("beaconchain chan wrote", node.Consensus.ShardID, acceptedBlock.String())
		}
		return nil
	})

	g.Go(func() error {
		for acceptedBlock := range chans[1] {
			if acceptedBlock != nil && node.Consensus.ShardID != shard.BeaconChainShardID {
				if _, err := node.Blockchain().InsertChain(
					types.Blocks{acceptedBlock}, true,
				); err != nil {
					return err
				}
				fmt.Println("blockchain chan wrote", node.Consensus.ShardID, acceptedBlock.String())
			}
		}
		return nil
	})

	g.Go(func() error {
		for blk := range node.IncomingBlocks {
			if b := blk; b != nil {
				if blk.ParentHash() == node.Blockchain().CurrentHeader().Hash() {

					fmt.Println(
						node.Consensus.ShardID,
						"GOT IT!->",
						b.String(),
						b.Number(),
					)

					if _, err := node.Blockchain().InsertChain(
						types.Blocks{b}, true,
					); err != nil {
						fmt.Println("why couldnt insert", err.Error())
						return err
					}

				}

			}
		}
		return nil
	})

	return g.Wait()

}
