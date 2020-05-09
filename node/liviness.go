package node

import (
	"fmt"
	"time"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/utils"
)

// HandleConsensus ..
func (node *Node) HandleConsensus() error {
	tick := time.NewTicker(2 * time.Second)

	go func() {
		for msg := range node.Consensus.IncomingConsensusMessage {
			if err := node.Consensus.HandleMessageUpdate(&msg); err != nil {
				utils.Logger().Info().Err(err).Msg("some visibility into consensus messages")
			}
		}
	}()

	timeLast := time.Now()

	for {

		select {
		case <-node.Consensus.ResetConsensusTimeout:
			timeLast = time.Now()
		case <-tick.C:
			if node.Consensus.IsLeader() {
				break
			}

			if node.Consensus.Current.Mode() == consensus.Normal {
				since := time.Since(timeLast).Round(time.Second)

				if since > 20*time.Second {

					fmt.Println(
						"was it more than 20 second",
						node.Consensus.PubKey.SerializeToHexStr(),
						node.Consensus.IsLeader(),
						node.Consensus.Current.Mode(),
					)
				}

			}
			// if since > 20*time.Second {
			// 	fmt.Println("view change will actually happen this went off")
			// 	node.Consensus.StartViewChange(node.Consensus.ViewID() + 1)
			// }

			// if m := node.Consensus.Current.Mode(); m == consensus.Syncing ||
			// 	m == consensus.Listening {

			// }
		}
	}

	return nil

}
