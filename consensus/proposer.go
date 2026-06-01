package consensus

import (
	"time"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/node/harmony/worker"
)

const (
	// maxProposerForwardStep mirrors internal/chain.maxBlockTimeStep (12s).
	// When local wall is only moderately ahead of parent.Time, clamp the
	// proposed unix time to parent+maxProposerForwardStep so the header stays
	// inside the engine's per-block step bound relative to that parent.
	// When wall is far ahead of parent (above skew-clamp threshold), do not
	// clamp so the proposed time can follow local wall (matches engine's
	// max(parent+step, localWall) step ceiling).
	maxProposerForwardStep int64 = 12
	// maxProposerCatchupWait caps sleeping until parent.Time+1 when the leader
	// wall is behind the parent header time. Without a cap, a far-future
	// parent would block this goroutine for a long time. 17s covers waiting
	// out a parent timestamp that is still valid under allowedFutureBlockTime
	// (15s) plus typical inter-second boundary (+1s), on the same machine.
	maxProposerCatchupWait = 17 * time.Second
)

// proposalTiming describes the next wait/propose action from the leader's
// local wall time and parent header unix time only (no chain or peer state).
// Exactly one mode applies: ready (propose), sleep then retry, or giveUp.
type proposalTiming struct {
	// ready: when true, the leader should propose with timestamp = proposeAt.
	ready     bool
	proposeAt time.Time
	// sleep: when non-zero, the leader should sleep for this duration and retry.
	sleep time.Duration
	// giveUp: when true, the leader should break out of the retry loop because
	// the parent timestamp is too far in the future to wait out.
	giveUp bool
}

// computeProposalTiming is the pure decision function used by
// WaitForConsensusReadyV2. Extracted so the timing logic can be unit-tested
// without spinning up the full consensus stack.
//
// When timestampValidation is false (before TimestampValidationEpoch), only the
// legacy sleep-until-parent+1 behavior applies.
//
// When timestampValidation is true (wall := now.Unix(), parent := parentTime):
//   - wall <= parent: sleep until parent+1, capped by maxProposerCatchupWait;
//     if the sleep would exceed the cap, giveUp for this attempt.
//   - wall > parent+maxProposerForwardStep && wall-parent <= skewClampThreshold:
//     clamp to parent+maxProposerForwardStep (skew clamp band).
//   - wall > parent+skewClampThreshold: no clamp; propose at wall (stall recovery).
//   - else wall > parent && wall <= parent+maxProposerForwardStep: propose at wall.
func computeProposalTiming(now time.Time, parentTime int64, skewClampThreshold int64, timestampValidation bool) proposalTiming {
	if !timestampValidation {
		return computeLegacyProposalTiming(now, parentTime)
	}
	timestamp := now.Unix()
	if timestamp > parentTime+maxProposerForwardStep &&
		timestamp-parentTime <= skewClampThreshold {
		timestamp = parentTime + maxProposerForwardStep
		now = time.Unix(timestamp, 0)
	}
	if timestamp <= parentTime {
		waitFor := time.Unix(parentTime+1, 0).Sub(now)
		if waitFor > maxProposerCatchupWait {
			return proposalTiming{giveUp: true}
		}
		return proposalTiming{sleep: waitFor}
	}
	return proposalTiming{ready: true, proposeAt: now}
}

// computeLegacyProposalTiming is the pre-TimestampValidationEpoch leader behavior:
// sleep until parent+1 when wall is not past parent, otherwise propose at wall.
func computeLegacyProposalTiming(now time.Time, parentTime int64) proposalTiming {
	timestamp := now.Unix()
	if timestamp <= parentTime {
		return proposalTiming{sleep: time.Unix(parentTime+1, 0).Sub(now)}
	}
	return proposalTiming{ready: true, proposeAt: now}
}

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
					currentHeader := p.consensus.Blockchain().CurrentHeader()
					parentTime := currentHeader.Time().Int64()
					chainConfig := p.consensus.Blockchain().Config()
					epoch := currentHeader.Epoch()
					timestampValidation := chainConfig != nil && chainConfig.IsTimestampValidation(epoch)
					skewClamp := int64(viewChangeSlot)
					timing := computeProposalTiming(time.Now(), parentTime, skewClamp, timestampValidation)
					if timing.giveUp {
						consensus.GetLogger().Warn().
							Int64("parentTime", parentTime).
							Int64("now", time.Now().Unix()).
							Msg("[Proposer] parent timestamp too far ahead, giving up this round")
						break
					}
					if !timing.ready {
						time.Sleep(timing.sleep)
						continue
					}
					now := timing.proposeAt
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
