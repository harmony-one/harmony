package downloader

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	"github.com/harmony-one/harmony/p2p/stream/protocols/sync"
)

type (
	// Downloader is responsible for sync task of one shard
	Downloader struct {
		bc           blockChain
		syncProtocol syncProtocol
		bh           *beaconHelper

		downloadC chan struct{}
		closeC    chan struct{}
		ctx       context.Context
		cancel    func()

		evtDownloadFinished           event.Feed // channel for each download task finished
		evtDownloadFinishedSubscribed bool
		evtDownloadStarted            event.Feed // channel for each download has started
		evtDownloadStartedSubscribed  bool

		status status
		config Config
		logger zerolog.Logger
	}
)

// NewDownloader creates a new downloader
func NewDownloader(host p2p.Host, bc core.BlockChain, isBeaconNode bool, config Config) *Downloader {
	config.fixValues()

	sp := sync.NewProtocol(sync.Config{
		Chain:      bc,
		Host:       host.GetP2PHost(),
		Discovery:  host.GetDiscovery(),
		ShardID:    nodeconfig.ShardID(bc.ShardID()),
		Network:    config.Network,
		BeaconNode: isBeaconNode,

		SmSoftLowCap: config.SmSoftLowCap,
		SmHardLowCap: config.SmHardLowCap,
		SmHiCap:      config.SmHiCap,
		DiscBatch:    config.SmDiscBatch,
	})
	host.AddStreamProtocol(sp)

	var bh *beaconHelper
	if config.BHConfig != nil && bc.ShardID() == 0 {
		bh = newBeaconHelper(bc, config.BHConfig.BlockC, config.BHConfig.InsertHook)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Downloader{
		bc:           bc,
		syncProtocol: sp,
		bh:           bh,

		downloadC: make(chan struct{}),
		closeC:    make(chan struct{}),
		ctx:       ctx,
		cancel:    cancel,

		status: newStatus(),
		config: config,
		logger: utils.Logger().With().Str("module", "downloader").Uint32("ShardID", bc.ShardID()).Logger(),
	}
}

// Start start the downloader
func (d *Downloader) Start() {
	go d.run()

	if d.bh != nil {
		d.bh.start()
	}
}

// Close close the downloader
func (d *Downloader) Close() {
	close(d.closeC)
	d.cancel()

	if d.bh != nil {
		d.bh.close()
	}
}

// DownloadAsync triggers the download async.
func (d *Downloader) DownloadAsync() {
	select {
	case d.downloadC <- struct{}{}:
		consensusTriggeredDownloadCounterVec.With(d.promLabels()).Inc()

	case <-time.After(100 * time.Millisecond):
	}
}

// NumPeers returns the number of peers connected of a specific shard.
func (d *Downloader) NumPeers() int {
	return d.syncProtocol.NumStreams()
}

// IsSyncing return the current sync status
func (d *Downloader) SyncStatus() (bool, uint64, uint64) {
	current := d.bc.CurrentBlock().NumberU64()
	syncing, target := d.status.get()
	if !syncing { // means synced
		target = current
	}
	// isSyncing, target, blocks to target
	return syncing, target, target - current
}

// SubscribeDownloadStarted subscribe download started
func (d *Downloader) SubscribeDownloadStarted(ch chan struct{}) event.Subscription {
	d.evtDownloadStartedSubscribed = true
	return d.evtDownloadStarted.Subscribe(ch)
}

// SubscribeDownloadFinished subscribe the download finished
func (d *Downloader) SubscribeDownloadFinished(ch chan struct{}) event.Subscription {
	d.evtDownloadFinishedSubscribed = true
	return d.evtDownloadFinished.Subscribe(ch)
}

func (d *Downloader) run() {
	d.waitForBootFinish()
	d.loop()
}

// waitForBootFinish wait for stream manager to finish the initial discovery and have
// enough peers to start downloader
func (d *Downloader) waitForBootFinish() {
	evtCh := make(chan streammanager.EvtStreamAdded, 1)
	sub := d.syncProtocol.SubscribeAddStreamEvent(evtCh)
	defer sub.Unsubscribe()

	checkCh := make(chan struct{}, 1)
	trigger := func() {
		select {
		case checkCh <- struct{}{}:
		default:
		}
	}
	trigger()

	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		d.logger.Info().Msg("waiting for initial bootstrap discovery")
		select {
		case <-t.C:
			trigger()

		case <-evtCh:
			trigger()

		case <-checkCh:
			if d.syncProtocol.NumStreams() >= d.config.InitStreams {
				return
			}
		case <-d.closeC:
			return
		}
	}
}

func (d *Downloader) loop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	initSync := true
	trigger := func() {
		select {
		case d.downloadC <- struct{}{}:
		case <-time.After(100 * time.Millisecond):
		}
	}
	go trigger()

	for {
		select {
		case <-ticker.C:
			go trigger()

		case <-d.downloadC:
			addedBN, err := d.doDownload(initSync)
			if err != nil {
				// If error happens, sleep 5 seconds and retry
				d.logger.Warn().Err(err).Bool("bootstrap", initSync).Msg("failed to download")
				go func() {
					time.Sleep(5 * time.Second)
					trigger()
				}()
				time.Sleep(1 * time.Second)
				continue
			}
			d.logger.Info().Int("block added", addedBN).
				Uint64("current height", d.bc.CurrentBlock().NumberU64()).
				Bool("initSync", initSync).
				Uint32("shard", d.bc.ShardID()).
				Msg("sync finished")

			if addedBN != 0 {
				// If block number has been changed, trigger another sync
				// and try to add last mile from pub-sub (blocking)
				go trigger()
				if d.bh != nil {
					d.bh.insertSync()
				}
			}
			initSync = false

		case <-d.closeC:
			return
		}
	}
}

func (d *Downloader) doDownload(initSync bool) (n int, err error) {
	if initSync {
		d.logger.Info().Uint64("current number", d.bc.CurrentBlock().NumberU64()).
			Uint32("shard ID", d.bc.ShardID()).Msg("start long range sync")

		n, err = d.doLongRangeSync()
	} else {
		d.logger.Info().Uint64("current number", d.bc.CurrentBlock().NumberU64()).
			Uint32("shard ID", d.bc.ShardID()).Msg("start short range sync")

		n, err = d.doShortRangeSync()
	}
	if err != nil {
		pl := d.promLabels()
		pl["error"] = err.Error()
		numFailedDownloadCounterVec.With(pl).Inc()
		return
	}
	return
}

func (d *Downloader) startSyncing() {
	d.status.startSyncing()
	if d.evtDownloadStartedSubscribed {
		d.evtDownloadStarted.Send(struct{}{})
	}
}

func (d *Downloader) finishSyncing() {
	d.status.finishSyncing()
	if d.evtDownloadFinishedSubscribed {
		d.evtDownloadFinished.Send(struct{}{})
	}
}

var emptySigVerifyErr *sigVerifyErr

type sigVerifyErr struct {
	err error
}

func (e *sigVerifyErr) Error() string {
	return fmt.Sprintf("[VerifyHeaderSignature] %v", e.err.Error())
}

func verifyAndInsertBlocks(bc blockChain, blocks types.Blocks) (int, error) {
	for i, block := range blocks {
		if err := verifyAndInsertBlock(bc, block, blocks[i+1:]...); err != nil {
			return i, err
		}
	}
	return len(blocks), nil
}

func verifyAndInsertBlock(bc blockChain, block *types.Block, nextBlocks ...*types.Block) error {
	var (
		sigBytes bls.SerializedSignature
		bitmap   []byte
		err      error
	)
	if len(nextBlocks) > 0 {
		// get commit sig from the next block
		next := nextBlocks[0]
		sigBytes = next.Header().LastCommitSignature()
		bitmap = next.Header().LastCommitBitmap()
	} else {
		// get commit sig from current block
		sigBytes, bitmap, err = chain.ParseCommitSigAndBitmap(block.GetCurrentCommitSig())
		if err != nil {
			return errors.Wrap(err, "parse commitSigAndBitmap")
		}
	}

	if err := bc.Engine().VerifyHeaderSignature(bc, block.Header(), sigBytes, bitmap); err != nil {
		return &sigVerifyErr{err}
	}
	if err := bc.Engine().VerifyHeader(bc, block.Header(), true); err != nil {
		return errors.Wrap(err, "[VerifyHeader]")
	}
	if _, err := bc.InsertChain(types.Blocks{block}, false); err != nil {
		return errors.Wrap(err, "[InsertChain]")
	}
	return nil
}
