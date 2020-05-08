package node

import (
	"context"
	"fmt"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
)

// HandleConsensus ..
func (node *Node) HandleConsensus() error {

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		select {
		case msg := <-node.Consensus.IncomingConsensusMessage:
			if err := node.Consensus.HandleMessageUpdate(&msg); err != nil {
				utils.Logger().Info().Err(err).Msg("some visibility into consensus messages")
			}
		case <-node.Consensus.CommitedBlock:
			cancel()
			//
		case <-ctx.Done():
			fmt.Println("something")
			continue
			// need to do a view change
		}
	}

	return nil

	// g.Go(func() error {
	// 	for due := range node.Consensus.Timeouts.Consensus.TimedOut {
	// 		fmt.Println("consensus did a timeout?")
	// 		// blkNow := node.Blockchain().CurrentHeader().Number().Uint64()
	// 		// if blkNow < due {
	// 		// 	viewIDNow := node.Consensus.ViewID()
	// 		// 	utils.Logger().Info().
	// 		// 		Uint64("viewID-now", viewIDNow).
	// 		// 		Msg("beginning view change")
	// 		// 	node.Consensus.StartViewChange(viewIDNow + 1)
	// 		// }
	// 	}
	// 	return nil
	// })

	// g.Go(func() error {
	// 	for due := range node.Consensus.Timeouts.ViewChange.TimedOut {
	// 		viewIDNow := node.Consensus.Current.ViewID()
	// 		if viewIDNow < due {
	// 			node.Consensus.StartViewChange(viewIDNow + 1)
	// 		}
	// 	}
	// 	return nil
	// })

}
