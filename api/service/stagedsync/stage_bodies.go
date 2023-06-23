package stagedsync

import (
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/log/v3"
)

type StageBodies struct {
	configs StageBodiesCfg
}
type StageBodiesCfg struct {
	ctx           context.Context
	bc            core.BlockChain
	db            kv.RwDB
	turbo         bool
	turboModeCh   chan struct{}
	bgProcRunning bool
	isBeacon      bool
	cachedb       kv.RwDB
	logProgress   bool
}

func NewStageBodies(cfg StageBodiesCfg) *StageBodies {
	return &StageBodies{
		configs: cfg,
	}
}

func NewStageBodiesCfg(ctx context.Context, bc core.BlockChain, dbDir string, db kv.RwDB, isBeacon bool, turbo bool, logProgress bool) StageBodiesCfg {
	cachedb, err := initBlocksCacheDB(ctx, dbDir, isBeacon)
	if err != nil {
		panic("can't initialize sync caches")
	}
	return StageBodiesCfg{
		ctx:         ctx,
		bc:          bc,
		db:          db,
		turbo:       turbo,
		isBeacon:    isBeacon,
		cachedb:     cachedb,
		logProgress: logProgress,
	}
}

func initBlocksCacheDB(ctx context.Context, dbDir string, isBeacon bool) (db kv.RwDB, err error) {
	// create caches db
	cachedbName := BlockCacheDB
	if isBeacon {
		cachedbName = "beacon_" + cachedbName
	}
	dbPath := filepath.Join(dbDir, cachedbName)
	cachedb := mdbx.NewMDBX(log.New()).Path(dbPath).MustOpen()
	tx, errRW := cachedb.BeginRw(ctx)
	if errRW != nil {
		utils.Logger().Error().
			Err(errRW).
			Msg("[STAGED_SYNC] initializing sync caches failed")
		return nil, errRW
	}
	defer tx.Rollback()
	if err := tx.CreateBucket(DownloadedBlocksBucket); err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("[STAGED_SYNC] creating cache bucket failed")
		return nil, err
	}
	if err := tx.CreateBucket(StageProgressBucket); err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("[STAGED_SYNC] creating progress bucket failed")
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return cachedb, nil
}

// Exec progresses Bodies stage in the forward direction
func (b *StageBodies) Exec(firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) (err error) {

	maxPeersHeight := s.state.syncStatus.MaxPeersHeight
	currentHead := b.configs.bc.CurrentBlock().NumberU64()
	if currentHead >= maxPeersHeight {
		return nil
	}
	currProgress := uint64(0)
	targetHeight := s.state.syncStatus.currentCycle.TargetHeight
	isBeacon := s.state.isBeacon
	isLastCycle := targetHeight >= maxPeersHeight
	canRunInTurboMode := b.configs.turbo && !isLastCycle

	if errV := CreateView(b.configs.ctx, b.configs.db, tx, func(etx kv.Tx) error {
		if currProgress, err = s.CurrentStageProgress(etx); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	if currProgress == 0 {
		if err := b.clearBlocksBucket(tx, s.state.isBeacon); err != nil {
			return err
		}
		currProgress = currentHead
	}

	if currProgress >= targetHeight {
		return nil
	}

	// load cached blocks to main sync db
	if b.configs.turbo && !firstCycle {
		if currProgress, err = b.loadBlocksFromCache(s, currProgress, tx); err != nil {
			return err
		}
	}

	if currProgress >= targetHeight {
		return nil
	}

	size := uint64(0)
	startTime := time.Now()
	startBlock := currProgress
	if b.configs.logProgress {
		fmt.Print("\033[s") // save the cursor position
	}

	for ok := true; ok; ok = currProgress < targetHeight {
		maxSize := targetHeight - currProgress
		size = uint64(downloadTaskBatch * len(s.state.syncConfig.peers))
		if size > maxSize {
			size = maxSize
		}
		if err = b.loadBlockHashesToTaskQueue(s, currProgress+1, size, tx); err != nil {
			s.state.RevertTo(b.configs.bc.CurrentBlock().NumberU64(), b.configs.bc.CurrentBlock().Hash())
			return err
		}

		// Download blocks.
		verifyAllSig := true //TODO: move it to configs
		if err = b.downloadBlocks(s, verifyAllSig, tx); err != nil {
			return nil
		}
		// save blocks and update current progress
		if currProgress, err = b.saveDownloadedBlocks(s, currProgress, tx); err != nil {
			return err
		}
		// log the stage progress in console
		if b.configs.logProgress {
			//calculating block speed
			dt := time.Now().Sub(startTime).Seconds()
			speed := float64(0)
			if dt > 0 {
				speed = float64(currProgress-startBlock) / dt
			}
			blockSpeed := fmt.Sprintf("%.2f", speed)
			fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
			fmt.Println("downloading blocks progress:", currProgress, "/", targetHeight, "(", blockSpeed, "blocks/s", ")")
		}
	}

	// Run background process in turbo mode
	if canRunInTurboMode && currProgress < maxPeersHeight {
		b.configs.turboModeCh = make(chan struct{})
		go b.runBackgroundProcess(tx, s, isBeacon, currProgress, currProgress+s.state.MaxBackgroundBlocks)
	}
	return nil
}

// runBackgroundProcess continues downloading blocks in the background and caching them on disk while next stages are running.
// In the next sync cycle, this stage will use cached blocks rather than download them from peers.
// This helps performance and reduces stage duration. It also helps to use the resources more efficiently.
func (b *StageBodies) runBackgroundProcess(tx kv.RwTx, s *StageState, isBeacon bool, startHeight uint64, targetHeight uint64) error {

	s.state.syncStatus.currentCycle.lock.RLock()
	defer s.state.syncStatus.currentCycle.lock.RUnlock()

	if s.state.syncStatus.currentCycle.Number == 0 || len(s.state.syncStatus.currentCycle.ExtraHashes) == 0 {
		return nil
	}
	currProgress := startHeight
	var err error
	size := uint64(0)
	b.configs.bgProcRunning = true

	defer func() {
		if b.configs.bgProcRunning {
			close(b.configs.turboModeCh)
			b.configs.bgProcRunning = false
		}
	}()

	for ok := true; ok; ok = currProgress < targetHeight {
		select {
		case <-b.configs.turboModeCh:
			return nil
		default:
			if currProgress >= targetHeight {
				return nil
			}

			maxSize := targetHeight - currProgress
			size = uint64(downloadTaskBatch * len(s.state.syncConfig.peers))
			if size > maxSize {
				size = maxSize
			}
			if err = b.loadExtraBlockHashesToTaskQueue(s, currProgress+1, size); err != nil {
				return err
			}
			// Download blocks.
			verifyAllSig := true //TODO: move it to configs
			if err = b.downloadBlocks(s, verifyAllSig, nil); err != nil {
				return nil
			}
			// save blocks and update current progress
			if currProgress, err = b.cacheBlocks(s, currProgress); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *StageBodies) clearBlocksBucket(tx kv.RwTx, isBeacon bool) error {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = b.configs.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}
	bucketName := GetBucketName(DownloadedBlocksBucket, isBeacon)
	if err := tx.ClearBucket(bucketName); err != nil {
		return err
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// downloadBlocks downloads blocks from state sync task queue.
func (b *StageBodies) downloadBlocks(s *StageState, verifyAllSig bool, tx kv.RwTx) (err error) {
	ss := s.state
	var wg sync.WaitGroup
	taskQueue := downloadTaskQueue{ss.stateSyncTaskQueue}
	s.state.InitDownloadedBlocksMap()

	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !peerConfig.client.IsReady() {
				// try to connect
				if ready := peerConfig.client.WaitForConnection(1000 * time.Millisecond); !ready {
					if !peerConfig.client.IsConnecting() { // if it's idle or closed then remove it
						ss.syncConfig.RemovePeer(peerConfig, "not ready to download blocks")
					}
					return
				}
			}
			for !taskQueue.empty() {
				tasks, err := taskQueue.poll(downloadTaskBatch, time.Millisecond)
				if err != nil || len(tasks) == 0 {
					if err == queue.ErrDisposed {
						continue
					}
					utils.Logger().Error().
						Err(err).
						Msg("[STAGED_SYNC] downloadBlocks: ss.stateSyncTaskQueue poll timeout")
					break
				}
				payload, err := peerConfig.GetBlocks(tasks.blockHashes())
				if err != nil {
					isBrokenPeer := peerConfig.AddFailedTime(downloadBlocksRetryLimit)
					utils.Logger().Error().
						Err(err).
						Str("peerID", peerConfig.ip).
						Str("port", peerConfig.port).
						Msg("[STAGED_SYNC] downloadBlocks: GetBlocks failed")
					if err := taskQueue.put(tasks); err != nil {
						utils.Logger().Error().
							Err(err).
							Interface("taskIndexes", tasks.indexes()).
							Msg("cannot add task back to queue")
					}
					if isBrokenPeer {
						ss.syncConfig.RemovePeer(peerConfig, "get blocks failed")
					}
					return
				}
				if len(payload) == 0 {
					isBrokenPeer := peerConfig.AddFailedTime(downloadBlocksRetryLimit)
					utils.Logger().Error().
						Str("peerID", peerConfig.ip).
						Str("port", peerConfig.port).
						Msg("[STAGED_SYNC] downloadBlocks: no more retrievable blocks")
					if err := taskQueue.put(tasks); err != nil {
						utils.Logger().Error().
							Err(err).
							Interface("taskIndexes", tasks.indexes()).
							Interface("taskBlockes", tasks.blockHashesStr()).
							Msg("downloadBlocks: cannot add task")
					}
					if isBrokenPeer {
						ss.syncConfig.RemovePeer(peerConfig, "no blocks in payload")
					}
					return
				}
				// node received blocks from peer, so it is working now
				peerConfig.failedTimes = 0

				failedTasks, err := b.handleBlockSyncResult(s, payload, tasks, verifyAllSig, tx)
				if err != nil {
					isBrokenPeer := peerConfig.AddFailedTime(downloadBlocksRetryLimit)
					utils.Logger().Error().
						Err(err).
						Str("peerID", peerConfig.ip).
						Str("port", peerConfig.port).
						Msg("[STAGED_SYNC] downloadBlocks: handleBlockSyncResult failed")
					if err := taskQueue.put(tasks); err != nil {
						utils.Logger().Error().
							Err(err).
							Interface("taskIndexes", tasks.indexes()).
							Interface("taskBlockes", tasks.blockHashesStr()).
							Msg("downloadBlocks: cannot add task")
					}
					if isBrokenPeer {
						ss.syncConfig.RemovePeer(peerConfig, "handleBlockSyncResult failed")
					}
					return
				}

				if len(failedTasks) != 0 {
					isBrokenPeer := peerConfig.AddFailedTime(downloadBlocksRetryLimit)
					utils.Logger().Error().
						Str("peerID", peerConfig.ip).
						Str("port", peerConfig.port).
						Msg("[STAGED_SYNC] downloadBlocks: some tasks failed")
					if err := taskQueue.put(failedTasks); err != nil {
						utils.Logger().Error().
							Err(err).
							Interface("task Indexes", failedTasks.indexes()).
							Interface("task Blocks", tasks.blockHashesStr()).
							Msg("cannot add task")
					}
					if isBrokenPeer {
						ss.syncConfig.RemovePeer(peerConfig, "some blocks failed to handle")
					}
					return
				}
			}
		}()
		return
	})
	wg.Wait()
	return nil
}

func (b *StageBodies) handleBlockSyncResult(s *StageState, payload [][]byte, tasks syncBlockTasks, verifyAllSig bool, tx kv.RwTx) (syncBlockTasks, error) {
	if len(payload) > len(tasks) {
		utils.Logger().Error().
			Err(ErrUnexpectedNumberOfBlocks).
			Int("expect", len(tasks)).
			Int("got", len(payload))
		return tasks, ErrUnexpectedNumberOfBlocks
	}

	var failedTasks syncBlockTasks
	if len(payload) < len(tasks) {
		utils.Logger().Warn().
			Err(ErrUnexpectedNumberOfBlocks).
			Int("expect", len(tasks)).
			Int("got", len(payload))
		failedTasks = append(failedTasks, tasks[len(payload):]...)
	}

	s.state.lockBlocks.Lock()
	defer s.state.lockBlocks.Unlock()

	for i, blockBytes := range payload {
		if len(blockBytes[:]) <= 1 {
			failedTasks = append(failedTasks, tasks[i])
			continue
		}
		k := uint64(tasks[i].index) // fmt.Sprintf("%d", tasks[i].index) //fmt.Sprintf("%020d", tasks[i].index)
		s.state.downloadedBlocks[k] = make([]byte, len(blockBytes))
		copy(s.state.downloadedBlocks[k], blockBytes[:])
	}

	return failedTasks, nil
}

func (b *StageBodies) saveProgress(s *StageState, progress uint64, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = b.configs.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// save progress
	if err = s.Update(tx, progress); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] saving progress for block bodies stage failed")
		return ErrSavingBodiesProgressFail
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (b *StageBodies) loadBlockHashesToTaskQueue(s *StageState, startIndex uint64, size uint64, tx kv.RwTx) error {
	s.state.stateSyncTaskQueue = queue.New(0)
	if errV := CreateView(b.configs.ctx, b.configs.db, tx, func(etx kv.Tx) error {

		for i := startIndex; i < startIndex+size; i++ {
			key := strconv.FormatUint(i, 10)
			id := int(i - startIndex)
			bucketName := GetBucketName(BlockHashesBucket, s.state.isBeacon)
			blockHash, err := etx.GetOne(bucketName, []byte(key))
			if err != nil {
				return err
			}
			if blockHash == nil || len(blockHash) == 0 {
				break
			}
			if err := s.state.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash}); err != nil {
				s.state.stateSyncTaskQueue = queue.New(0)
				utils.Logger().Error().
					Err(err).
					Int("taskIndex", id).
					Str("taskBlock", hex.EncodeToString(blockHash)).
					Msg("[STAGED_SYNC] loadBlockHashesToTaskQueue: cannot add task")
				break
			}
		}
		return nil

	}); errV != nil {
		return errV
	}

	if s.state.stateSyncTaskQueue.Len() != int64(size) {
		return ErrAddTaskFailed
	}
	return nil
}

func (b *StageBodies) loadExtraBlockHashesToTaskQueue(s *StageState, startIndex uint64, size uint64) error {

	s.state.stateSyncTaskQueue = queue.New(0)

	for i := startIndex; i < startIndex+size; i++ {
		id := int(i - startIndex)
		blockHash := s.state.syncStatus.currentCycle.ExtraHashes[i]
		if len(blockHash[:]) == 0 {
			break
		}
		if err := s.state.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash}); err != nil {
			s.state.stateSyncTaskQueue = queue.New(0)
			utils.Logger().Warn().
				Err(err).
				Int("taskIndex", id).
				Str("taskBlock", hex.EncodeToString(blockHash)).
				Msg("[STAGED_SYNC] loadBlockHashesToTaskQueue: cannot add task")
			break
		}
	}

	if s.state.stateSyncTaskQueue.Len() != int64(size) {
		return ErrAddTasksToQueueFail
	}
	return nil
}

func (b *StageBodies) saveDownloadedBlocks(s *StageState, progress uint64, tx kv.RwTx) (p uint64, err error) {
	p = progress

	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = b.configs.db.BeginRw(context.Background())
		if err != nil {
			return p, err
		}
		defer tx.Rollback()
	}

	downloadedBlocks := s.state.GetDownloadedBlocks()

	for i := uint64(0); i < uint64(len(downloadedBlocks)); i++ {
		blockBytes := downloadedBlocks[i]
		n := progress + i + 1
		blkNumber := marshalData(n)
		bucketName := GetBucketName(DownloadedBlocksBucket, s.state.isBeacon)
		if err := tx.Put(bucketName, blkNumber, blockBytes); err != nil {
			utils.Logger().Error().
				Err(err).
				Uint64("block height", n).
				Msg("[STAGED_SYNC] adding block to db failed")
			return p, err
		}
		p++
	}
	// check if all block hashes are added to db break the loop
	if p-progress != uint64(len(downloadedBlocks)) {
		return progress, ErrSaveBlocksFail
	}
	// save progress
	if err = s.Update(tx, p); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] saving progress for block bodies stage failed")
		return progress, ErrSavingBodiesProgressFail
	}
	// if it's using its own transaction, commit transaction to db to cache all downloaded blocks
	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return progress, err
		}
	}
	// it cached blocks successfully, so, it returns the cache progress
	return p, nil
}

func (b *StageBodies) cacheBlocks(s *StageState, progress uint64) (p uint64, err error) {
	p = progress

	tx, err := b.configs.cachedb.BeginRw(context.Background())
	if err != nil {
		return p, err
	}
	defer tx.Rollback()

	downloadedBlocks := s.state.GetDownloadedBlocks()

	for i := uint64(0); i < uint64(len(downloadedBlocks)); i++ {
		blockBytes := downloadedBlocks[i]
		n := progress + i + 1
		blkNumber := marshalData(n) // fmt.Sprintf("%020d", p+1)
		if err := tx.Put(DownloadedBlocksBucket, blkNumber, blockBytes); err != nil {
			utils.Logger().Error().
				Err(err).
				Uint64("block height", p).
				Msg("[STAGED_SYNC] caching block failed")
			return p, err
		}
		p++
	}
	// check if all block hashes are added to db break the loop
	if p-progress != uint64(len(downloadedBlocks)) {
		return p, ErrCachingBlocksFail
	}

	// save progress
	if err = tx.Put(StageProgressBucket, []byte(LastBlockHeight), marshalData(p)); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] saving cache progress for blocks stage failed")
		return p, ErrSavingCachedBodiesProgressFail
	}

	if err := tx.Commit(); err != nil {
		return p, err
	}

	return p, nil
}

// clearCache removes block hashes from cache db
func (b *StageBodies) clearCache() error {
	tx, err := b.configs.cachedb.BeginRw(context.Background())
	if err != nil {
		return nil
	}
	defer tx.Rollback()

	if err := tx.ClearBucket(DownloadedBlocksBucket); err != nil {
		return nil
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// load blocks from cache db to main sync db and update the progress
func (b *StageBodies) loadBlocksFromCache(s *StageState, startHeight uint64, tx kv.RwTx) (p uint64, err error) {

	p = startHeight

	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = b.configs.db.BeginRw(b.configs.ctx)
		if err != nil {
			return p, err
		}
		defer tx.Rollback()
	}

	defer func() {
		// Clear cache db
		b.configs.cachedb.Update(context.Background(), func(etx kv.RwTx) error {
			if err := etx.ClearBucket(DownloadedBlocksBucket); err != nil {
				return err
			}
			return nil
		})
	}()

	errV := b.configs.cachedb.View(context.Background(), func(rtx kv.Tx) error {
		lastCachedHeightBytes, err := rtx.GetOne(StageProgressBucket, []byte(LastBlockHeight))
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Msgf("[STAGED_SYNC] retrieving cache progress for blocks stage failed")
			return ErrRetrievingCachedBodiesProgressFail
		}
		lastHeight, err := unmarshalData(lastCachedHeightBytes)
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Msgf("[STAGED_SYNC] retrieving cache progress for blocks stage failed")
			return ErrRetrievingCachedBodiesProgressFail
		}

		if startHeight >= lastHeight {
			return nil
		}

		// load block hashes from cache db snd copy them to main sync db
		for ok := true; ok; ok = p < lastHeight {
			key := marshalData(p + 1)
			blkBytes, err := rtx.GetOne(DownloadedBlocksBucket, []byte(key))
			if err != nil {
				utils.Logger().Error().
					Err(err).
					Uint64("block height", p+1).
					Msg("[STAGED_SYNC] retrieve block from cache failed")
				return err
			}
			if len(blkBytes[:]) <= 1 {
				break
			}
			bucketName := GetBucketName(DownloadedBlocksBucket, s.state.isBeacon)
			if err = tx.Put(bucketName, []byte(key), blkBytes); err != nil {
				return err
			}
			p++
		}
		return nil
	})
	if errV != nil {
		return startHeight, errV
	}

	// save progress
	if err = s.Update(tx, p); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] saving retrieved cached progress for blocks stage failed")
		return startHeight, ErrSavingCachedBodiesProgressFail
	}

	// update the progress
	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return startHeight, err
		}
	}

	return p, nil
}

func (b *StageBodies) Revert(firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = b.configs.db.BeginRw(b.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// terminate background process in turbo mode
	if b.configs.bgProcRunning {
		b.configs.bgProcRunning = false
		b.configs.turboModeCh <- struct{}{}
		close(b.configs.turboModeCh)
	}

	// clean block hashes db
	blocksBucketName := GetBucketName(DownloadedBlocksBucket, b.configs.isBeacon)
	if err = tx.ClearBucket(blocksBucketName); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] clear blocks bucket after revert failed")
		return err
	}

	// clean cache db as well
	if err := b.clearCache(); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] clear blocks cache failed")
		return err
	}

	// save progress
	currentHead := b.configs.bc.CurrentBlock().NumberU64()
	if err = s.Update(tx, currentHead); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] saving progress for block bodies stage after revert failed")
		return err
	}

	if err = u.Done(tx); err != nil {
		return err
	}

	if useInternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (b *StageBodies) CleanUp(firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = b.configs.db.BeginRw(b.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// terminate background process in turbo mode
	if b.configs.bgProcRunning {
		b.configs.bgProcRunning = false
		b.configs.turboModeCh <- struct{}{}
		close(b.configs.turboModeCh)
	}
	blocksBucketName := GetBucketName(DownloadedBlocksBucket, b.configs.isBeacon)
	tx.ClearBucket(blocksBucketName)

	if useInternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
