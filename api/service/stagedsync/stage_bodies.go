package stagedsync

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
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
}

func NewStageBodies(cfg StageBodiesCfg) *StageBodies {
	return &StageBodies{
		configs: cfg,
	}
}

func NewStageBodiesCfg(ctx context.Context, bc core.BlockChain, db kv.RwDB, isBeacon bool, turbo bool) StageBodiesCfg {
	cachedb, err := initBlocksCacheDB(ctx, isBeacon)
	if err != nil {
		panic("can't initialize sync caches")
	}
	return StageBodiesCfg{
		ctx:      ctx,
		bc:       bc,
		db:       db,
		turbo:    turbo,
		isBeacon: isBeacon,
		cachedb:  cachedb,
	}
}

func initBlocksCacheDB(ctx context.Context, isBeacon bool) (db kv.RwDB, err error) {
	// create caches db
	cachedbName := Block_Cache_DB
	if isBeacon {
		cachedbName = "beacon_" + cachedbName
	}
	cachedb := mdbx.NewMDBX(log.New()).Path(cachedbName).MustOpen()
	tx, errRW := cachedb.BeginRw(ctx)
	if errRW != nil {
		utils.Logger().
			Err(errRW).
			Msg("[STAGED_SYNC] initializing sync caches failed")
		return nil, errRW
	}
	defer tx.Rollback()
	if err := tx.CreateBucket(DownloadedBlocksBucket); err != nil {
		utils.Logger().
			Err(err).
			Msg("[STAGED_SYNC] creating cache bucket failed")
		return nil, err
	}
	if err := tx.CreateBucket(StageProgressBucket); err != nil {
		utils.Logger().
			Err(err).
			Msg("[STAGED_SYNC] creating progress bucket failed")
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return cachedb, nil
}

// ExecBodiesStage progresses Bodies stage in the forward direction
func (b *StageBodies) Exec(firstCycle bool, invalidBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) (err error) {

	currProgress := uint64(0)
	targetHeight := uint64(0)
	isBeacon := s.state.isBeacon
	canRunInTurboMode := b.configs.turbo
	// terminate background process in turbo mode
	// if canRunInTurboMode && b.configs.turboModeCh != nil && b.configs.bgProcRunning {
	// 	b.configs.turboModeCh <- struct{}{}
	// }

	if errV := CreateView(b.configs.ctx, b.configs.db, tx, func(etx kv.Tx) error {
		if targetHeight, err = GetStageProgress(etx, BlockHashes, isBeacon); err != nil {
			return err
		}
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
		currProgress = b.configs.bc.CurrentBlock().NumberU64()
	}

	if currProgress >= targetHeight {
		// if canRunInTurboMode && currProgress < s.state.syncStatus.currentCycle.TargetHeight {
		// 	b.configs.turboModeCh = make(chan struct{})
		// 	go b.runBackgroundProcess(nil, s, isBeacon, currProgress, currProgress+s.state.MaxBackgroundBlocks)
		// }
		return nil
	}

	// load cached blocks to main sync db
	if canRunInTurboMode {
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

	fmt.Print("\033[s") // save the cursor position
	for ok := true; ok; ok = currProgress < targetHeight {
		maxSize := targetHeight - currProgress
		size = uint64(downloadTaskBatch * len(s.state.syncConfig.peers))
		if size > maxSize {
			size = maxSize
		}
		if err = b.loadBlockHashesToTaskQueue(s, currProgress+1, size, tx); err != nil {
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

	// Run background process in turbo mode
	if canRunInTurboMode && currProgress < targetHeight {
		b.configs.turboModeCh = make(chan struct{})
		go b.runBackgroundProcess(nil, s, isBeacon, currProgress, currProgress+s.state.MaxBackgroundBlocks)
	}
	return nil
}

func (b *StageBodies) runBackgroundProcess(tx kv.RwTx, s *StageState, isBeacon bool, startHeight uint64, targetHeight uint64) error {
	//TODO: clear bg blocks db first
	currProgress := startHeight
	var err error
	size := uint64(0)
	b.configs.bgProcRunning = true

	b.configs.cachedb.View(context.Background(), func(etx kv.Tx) error {

		for ok := true; ok; ok = currProgress < targetHeight {
			select {
			case <-b.configs.turboModeCh:
				close(b.configs.turboModeCh)
				b.configs.bgProcRunning = false
				return nil
			default:
				if currProgress >= targetHeight {
					close(b.configs.turboModeCh)
					b.configs.bgProcRunning = false
					return nil
				}

				maxSize := targetHeight - currProgress
				size = uint64(downloadTaskBatch * len(s.state.syncConfig.peers))
				if size > maxSize {
					size = maxSize
				}

				if err = b.loadExtraBlockHashesToTaskQueue(s, currProgress+1, size, nil); err != nil {
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
			// time.Sleep(1 * time.Millisecond)
		}
		return nil
	})

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
	// Initialize blockchain
	var wg sync.WaitGroup
	count := 0
	taskQueue := downloadTaskQueue{ss.stateSyncTaskQueue}
	s.state.downloadedBlocks = make(map[string][]byte)

	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
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
					peerConfig.failedTimes++
					utils.Logger().Warn().Err(err).
						Str("peerID", peerConfig.ip).
						Str("port", peerConfig.port).
						Msg("[STAGED_SYNC] downloadBlocks: GetBlocks failed")
					if err := taskQueue.put(tasks); err != nil {
						utils.Logger().Warn().
							Err(err).
							Interface("taskIndexes", tasks.indexes()).
							Msg("cannot add task back to queue")
					}
					if peerConfig.failedTimes > downloadBlocksRetryLimit {
						ss.syncConfig.RemovePeer(peerConfig)
					}
					return
				}
				if len(payload) == 0 {
					peerConfig.failedTimes++
					utils.Logger().Error().Int("failNumber", count).
						Msg("[STAGED_SYNC] downloadBlocks: no more retrievable blocks")
					if err := taskQueue.put(tasks); err != nil {
						utils.Logger().Warn().
							Err(err).
							Interface("taskIndexes", tasks.indexes()).
							Interface("taskBlockes", tasks.blockHashesStr()).
							Msg("downloadBlocks: cannot add task")
					}
					ss.syncConfig.RemovePeer(peerConfig)
					if peerConfig.failedTimes > downloadBlocksRetryLimit {
						ss.syncConfig.RemovePeer(peerConfig)
					}
					return
				}
				// node received blocks from peer, so it is working now  
				peerConfig.failedTimes = 0

				failedTasks, err := b.handleBlockSyncResult(s, payload, tasks, verifyAllSig, tx)
				if err != nil {
					if err := taskQueue.put(tasks); err != nil {
						utils.Logger().Warn().
							Err(err).
							Interface("taskIndexes", tasks.indexes()).
							Interface("taskBlockes", tasks.blockHashesStr()).
							Msg("downloadBlocks: cannot add task")
					}
					peerConfig.failedTimes++
					if peerConfig.failedTimes > downloadBlocksRetryLimit {
						ss.syncConfig.RemovePeer(peerConfig)
					}
					return
				}

				if len(failedTasks) != 0 {
					peerConfig.failedTimes++
					if peerConfig.failedTimes > downloadBlocksRetryLimit {
						ss.syncConfig.RemovePeer(peerConfig)
						return
					}
					if err := taskQueue.put(failedTasks); err != nil {
						utils.Logger().Warn().
							Err(err).
							Interface("task Indexes", failedTasks.indexes()).
							Interface("task Blocks", tasks.blockHashesStr()).
							Msg("cannot add task")
					}
				}
			}
		}()
		return
	})
	wg.Wait()
	utils.Logger().Info().Msg("[STAGED_SYNC] downloadBlocks: finished")
	return nil
}

func (b *StageBodies) handleBlockSyncResult(s *StageState, payload [][]byte, tasks syncBlockTasks, verifyAllSig bool, tx kv.RwTx) (syncBlockTasks, error) {
	if len(payload) > len(tasks) {
		utils.Logger().Warn().
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

	s.state.syncMux.Lock()
	defer s.state.syncMux.Unlock()

	for i, blockBytes := range payload {
		k := fmt.Sprintf("%020d", tasks[i].index) //block.NumberU64())
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
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving progress for block bodies stage failed: %v", err)
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
		return nil

	}); errV != nil {
		return errV
	}

	if s.state.stateSyncTaskQueue.Len() != int64(size) {
		return fmt.Errorf("cannot add task to queue")
	}
	return nil
}

func (b *StageBodies) loadExtraBlockHashesToTaskQueue(s *StageState, startIndex uint64, size uint64, tx kv.RwTx) error {
	s.state.stateSyncTaskQueue = queue.New(0)
	if errV := CreateView(b.configs.ctx, b.configs.db, tx, func(etx kv.Tx) error {

		for i := startIndex; i < startIndex+size; i++ {
			key := strconv.FormatUint(i, 10)
			id := int(i - startIndex)
			bucketName := GetBucketName(ExtraBlockHashesBucket, s.state.isBeacon)
			blockHash, err := etx.GetOne(bucketName, []byte(key))
			if err != nil {
				return err
			}
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
		return nil

	}); errV != nil {
		return errV
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

	// sort blocks by task id
	taskIDs := make([]string, 0, len(s.state.downloadedBlocks))
	for id := range s.state.downloadedBlocks {
		taskIDs = append(taskIDs, id)
	}
	sort.Strings(taskIDs)
	for _, key := range taskIDs {
		blockBytes := s.state.downloadedBlocks[key]
		blkNumber := fmt.Sprintf("%020d", p+1)
		bucketName := GetBucketName(DownloadedBlocksBucket, s.state.isBeacon)
		if err := tx.Put(bucketName, []byte(blkNumber), blockBytes); err != nil {
			utils.Logger().Warn().
				Err(err).
				Uint64("block height", p).
				Msg("[STAGED_SYNC] adding block to db failed")
			return p, err
		}
		p++
	}
	// check if all block hashes are added to db break the loop
	if p-progress != uint64(len(s.state.downloadedBlocks)) {
		return progress, fmt.Errorf("save downloaded block bodies failed")
	}
	// save progress
	if err = s.Update(tx, p); err != nil {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving progress for block bodies stage failed: %v", err)
		return progress, ErrSavingBodiesProgressFail
	}
	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return progress, err
		}
	}
	return p, nil
}

func (b *StageBodies) cacheBlocks(s *StageState, progress uint64) (p uint64, err error) {
	p = progress

	tx, err := b.configs.cachedb.BeginRw(context.Background())
	if err != nil {
		return p, err
	}
	defer tx.Rollback()

	// sort blocks by task id
	taskIDs := make([]string, 0, len(s.state.downloadedBlocks))
	for id := range s.state.downloadedBlocks {
		taskIDs = append(taskIDs, id)
	}
	sort.Strings(taskIDs)
	for _, key := range taskIDs {
		blockBytes := s.state.downloadedBlocks[key]
		blkNumber := fmt.Sprintf("%020d", p+1)
		if err := tx.Put(DownloadedBlocksBucket, []byte(blkNumber), blockBytes); err != nil {
			utils.Logger().Warn().
				Err(err).
				Uint64("block height", p).
				Msg("[STAGED_SYNC] caching block failed")
			return p, err
		}
		p++
	}
	// check if all block hashes are added to db break the loop
	if p-progress != uint64(len(s.state.downloadedBlocks)) {
		return p, fmt.Errorf("caching downloaded block bodies failed")
	}

	// save progress
	if err = tx.Put(StageProgressBucket, []byte(LastBlockHeight), marshalData(p)); err != nil {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving cache progress for blocks stage failed: %v", err)
		return p, ErrSavingCachedBodiesProgressFail
	}

	if err := tx.Commit(); err != nil {
		return p, err
	}

	return p, nil
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

	errV := b.configs.cachedb.View(context.Background(), func(rtx kv.Tx) error {
		lastCachedHeightBytes, err := rtx.GetOne(StageProgressBucket, []byte(LastBlockHeight))
		if err != nil {
			utils.Logger().Info().
				Msgf("[STAGED_SYNC] retrieving cache progress for blocks stage failed: %v", err)
			return ErrRetrievingCachedBodiesProgressFail
		}
		lastHeight, err := unmarshalData(lastCachedHeightBytes)
		if err != nil {
			utils.Logger().Info().
				Msgf("[STAGED_SYNC] retrieving cache progress for blocks stage failed: %v", err)
			return ErrRetrievingCachedBodiesProgressFail
		}

		if p >= lastHeight {
			return nil
		}

		// load block hashes from cache db snd copy them to main sync db
		for ok := true; ok; ok = p < lastHeight {
			key := fmt.Sprintf("%020d", p+1)
			blkBytes, err := rtx.GetOne(DownloadedBlocksBucket, []byte(key))
			if err != nil {
				utils.Logger().Warn().
					Err(err).
					Str("block height", key).
					Msg("[STAGED_SYNC] retrieve block from cache failed")
				return err
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
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving retrieved cached progress for blocks stage failed: %v", err)
		return p, ErrSavingCachedBodiesProgressFail
	}

	// update the progress
	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return p, err
		}
	}

	return p, nil
}

func (b *StageBodies) Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = b.configs.db.BeginRw(b.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// MakeBodiesNonCanonical

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

func (b *StageBodies) Prune(firstCycle bool, p *PruneState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = b.configs.db.BeginRw(b.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// terminate background process in turbo mode
	if b.configs.turboModeCh != nil && b.configs.bgProcRunning {
		b.configs.turboModeCh <- struct{}{}
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
