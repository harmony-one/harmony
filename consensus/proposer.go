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

// StartCheckingForNewProposals checks the proposal queue and generate new block for consensus.
// only leader will receive the ready signal
func (p *Proposer) StartCheckingForNewProposals(stopChan chan struct{}, stoppedChan chan struct{}) {
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
			//case proposal := <-consensus.GetReadySignal():

			case <-time.NewTicker(100 * time.Millisecond).C:
				if !consensus.proposalManager.IsReady() {
					continue
				}
				numProposalsInQueue := consensus.proposalManager.Length()
				if numProposalsInQueue == 0 {
					continue
				}
				proposal, errNewProposal := consensus.proposalManager.GetNextProposal()
				if errNewProposal != nil {
					utils.Logger().Debug().Err(errNewProposal).Msg("[ProposeNewBlock] Cannot get next proposal")
				}
				if proposal == nil {
					continue
				}
				if err := p.CreateProposal(proposal); err != nil {
					utils.Logger().Warn().Err(err).
						Str("Caller", proposal.Caller).
						Uint64("Height", proposal.Height).
						Uint64("ViewID", proposal.ViewID).
						Msg("[ProposeNewBlock] proposal creation failed")
				}
			}
		}
	}()
}

func (p *Proposer) CreateProposal(proposal *Proposal) error {

	consensus := p.consensus
	if consensus == nil {
		utils.Logger().Warn().Msg("[CreateProposal] trying to create a new block proposal while consensus is not initialized yet")
		return nil
	}

	// set proposal manager status status
	consensus.proposalManager.SetToCreatingNewProposalMode()
	defer func() {
		if !consensus.proposalManager.IsWaitingForCommitSigs() {
			consensus.proposalManager.Done()
		}
	}()

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
			if waitTime > 0 {
				consensus.proposalManager.SetToWaitingForCommitSigsMode()
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
				if waitTime > 0 {
					consensus.proposalManager.Done()
				}
			case commitSigs := <-consensus.GetCommitSigChannel():
				utils.Logger().Info().Msg("[ProposeNewBlock] received commit sigs asynchronously")
				if len(commitSigs) > bls.BLSSignatureSizeInBytes {
					newCommitSigsChan <- commitSigs
					if waitTime > 0 {
						consensus.proposalManager.Done()
					}
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
	return nil
}
