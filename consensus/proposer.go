package consensus

import (
	"time"

	"github.com/harmony-one/harmony/crypto/bls"
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

		consensus.GetLogger().Debug().
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
				consensus.GetLogger().Warn().
					Msg("Consensus new block proposal: STOPPED!")
				return
			case proposal := <-consensus.GetReadySignal():
				for retryCount := 0; retryCount < 3 && consensus.IsLeader(); retryCount++ {
					var (
						currentHeader = p.consensus.Blockchain().CurrentHeader()
						parentTime    = currentHeader.Time().Int64()
						now           = consensus.registry.Now()
						timestamp     = now.Unix()
					)
					// Block timestamps are validated at second precision and must strictly increase.
					// We decide whether we can propose using consensus.registry.Now(), which is the
					// NTP-adjusted consensus clock. Do not use time.Until(target) here: time.Until()
					// is based on raw local time.Now(), and local time can be a few milliseconds
					// ahead/behind registry.Now().
					//
					// Near the next-second boundary this matters:
					//   registry.Now() may still be 24.999s while time.Now() is already 25.004s.
					// If we sleep using local time, the sleep duration can be <= 0 even though the
					// consensus clock still has not crossed parentTime+1. That can burn all retries,
					// skip proposal setup, and leave finalCommit with no receiver for commitSigAndBitmap.
					//
					// Therefore compute the delay using the same clock used for the timestamp check,
					// then re-read registry.Now() after sleeping before deciding whether to continue.
					if timestamp <= parentTime {
						target := time.Unix(parentTime+1, 0)

						if delay := target.Sub(consensus.registry.Now()); delay > 0 {
							time.Sleep(delay)
						}

						now = consensus.registry.Now()
						timestamp = now.Unix()

						if timestamp <= parentTime {
							consensus.GetLogger().Warn().
								Int64("parentTimeUnix", parentTime).
								Time("registryNow", now).
								Time("targetTime", target).
								Int64("registryTimestampUnix", timestamp).
								Int("retryCount", retryCount).
								Msg("[timestamp-guard] registry clock still not past parent timestamp after sleep")

							continue
						}
					}
					consensus.GetLogger().Info().
						Uint64("blockNum", proposal.blockNum).
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
								consensus.GetLogger().Info().Msg("[ProposeNewBlock] Sync block proposal, reading commit sigs directly from DB")
							} else {
								consensus.GetLogger().Info().Msg("[ProposeNewBlock] Timeout waiting for commit sigs, reading directly from DB")
							}
							sigs, err := consensus.BlockCommitSigs(consensus.Blockchain().CurrentBlock().NumberU64())

							if err != nil {
								consensus.GetLogger().Error().Err(err).Msg("[ProposeNewBlock] Cannot get commit signatures from last block")
							} else {
								newCommitSigsChan <- sigs
							}
						case commitSigs := <-consensus.GetCommitSigChannel():
							consensus.GetLogger().Info().Msg("[ProposeNewBlock] received commit sigs asynchronously")
							if len(commitSigs) > bls.BLSSignatureSizeInBytes {
								newCommitSigsChan <- commitSigs
							}
						}
					}()
					newBlock, err := consensus.ProposeNewBlock(now, newCommitSigsChan)
					if err == nil {
						consensus.GetLogger().Info().
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
						consensus.GetLogger().Err(err).Int("retryCount", retryCount).
							Msg("!!!!!!!!!Failed Proposing New Block!!!!!!!!!")
						continue
					}
				}
			}
		}
	}()
}
