package node

import (
	"context"
	"fmt"
	"time"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/pkg/errors"
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

	sufficientPeers := make(chan int, 1)
	problem := make(chan error, 1)
	needed := node.Consensus.MinPeers

	go func() {
		<-time.After(4 * time.Second)

		for {

			<-time.After(2 * time.Second)
			conns, err := node.host.CoreAPI.Swarm().Peers(context.Background())

			if err != nil {
				sufficientPeers = nil
				problem <- err
				return
			}

			count := 0
			for _, conn := range conns {
				protocols, err := node.host.IPFSNode.PeerHost.Peerstore().SupportsProtocols(
					conn.ID(), p2p.Protocol,
				)

				if err != nil {
					sufficientPeers = nil
					problem <- err
				}

				seen := false
				for _, protocol := range protocols {
					if seen = protocol == p2p.Protocol; seen {
						break
					}
				}

				if !seen {
					continue
				}
				count++
			}

			if count >= needed {
				sufficientPeers <- count
				problem = nil
				return
			}

		}

	}()

	select {
	case <-time.After(maxWaitBootstrap):
		return errors.New("took too long")
	case err := <-problem:
		return err
	case count := <-sufficientPeers:
		utils.Logger().Info().
			Int("have", count).
			Int("needed", needed).
			Msg("got enough peers for consensus")
	}

	go func() {
		if node.Consensus.IsLeader() {
			node.Consensus.ProposalNewBlock <- struct{}{}
			utils.Logger().Info().Msg("kicked off consensus as leader")
		} else {
			node.Consensus.SetNextBlockDue(
				time.Now().Add(consensus.BlockTime),
			)
		}
	}()

	timeLast := time.Now()
	tick := time.NewTicker(2 * time.Second)

	for {

		select {
		case <-node.Consensus.ResetConsensusTimeout:
			timeLast = time.Now()
		case <-tick.C:

			if node.Consensus.Current.Mode() == consensus.Normal {
				since := time.Since(timeLast).Round(time.Second)

				if since > 60*time.Second {

					fmt.Println(
						"was it more than 60 second",
						node.Consensus.PubKey.SerializeToHexStr(),
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
