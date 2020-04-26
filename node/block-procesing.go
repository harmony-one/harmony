package node

import (
	"fmt"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/shard"
	"golang.org/x/sync/errgroup"
)

// HandleConsensusBlockProcessing ..
func (node *Node) HandleConsensusBlockProcessing() error {
	var g errgroup.Group

	g.Go(func() error {
		for accepted := range node.Consensus.RoundCompleted.Request {
			fmt.Println("received block post consensus process", accepted.Blk.String())
			if err := node.postConsensusProcessing(accepted.Blk); err != nil {
				accepted.Err <- err
				continue
			}
			_, err := node.Blockchain().InsertChain(types.Blocks{accepted.Blk}, true)
			accepted.Err <- err
			fmt.Println("received block post consensus process-finished", accepted.Blk.String())
		}
		return nil
	})

	g.Go(func() error {
		for verify := range node.Consensus.Verify.Request {
			fmt.Println("received block verify process", verify.Blk.String())
			verify.Err <- node.verifyBlock(verify.Blk)
			fmt.Println("received block verify process", verify.Blk.String())
		}
		return nil
	})

	return g.Wait()

}

// HandleIncomingBlocks ..
func (node *Node) HandleIncomingBlocks() error {
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
		for blk := range node.IncomingBlocksClient {
			if b := blk; b != nil {
				if b.ShardID() == shard.BeaconChainShardID {
					chans[0] <- b
				} else {
					chans[1] <- b
				}
			}
		}
		return nil
	})

	return g.Wait()

}
