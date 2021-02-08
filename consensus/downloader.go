package consensus

import (
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/core/types"
	"github.com/pkg/errors"
)

// downloader is the adapter interface for downloader.Downloader, which is used for
// 1. Subscribe download finished event to help syncing to the latest block.
// 2. Trigger the downloader to start working
type downloader interface {
	SubscribeDownloadFinished(ch chan struct{}) event.Subscription
	DownloadAsync()
}

// Set downloader set the downloader of the shard to consensus
// TODO: It will be better to move this to consensus.New and register consensus as a service
func (consensus *Consensus) SetDownloader(d downloader) {
	consensus.downloader = d

	consensus.downloadCh = make(chan struct{})
	consensus.downloadSub = d.SubscribeDownloadFinished(consensus.downloadCh)
}

func (consensus *Consensus) downloadFinishedLoop() {
	for {
		select {
		case <-consensus.downloadCh:
			err := consensus.addConsensusLastMile()
			if err != nil {
				consensus.getLogger().Error().Err(err).Msg("add last mile failed")
			}

		case err := <-consensus.downloadSub.Err():
			consensus.getLogger().Info().Err(err).Msg("consensus download finished loop closed")
			return
		}
	}
}

func (consensus *Consensus) addConsensusLastMile() error {
	curBN := consensus.Blockchain.CurrentBlock().NumberU64()
	blockIter, err := consensus.GetLastMileBlockIter(curBN + 1)
	if err != nil {
		return err
	}
	for {
		block := blockIter.Next()
		if block == nil {
			break
		}
		if _, err := consensus.Blockchain.InsertChain(types.Blocks{block}, true); err != nil {
			return errors.Wrap(err, "failed to InsertChain")
		}
	}
	return nil
}

func (consensus *Consensus) spinUpStateSync() {
	if consensus.downloader != nil {
		consensus.downloader.DownloadAsync()
		consensus.current.SetMode(Syncing)
		for _, v := range consensus.consensusTimeout {
			v.Stop()
		}
	} else {
		consensus.spinLegacyStateSync()
	}
}

func (consensus *Consensus) spinLegacyStateSync() {
	select {
	case consensus.BlockNumLowChan <- struct{}{}:
		consensus.current.SetMode(Syncing)
		for _, v := range consensus.consensusTimeout {
			v.Stop()
		}
	default:
	}
}
