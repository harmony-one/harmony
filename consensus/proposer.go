package consensus

import (
	"time"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/harmony/worker"
)

type Proposer struct {
	consensus *Consensus
}

func NewProposer(consensus *Consensus) *Proposer {
	return &Proposer{consensus}
}

// WaitForConsensusReadyV2 listen for the readiness signal from consensus and generate new block for consensus.
// only leader will receive the ready signal
func (p *Proposer) WaitForConsensusReadyV2(stopChan chan struct{}, stoppedChan chan struct{}) {
	consensus := p.consensus
	go func() {
		// Setup stoppedChan
		defer close(stoppedChan)

		utils.Logger().Debug().
			Msg("Waiting for Consensus ready")
		select {
		case <-time.After(30 * time.Second):
		case <-stopChan:
			return
		}

		for {
			// keep waiting for Consensus ready
			select {
			case <-stopChan:
				utils.Logger().Warn().
					Msg("Consensus new block proposal: STOPPED!")
				return
			case proposal := <-consensus.GetReadySignal():
				for retryCount := 0; retryCount < 3 && consensus.IsLeader(); retryCount++ {
					time.Sleep(SleepPeriod)
					utils.Logger().Info().
						Uint64("blockNum", consensus.Blockchain().CurrentBlock().NumberU64()+1).
						Bool("asyncProposal", proposal.Type == AsyncProposal).
						Str("called", proposal.Caller).
						Msg("PROPOSING NEW BLOCK ------------------------------------------------")

					// Prepare last commit signatures
					newCommitSigsChan := make(chan []byte)

					go func() {
						waitTime := 0 * time.Second
						if proposal.Type == AsyncProposal {
							waitTime = worker.CommitSigReceiverTimeout
						}
						select {
						case <-time.After(waitTime):
							if waitTime == 0 {
								utils.Logger().Info().Msg("[ProposeNewBlock] Sync block proposal, reading commit sigs directly from DB")
							} else {
								utils.Logger().Info().Msg("[ProposeNewBlock] Timeout waiting for commit sigs, reading directly from DB")
							}
							sigs, err := consensus.BlockCommitSigs(consensus.Blockchain().CurrentBlock().NumberU64())

							if err != nil {
								utils.Logger().Error().Err(err).Msg("[ProposeNewBlock] Cannot get commit signatures from last block")
							} else {
								newCommitSigsChan <- sigs
							}
						case commitSigs := <-consensus.GetCommitSigChannel():
							utils.Logger().Info().Msg("[ProposeNewBlock] received commit sigs asynchronously")
							if len(commitSigs) > bls.BLSSignatureSizeInBytes {
								newCommitSigsChan <- commitSigs
							}
						}
					}()
					newBlock, err := consensus.ProposeNewBlock(newCommitSigsChan)
					if err == nil {
						utils.Logger().Info().
							Uint64("blockNum", newBlock.NumberU64()).
							Uint64("epoch", newBlock.Epoch().Uint64()).
							Uint64("viewID", newBlock.Header().ViewID().Uint64()).
							Int("numTxs", newBlock.Transactions().Len()).
							Int("numStakingTxs", newBlock.StakingTransactions().Len()).
							Int("crossShardReceipts", newBlock.IncomingReceipts().Len()).
							Msgf("=========Successfully Proposed New Block, shard: %d epoch: %d number: %d ==========", newBlock.ShardID(), newBlock.Epoch().Uint64(), newBlock.NumberU64())

						// Send the new block to Consensus so it can be confirmed.
						consensus.BlockChannel(newBlock)
						break
					} else {
						utils.Logger().Err(err).Int("retryCount", retryCount).
							Msg("!!!!!!!!!Failed Proposing New Block!!!!!!!!!")
						continue
					}
				}
			}
		}
	}()
}
