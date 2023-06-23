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
	SubscribeDownloadStarted(ch chan struct{}) event.Subscription
	DownloadAsync()
}

// Set downloader set the downloader of the shard to consensus
// TODO: It will be better to move this to consensus.New and register consensus as a service
func (consensus *Consensus) SetDownloader(d downloader) {
	consensus.dHelper = newDownloadHelper(consensus, d)
}

type downloadHelper struct {
	d downloader
	c *Consensus

	startedCh  chan struct{}
	finishedCh chan struct{}

	startedSub  event.Subscription
	finishedSub event.Subscription
}

func newDownloadHelper(c *Consensus, d downloader) *downloadHelper {
	startedCh := make(chan struct{}, 1)
	startedSub := d.SubscribeDownloadStarted(startedCh)

	finishedCh := make(chan struct{}, 1)
	finishedSub := d.SubscribeDownloadFinished(finishedCh)

	return &downloadHelper{
		c:           c,
		d:           d,
		startedCh:   startedCh,
		finishedCh:  finishedCh,
		startedSub:  startedSub,
		finishedSub: finishedSub,
	}
}

func (dh *downloadHelper) start() {
	go dh.downloadStartedLoop()
	go dh.downloadFinishedLoop()
}

func (dh *downloadHelper) close() {
	dh.startedSub.Unsubscribe()
	dh.finishedSub.Unsubscribe()
}

func (dh *downloadHelper) downloadStartedLoop() {
	for {
		select {
		case <-dh.startedCh:
			dh.c.BlocksNotSynchronized()

		case err := <-dh.startedSub.Err():
			dh.c.getLogger().Info().Err(err).Msg("consensus download finished loop closed")
			return
		}
	}
}

func (dh *downloadHelper) downloadFinishedLoop() {
	for {
		select {
		case <-dh.finishedCh:
			err := dh.c.AddConsensusLastMile()
			if err != nil {
				dh.c.getLogger().Error().Err(err).Msg("add last mile failed")
			}
			dh.c.BlocksSynchronized()

		case err := <-dh.finishedSub.Err():
			dh.c.getLogger().Info().Err(err).Msg("consensus download finished loop closed")
			return
		}
	}
}

func (consensus *Consensus) AddConsensusLastMile() error {
	curBN := consensus.Blockchain().CurrentBlock().NumberU64()
	err := consensus.GetLastMileBlockIter(curBN+1, func(blockIter *LastMileBlockIter) error {
		for {
			block := blockIter.Next()
			if block == nil {
				break
			}
			if _, err := consensus.Blockchain().InsertChain(types.Blocks{block}, true); err != nil {
				return errors.Wrap(err, "failed to InsertChain")
			}
		}
		return nil
	})
	return err
}

func (consensus *Consensus) spinUpStateSync() {
	if consensus.dHelper != nil {
		consensus.dHelper.d.DownloadAsync()
		consensus.current.SetMode(Syncing)
		for _, v := range consensus.consensusTimeout {
			v.Stop()
		}
	} else {
		select {
		case consensus.BlockNumLowChan <- struct{}{}:
			consensus.current.SetMode(Syncing)
			for _, v := range consensus.consensusTimeout {
				v.Stop()
			}
		default:
		}
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
