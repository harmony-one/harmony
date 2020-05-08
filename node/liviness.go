package node

import (
	"context"
	"fmt"
	"time"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/utils"
)

// HandleConsensus ..
func (node *Node) HandleConsensus() error {

	go func() {
		for msg := range node.Consensus.IncomingConsensusMessage {
			if err := node.Consensus.HandleMessageUpdate(&msg); err != nil {
				utils.Logger().Info().Err(err).Msg("some visibility into consensus messages")
			}
		}
	}()

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		select {
		case <-node.Consensus.CommitedBlock:
			cancel()
			continue
		case <-ctx.Done():
			node.Consensus.Current.SetMode(consensus.ViewChanging)

		ViewChangeLoop:
			for {

				newViewID := node.Consensus.Current.ViewID() + 1
				node.Consensus.Current.SetViewID(newViewID)
				diff := int64(newViewID - node.Consensus.ViewID())
				duration := time.Duration(diff * diff * int64(1*time.Minute))

				utils.Logger().Info().
					Uint64("ViewChangingID", newViewID).
					Dur("timeoutDuration", duration).
					Msg("[startViewChange]")

				ctx, cancel := context.WithTimeout(context.Background(), duration)
				node.Consensus.StartViewChange(newViewID)

				select {
				case <-ctx.Done():
					fmt.Println("view change timed out, try again")
					// try again
					continue
				case <-node.Consensus.ViewChangeSucceed:
					fmt.Println("view change worked, should be plain consensus now")
					cancel()

					break ViewChangeLoop
				}
			}
		}
	}

	return nil

}
