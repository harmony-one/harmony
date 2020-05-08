package node

import (
	"github.com/harmony-one/harmony/internal/utils"
)

// HandleConsensusMessageProcessing ..
func (node *Node) HandleConsensusMessageProcessing() error {

	for msg := range node.Consensus.IncomingConsensusMessage {
		if err := node.Consensus.HandleMessageUpdate(&msg); err != nil {
			utils.Logger().Info().Err(err).Msg("some visibility into consensus messages")
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
