package stagedstreamsync

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/rs/zerolog"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	streamSyncProtocol "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	"github.com/harmony-one/harmony/shard"
)

type (
	// Downloader is responsible for sync task of one shard
	Downloader struct {
		bc                 blockChain
		nodeConfig         *nodeconfig.ConfigType
		syncProtocol       syncProtocol
		bh                 *beaconHelper
		stagedSyncInstance *StagedStreamSync

		downloadC chan struct{}
		closeC    chan struct{}
		ctx       context.Context
		cancel    context.CancelFunc

		config Config
		logger zerolog.Logger

		syncMutex sync.Mutex
		lock      sync.Mutex
	}
)

// NewDownloader creates a new downloader
func NewDownloader(host p2p.Host,
	bc core.BlockChain,
	nodeConfig *nodeconfig.ConfigType,
	consensus *consensus.Consensus,
	dbDir string,
	isBeaconNode bool,
	config Config,
	setNodeSyncStatus func(bool)) *Downloader {

	config.fixValues()
	isEpochChain := !isBeaconNode && bc.ShardID() == shard.BeaconChainShardID

	protoCfg := protocolConfig(host, bc, nodeConfig, isBeaconNode, config)
	sp := streamSyncProtocol.NewProtocol(*protoCfg)
	host.AddStreamProtocol(sp)

	// beacon nodes support epoch chain as well
	if isBeaconNode {
		epochProtoCfg := protocolConfig(host, bc, nodeConfig, isBeaconNode, config)
		epochProtoCfg.EpochChain = true
		epochChainProtocol := streamSyncProtocol.NewProtocol(*epochProtoCfg)
		host.AddStreamProtocol(epochChainProtocol)
	}

	logger := utils.Logger().With().
		Str("module", "StagedStreamSync").
		Uint32("ShardID", bc.ShardID()).
		Bool("isBeaconNode", isBeaconNode).
		Logger()

	var bh *beaconHelper
	if config.BHConfig != nil && !isBeaconNode && bc.ShardID() == shard.BeaconChainShardID {
		bh = newBeaconHelper(bc, logger, config.BHConfig.BlockC, config.BHConfig.InsertHook)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// create an instance of staged sync for the downloader
	var stagedSyncInstance *StagedStreamSync
	var err error
	if isEpochChain {
		stagedSyncInstance, err = CreateStagedEpochSync(ctx, bc, nodeConfig, consensus, dbDir, sp, config, isBeaconNode, logger, setNodeSyncStatus)
	} else {
		stagedSyncInstance, err = CreateStagedSync(ctx, bc, nodeConfig, consensus, dbDir, sp, config, isBeaconNode, logger, setNodeSyncStatus)
	}
	if err != nil {
		cancel()
		return nil
	}

	return &Downloader{
		bc:                 bc,
		nodeConfig:         nodeConfig,
		syncProtocol:       sp,
		bh:                 bh,
		stagedSyncInstance: stagedSyncInstance,

		downloadC: make(chan struct{}, 1), // It's buffered to avoid missed signals
		closeC:    make(chan struct{}),
		ctx:       ctx,
		cancel:    cancel,

		config: config,
		logger: logger,

		syncMutex: sync.Mutex{},
		lock:      sync.Mutex{},
	}
}

// protocolConfig returns protocol config
func protocolConfig(host p2p.Host,
	bc core.BlockChain,
	nodeConfig *nodeconfig.ConfigType,
	isBeaconNode bool,
	config Config) *streamSyncProtocol.Config {

	return &streamSyncProtocol.Config{
		Chain:                bc,
		Host:                 host,
		Discovery:            host.GetDiscovery(),
		ShardID:              nodeconfig.ShardID(bc.ShardID()),
		Network:              config.Network,
		BeaconNode:           isBeaconNode,
		Validator:            nodeConfig.Role() == nodeconfig.Validator,
		Explorer:             nodeConfig.Role() == nodeconfig.ExplorerNode,
		EpochChain:           !isBeaconNode && bc.ShardID() == shard.BeaconChainShardID,
		MaxAdvertiseWaitTime: config.MaxAdvertiseWaitTime,
		SmSoftLowCap:         config.SmSoftLowCap,
		SmHardLowCap:         config.SmHardLowCap,
		SmHiCap:              config.SmHiCap,
		DiscBatch:            config.SmDiscBatch,
	}
}

// Start starts the downloader
func (d *Downloader) Start() {
	if d.nodeConfig.IsOffline {
		return
	}

	go func() {
		d.waitForBootFinish()
		d.loop()
	}()

	if d.bh != nil {
		d.bh.start()
	}
}

// Close closes the downloader
func (d *Downloader) Close() {
	d.lock.Lock()
	defer d.lock.Unlock()

	select {
	case <-d.closeC:
		// Already closed
		return
	default:
		close(d.closeC)
	}

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
	case <-d.closeC:
		// Do not send if closed
	default:
	}
}

// NumPeers returns the number of peers connected of a specific shard.
func (d *Downloader) NumPeers() int {
	return d.syncProtocol.NumStreams()
}

// SyncStatus returns the current sync status
func (d *Downloader) SyncStatus() (bool, uint64, uint64) {
	syncing, target := d.stagedSyncInstance.status.Get()
	current := d.bc.CurrentBlock().NumberU64()

	// Calculate the actual difference
	var diff uint64
	if target > current {
		diff = target - current
	}

	// Return (isSynchronized, targetHeight, heightDifference)
	// where isSynchronized = !syncing (true when not syncing, false when syncing)
	return !syncing, target, diff
}

// SubscribeDownloadStarted subscribes download started
func (d *Downloader) SubscribeDownloadStarted(ch chan struct{}) event.Subscription {
	d.stagedSyncInstance.evtDownloadStartedSubscribed = true
	return d.stagedSyncInstance.evtDownloadStarted.Subscribe(ch)
}

// SubscribeDownloadFinished subscribes the download finished
func (d *Downloader) SubscribeDownloadFinished(ch chan struct{}) event.Subscription {
	d.stagedSyncInstance.evtDownloadFinishedSubscribed = true
	return d.stagedSyncInstance.evtDownloadFinished.Subscribe(ch)
}

// waitForBootFinish waits for stream manager to finish the initial discovery and have
// enough peers to start downloader
func (d *Downloader) waitForBootFinish() {
	bootCompleted, numStreams := d.waitForEnoughStreams(d.config.InitStreams)
	if bootCompleted {
		fmt.Printf("boot completed for shard %d ( %d streams are connected )\n",
			d.bc.ShardID(), numStreams)
	}
}

func (d *Downloader) waitForEnoughStreams(requiredStreams int) (bool, int) {
	d.logger.Info().Int("requiredStreams", requiredStreams).
		Msg("waiting for enough stream connections to continue syncing")

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
		select {
		case <-t.C:
			trigger()

		case <-evtCh:
			trigger()

		case <-checkCh:
			d.logger.Debug().
				Int("requiredStreams", requiredStreams).
				Int("NumStreams", d.syncProtocol.NumStreams()).
				Msg("check stream connections...")
			if d.syncProtocol.NumStreams() >= requiredStreams {
				d.logger.Info().
					Int("requiredStreams", requiredStreams).
					Int("NumStreams", d.syncProtocol.NumStreams()).
					Msg("it has enough stream connections and will continue syncing")
				return true, d.syncProtocol.NumStreams()
			}
		case <-d.closeC:
			return false, d.syncProtocol.NumStreams()
		}
	}
}

func (d *Downloader) loop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	// Helps to check if sync already in progress, skipping trigger
	var isDownloading int32

	// Shard chain and beacon chain nodes start with initSync=true
	// to ensure they first go through long-range sync.
	// Epoch chain nodes only require epoch sync.
	d.stagedSyncInstance.initSync = !d.stagedSyncInstance.isEpochChain

	trigger := func() {
		select {
		case d.downloadC <- struct{}{}: // Notify to start syncing
		case <-d.closeC: // Stop if downloader is closing
		default:
		}
	}

	// Start an initial sync trigger immediately
	go trigger()

	for {
		select {
		case <-ticker.C:
			trigger()

		case <-d.downloadC:
			if atomic.CompareAndSwapInt32(&isDownloading, 0, 1) {
				go func() {
					defer atomic.StoreInt32(&isDownloading, 0)
					d.handleDownload(trigger)
				}()
			}

		case <-d.closeC:
			return
		}
	}
}

func (d *Downloader) handleDownload(trigger func()) {
	d.syncMutex.Lock()
	defer d.syncMutex.Unlock()

	// if it's leader, skip syncing for now
	if d.stagedSyncInstance.consensus != nil && d.stagedSyncInstance.consensus.IsLeader() {
		// Retry sync after 1 seconds
		go func() {
			time.Sleep(1 * time.Second)
			trigger()
		}()
		return
	}

	var estimatedHeight uint64
	var addedBN int
	var rangeSwitched bool
	var err error

	// Perform sync and get estimated height and blocks added
	if d.stagedSyncInstance.isEpochChain {
		addedBN, err = d.stagedSyncInstance.doEpochSync(d.ctx)
	} else {
		estimatedHeight, addedBN, rangeSwitched, err = d.stagedSyncInstance.doSync(d.ctx)
	}

	switch err {
	case nil:
		// If new blocks were added, trigger another sync and process last-mile blocks
		if addedBN != 0 || rangeSwitched {
			trigger()
			if d.bh != nil {
				d.bh.insertSync()
			}
		}

	case ErrNotEnoughStreams:
		// Log sync failure and retry after a short delay
		d.logger.Error().
			Err(err).
			Bool("initSync", d.stagedSyncInstance.initSync).
			Uint64("estimated height", estimatedHeight).
			Msg(WrapStagedSyncMsg("sync loop failed"))
		// Wait for enough available streams before retrying
		d.waitForEnoughStreams(d.config.MinStreams)
		trigger()
		return

	case ErrInvalidEarlySync:
		if d.NumPeers() < d.config.Concurrency {
			// Wait for enough available streams before retrying
			d.waitForEnoughStreams(d.config.MinStreams)
		}
		// Retry sync after 10 seconds
		go func() {
			time.Sleep(10 * time.Second)
			trigger()
		}()

	default:
		if d.NumPeers() < d.config.MinStreams {
			// Wait for enough available streams before retrying
			d.waitForEnoughStreams(d.config.MinStreams)
		}
		// Handle unresolvable bad blocks
		if d.stagedSyncInstance.invalidBlock.Active {
			numTriedStreams := len(d.stagedSyncInstance.invalidBlock.StreamID)

			// If multiple streams fail to resolve the bad block, mark it as unresolvable
			if numTriedStreams >= d.config.InitStreams && !d.stagedSyncInstance.invalidBlock.IsLogged {
				d.logger.Error().
					Uint64("bad block number", d.stagedSyncInstance.invalidBlock.Number).
					Str("bad block hash", d.stagedSyncInstance.invalidBlock.Hash.String()).
					Msg(WrapStagedSyncMsg("unresolvable bad block"))
				d.stagedSyncInstance.invalidBlock.IsLogged = true

				// TODO: If no new untried streams exist, consider sleeping or panicking
			}
		}

		// Log sync failure and retry after a short delay
		d.logger.Error().
			Err(err).
			Bool("initSync", d.stagedSyncInstance.initSync).
			Uint64("estimated height", estimatedHeight).
			Msg(WrapStagedSyncMsg("sync loop failed"))

		// Retry sync after 5 seconds
		go func() {
			// if rangeSwitched, don't wait for 5 seconds
			if !rangeSwitched {
				time.Sleep(5 * time.Second)
			}
			trigger()
		}()
	}
}
