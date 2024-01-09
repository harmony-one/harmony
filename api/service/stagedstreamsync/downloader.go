package stagedstreamsync

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/rs/zerolog"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	"github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	"github.com/harmony-one/harmony/shard"
)

type (
	// Downloader is responsible for sync task of one shard
	Downloader struct {
		bc                 blockChain
		syncProtocol       syncProtocol
		bh                 *beaconHelper
		stagedSyncInstance *StagedStreamSync
		isBeaconNode       bool

		downloadC chan struct{}
		closeC    chan struct{}
		ctx       context.Context
		cancel    context.CancelFunc

		config Config
		logger zerolog.Logger
	}
)

// NewDownloader creates a new downloader
func NewDownloader(host p2p.Host, bc core.BlockChain, consensus *consensus.Consensus, dbDir string, isBeaconNode bool, config Config) *Downloader {
	config.fixValues()

	sp := sync.NewProtocol(sync.Config{
		Chain:                bc,
		Host:                 host.GetP2PHost(),
		Discovery:            host.GetDiscovery(),
		ShardID:              nodeconfig.ShardID(bc.ShardID()),
		Network:              config.Network,
		BeaconNode:           isBeaconNode,
		MaxAdvertiseWaitTime: config.MaxAdvertiseWaitTime,
		SmSoftLowCap:         config.SmSoftLowCap,
		SmHardLowCap:         config.SmHardLowCap,
		SmHiCap:              config.SmHiCap,
		DiscBatch:            config.SmDiscBatch,
	})

	host.AddStreamProtocol(sp)

	var bh *beaconHelper
	if config.BHConfig != nil && bc.ShardID() == shard.BeaconChainShardID {
		bh = newBeaconHelper(bc, config.BHConfig.BlockC, config.BHConfig.InsertHook)
	}

	logger := utils.Logger().With().
		Str("module", "staged stream sync").
		Uint32("ShardID", bc.ShardID()).Logger()

	ctx, cancel := context.WithCancel(context.Background())

	// create an instance of staged sync for the downloader
	stagedSyncInstance, err := CreateStagedSync(ctx, bc, consensus, dbDir, isBeaconNode, sp, config, logger)
	if err != nil {
		cancel()
		return nil
	}

	return &Downloader{
		bc:                 bc,
		syncProtocol:       sp,
		bh:                 bh,
		stagedSyncInstance: stagedSyncInstance,
		isBeaconNode:       isBeaconNode,

		downloadC: make(chan struct{}),
		closeC:    make(chan struct{}),
		ctx:       ctx,
		cancel:    cancel,

		config: config,
		logger: logger,
	}
}

// Start starts the downloader
func (d *Downloader) Start() {
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

// SyncStatus returns the current sync status
func (d *Downloader) SyncStatus() (bool, uint64, uint64) {
	syncing, target := d.stagedSyncInstance.status.get()
	if !syncing {
		target = d.bc.CurrentBlock().NumberU64()
	}
	return syncing, target, 0
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
			if d.syncProtocol.NumStreams() >= requiredStreams {
				return true, d.syncProtocol.NumStreams()
			}
		case <-d.closeC:
			return false, d.syncProtocol.NumStreams()
		}
	}
}

func (d *Downloader) loop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// for shard chain and beacon chain node, first we start with initSync=true to
	// make sure it goes through the long range sync first.
	// for epoch chain we do only need to go through epoch sync process
	initSync := d.isBeaconNode || d.bc.ShardID() != shard.BeaconChainShardID

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
			bnBeforeSync := d.bc.CurrentBlock().NumberU64()
			estimatedHeight, addedBN, err := d.stagedSyncInstance.doSync(d.ctx, initSync)
			if err == ErrNotEnoughStreams {
				d.waitForEnoughStreams(d.config.MinStreams)
			}
			if err != nil {
				//TODO: if there is a bad block which can't be resolved
				if d.stagedSyncInstance.invalidBlock.Active {
					numTriedStreams := len(d.stagedSyncInstance.invalidBlock.StreamID)
					// if many streams couldn't solve it, then that's an unresolvable bad block
					if numTriedStreams >= d.config.InitStreams {
						if !d.stagedSyncInstance.invalidBlock.IsLogged {
							d.logger.Error().
								Uint64("bad block number", d.stagedSyncInstance.invalidBlock.Number).
								Msg(WrapStagedSyncMsg("unresolvable bad block"))
							d.stagedSyncInstance.invalidBlock.IsLogged = true
						}
						//TODO: if we don't have any new or untried stream in the list, sleep or panic
					}
				}
				// If any error happens, sleep 5 seconds and retry
				d.logger.Error().
					Err(err).
					Bool("initSync", initSync).
					Msg(WrapStagedSyncMsg("sync loop failed"))
				go func() {
					time.Sleep(5 * time.Second)
					trigger()
				}()
				time.Sleep(1 * time.Second)
				break
			}
			if initSync {
				d.logger.Info().Int("block added", addedBN).
					Uint64("current height", d.bc.CurrentBlock().NumberU64()).
					Bool("initSync", initSync).
					Uint32("shard", d.bc.ShardID()).
					Msg(WrapStagedSyncMsg("sync finished"))
			}
			// If block number has been changed, trigger another sync
			if addedBN != 0 {
				go trigger()
				// try to add last mile from pub-sub (blocking)
				if d.bh != nil {
					d.bh.insertSync()
				}
			}
			// if last doSync needed only to add a few blocks less than LastMileBlocksThreshold and
			// the node is fully synced now, then switch to short range
			// the reason why we need to check distanceBeforeSync is because, if it was long distance,
			// very likely, there are a couple of new blocks have been added to the other nodes which
			// we should still stay in long range and check them.
			bnAfterSync := d.bc.CurrentBlock().NumberU64()
			distanceBeforeSync := estimatedHeight - bnBeforeSync
			distanceAfterSync := estimatedHeight - bnAfterSync
			if estimatedHeight > 0 && addedBN > 0 &&
				distanceBeforeSync <= uint64(LastMileBlocksThreshold) &&
				distanceAfterSync <= uint64(LastMileBlocksThreshold) {
				initSync = false
			}

		case <-d.closeC:
			return
		}
	}

}
