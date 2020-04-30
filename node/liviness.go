package node

import (
	"fmt"

	"golang.org/x/sync/errgroup"
)

// HandleConsensusMessageProcessing ..
func (node *Node) HandleConsensusMessageProcessing() error {
	var g errgroup.Group

	g.Go(func() error {
		for msg := range node.Consensus.IncomingConsensusMessage {
			if err := node.Consensus.HandleMessageUpdate(msg); err != nil {
				fmt.Println("some visibility into consensus messages", err.Error())
			}
		}
		return nil
	})

	g.Go(func() error {
		for due := range node.Consensus.Timeouts.Consensus.TimedOut {
			blkNow := node.Blockchain().CurrentHeader().Number().Uint64()

			// fmt.Println("PLAIN CONSENSUS TIMEOUT", due, blkNow)

			if blkNow <= due {
				fmt.Println("starting a view change ->", node.Consensus.ViewID())
				node.Consensus.StartViewChange(node.Consensus.ViewID() + 1)
			}
		}
		return nil
	})

	g.Go(func() error {
		for due := range node.Consensus.Timeouts.ViewChange.TimedOut {
			viewIDNow := node.Consensus.Current.ViewID()

			// fmt.Println("VIEWCHANGE TIMEOUT", due, viewIDNow)

			if viewIDNow <= due {
				fmt.Println("starting a view change ->", node.Consensus.Current.ViewID())
				node.Consensus.StartViewChange(viewIDNow + 1)
			}
		}
		return nil
	})

	return g.Wait()
}
