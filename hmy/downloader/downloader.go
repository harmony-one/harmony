package downloader

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/core"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	"github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	"github.com/rs/zerolog"
)

type (
	// Downloader is responsible for sync task of one shard
	Downloader struct {
		bc           blockChain
		syncProtocol syncProtocol

		downloadC chan struct{}
		closeC    chan struct{}
		ctx       context.Context
		cancel    func()

		evtDownloadFinished event.Feed // channel for each download task finished

		config Config
		logger zerolog.Logger
	}
)

// NewDownloader creates a new downloader
func NewDownloader(host p2p.Host, bc *core.BlockChain, config Config) *Downloader {
	config.fixValues()

	syncProtocol := sync.NewProtocol(sync.Config{
		Chain:     bc,
		Host:      host.GetP2PHost(),
		Discovery: host.GetDiscovery(),
		ShardID:   nodeconfig.ShardID(bc.ShardID()),
		Network:   config.Network,

		SmSoftLowCap: config.SmSoftLowCap,
		SmHardLowCap: config.SmHardLowCap,
		SmHiCap:      config.SmHiCap,
		DiscBatch:    config.SmDiscBatch,
	})
	host.AddStreamProtocol(syncProtocol)
	ctx, cancel := context.WithCancel(context.Background())

	return &Downloader{
		bc:           bc,
		syncProtocol: syncProtocol,

		downloadC: make(chan struct{}),
		closeC:    make(chan struct{}),
		ctx:       ctx,
		cancel:    cancel,

		config: config,
		logger: utils.Logger().With().Str("module", "downloader").Logger(),
	}
}

// Start start the downloader
func (d *Downloader) Start() {
	go d.run()
}

// Close close the downloader
func (d *Downloader) Close() {
	close(d.closeC)
	d.cancel()
}

// DownloadAsync triggers the download async. If there is already a download task that is
// in progress, return ErrDownloadInProgress.
func (d *Downloader) DownloadAsync() {
	select {
	case d.downloadC <- struct{}{}:
	case <-time.After(100 * time.Millisecond):
	}
}

// SubscribeDownloadFinishedEvent subscribe the download finished
func (d *Downloader) SubscribeDownloadFinished(ch chan struct{}) event.Subscription {
	return d.evtDownloadFinished.Subscribe(ch)
}

func (d *Downloader) run() {
	d.waitForBootFinish()
	d.loop()
}

// waitForBootFinish wait for stream manager to finish the initial discovery and have
// enough peers to start downloader
func (d *Downloader) waitForBootFinish() {
	ch := make(chan streammanager.EvtStreamAdded, 1)
	sub := d.syncProtocol.SubscribeAddStreamEvent(ch)
	defer sub.Unsubscribe()

	t := time.NewTicker(10 * time.Second)

	for {
		d.logger.Info().Msg("waiting for initial bootstrap discovery")
		select {
		case <-t.C:
			// continue and log the message
		case <-ch:
			if d.syncProtocol.NumStreams() >= d.config.InitStreams {
				return
			}
		case <-d.closeC:
			return
		}
	}
}

func (d *Downloader) loop() {
	ticker := time.NewTicker(60 * time.Second)
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
			}
			d.logger.Info().Bool("initSync", initSync).Msg("sync finished")

			if addedBN != 0 {
				// If block number has been changed, trigger another sync
				go trigger()
			}
			if !initSync {
				// If we are doing short sync, we may want consensus to help us with
				// a couple of latest blocks
				d.evtDownloadFinished.Send(struct{}{})
			}

			initSync = false

		case <-d.closeC:
			return
		}
	}
}

func (d *Downloader) doDownload(initSync bool) (n int, err error) {
	if initSync {
		n, err = d.doLongRangeSync()
	} else {
		n, err = d.doShortRangeSync()
	}
	if err != nil {
		return
	}
	return
}
