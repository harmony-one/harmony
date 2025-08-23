package consensus

import (
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/core"
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
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.dHelper = newDownloadHelper(consensus, d)
}

type downloadHelper struct {
	d downloader

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

	out := &downloadHelper{
		d:           d,
		startedCh:   startedCh,
		finishedCh:  finishedCh,
		startedSub:  startedSub,
		finishedSub: finishedSub,
	}
	go out.downloadStartedLoop(c)
	go out.downloadFinishedLoop(c)
	return out
}

func (dh *downloadHelper) DownloadAsync() {
	dh.d.DownloadAsync()
}

func (dh *downloadHelper) downloadStartedLoop(c *Consensus) {
	for {
		select {
		case <-dh.startedCh:
			c.BlocksNotSynchronized("downloadStartedLoop")

		case err := <-dh.startedSub.Err():
			c.GetLogger().Info().Err(err).Msg("consensus download finished loop closed")
			return
		}
	}
}

func (dh *downloadHelper) downloadFinishedLoop(c *Consensus) {
	for {
		select {
		case <-dh.finishedCh:
			c.BlocksSynchronized("downloadFinishedLoop")

		case err := <-dh.finishedSub.Err():
			c.GetLogger().Info().Err(err).Msg("consensus download finished loop closed")
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
			_, err := consensus.Blockchain().InsertChain(types.Blocks{block}, true)
			switch {
			case errors.Is(err, core.ErrKnownBlock):
			case errors.Is(err, core.ErrNotLastBlockInEpoch):
			case err != nil:
				return errors.Wrap(err, "failed to InsertChain")
			}
		}
		return nil
	})
	return err
}

func (consensus *Consensus) spinUpStateSync() {
	// CRITICAL: Immediate consensus mode change to "Syncing" is essential to prevent race conditions.
	//
	// Here's why this is needed:
	// 1. When consensus detects the node is out of sync (e.g., receives block with higher number),
	//    it calls spinUpStateSync() immediately
	// 2. However, the sync detection loop only runs every 60 seconds (SyncFrequency)
	// 3. During this 60-second window, the node continues to participate in consensus with mode "Normal"
	// 4. This leads to "unknown ancestor" errors because the node tries to verify blocks it can't handle
	// 5. The event system (DownloadAsync -> startSyncing -> evtDownloadStarted) eventually changes the mode,
	//    but there's a race condition where consensus messages are processed before the mode changes
	//
	// By immediately setting the mode to "Syncing", we prevent the node from participating in consensus
	// while it's out of sync, eliminating the race condition and preventing "unknown ancestor" errors.
	consensus.BlocksNotSynchronized("spinUpStateSync")

	// Start the download process
	consensus.dHelper.DownloadAsync()

	// Stop all consensus timeouts since we're syncing
	for _, v := range consensus.consensusTimeout {
		v.Stop()
	}
}
