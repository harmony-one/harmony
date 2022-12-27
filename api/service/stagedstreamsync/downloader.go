package stagedstreamsync

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
	"github.com/harmony-one/harmony/shard"
)

type (
	// Downloader is responsible for sync task of one shard
	Downloader struct {
		bc                 blockChain
		syncProtocol       syncProtocol
		bh                 *beaconHelper
		stagedSyncInstance *StagedStreamSync

		downloadC chan struct{}
		closeC    chan struct{}
		ctx       context.Context
		cancel    func()

		config Config
		logger zerolog.Logger
	}
)

// NewDownloader creates a new downloader
func NewDownloader(host p2p.Host, bc core.BlockChain, config Config) *Downloader {
	config.fixValues()

	sp := sync.NewProtocol(sync.Config{
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

	host.AddStreamProtocol(sp)

	var bh *beaconHelper
	if config.BHConfig != nil && bc.ShardID() == shard.BeaconChainShardID {
		bh = newBeaconHelper(bc, config.BHConfig.BlockC, config.BHConfig.InsertHook)
	}

	logger := utils.Logger().With().Str("module", "StagedStreamSync").Uint32("ShardID", bc.ShardID()).Logger()

	ctx, cancel := context.WithCancel(context.Background())

	//TODO: use mem db should be in config file
	stagedSyncInstance, err := CreateStagedSync(ctx, bc, false, sp, config, logger, config.LogProgress)
	if err != nil {
		return nil
	}

	return &Downloader{
		bc:                 bc,
		syncProtocol:       sp,
		bh:                 bh,
		stagedSyncInstance: stagedSyncInstance,

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
		fmt.Printf("boot completed for shard %d, %d streams are connected\n", d.bc.ShardID(), d.syncProtocol.NumStreams())
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

// IsSyncing returns the current sync status
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
	initSync := d.bc.ShardID() != shard.BeaconChainShardID
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
			addedBN, err := d.stagedSyncInstance.doSync(d.ctx, initSync)
			if err != nil {
				//TODO: if there is a bad block which can't be resolved
				if d.stagedSyncInstance.invalidBlock.Active {
					numTriedStreams := len(d.stagedSyncInstance.invalidBlock.StreamID)
					// if many streams couldn't solve it, then that's an unresolvable bad block
					if numTriedStreams >= d.config.InitStreams {
						if !d.stagedSyncInstance.invalidBlock.IsLogged {
							fmt.Println("unresolvable bad block:", d.stagedSyncInstance.invalidBlock.Number)
							d.stagedSyncInstance.invalidBlock.IsLogged = true
						}
						//TODO: if we don't have any new or untried stream in the list, sleep or panic
					}
				}

				// If error happens, sleep 5 seconds and retry
				d.logger.Error().
					Err(err).
					Bool("initSync", initSync).
					Msg(WrapStagedSyncMsg("sync loop failed"))
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
				Msg(WrapStagedSyncMsg("sync finished"))

			if addedBN != 0 {
				// If block number has been changed, trigger another sync
				// and try to add last mile from pub-sub (blocking)
				go trigger()
				if d.bh != nil {
					d.bh.insertSync()
				}
			}
			d.stagedSyncInstance.initSync = false
			initSync = false

		case <-d.closeC:
			return
		}
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
