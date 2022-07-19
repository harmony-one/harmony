package stagedsync

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type StageBodies struct {
	configs StageBodiesCfg
}
type StageBodiesCfg struct {
	ctx context.Context
	bc  *core.BlockChain
	db  kv.RwDB
}

func NewStageBodies(cfg StageBodiesCfg) *StageBodies {
	return &StageBodies{
		configs: cfg,
	}
}

func NewStageBodiesCfg(ctx context.Context, bc *core.BlockChain, db kv.RwDB) StageBodiesCfg {
	return StageBodiesCfg{
		ctx: ctx,
		bc:  bc,
		db:  db,
	}
}

// ExecBodiesStage progresses Bodies stage in the forward direction
func (b *StageBodies) Exec(firstCycle bool, badBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) (err error) {

	currProgress := uint64(0)
	targetHeight := uint64(0)
	isBeacon := s.state.isBeacon

	if errV := b.configs.db.View(b.configs.ctx, func(etx kv.Tx) error {
		if targetHeight, err = GetStageProgress(etx, BlockHashes, isBeacon); err != nil {
			return err
		}
		if currProgress, err = s.CurrentStageProgress(etx); err != nil { //GetStageProgress(etx, Bodies, isBeacon); err != nil {
			return err
		}
		if currProgress == 0 {
			if err := b.clearBlocksBucket(nil, s.state.isBeacon); err != nil {
				return err
			}
			currProgress = b.configs.bc.CurrentBlock().NumberU64()
		}
		return nil
	}); errV != nil {
		return errV
	}

	if currProgress >= targetHeight {
		return nil
	}

	size := uint64(0)
	// currProgress++
	startTime := time.Now()
	startBlock := currProgress

	fmt.Print("\033[s") // save the cursor position
	for ok := true; ok; ok = currProgress < targetHeight {
		maxSize := targetHeight - currProgress
		size = uint64(downloadTaskBatch * len(s.state.syncConfig.peers))
		if size > maxSize {
			size = maxSize
		}
		if err = b.loadBlockHashesToTaskQueue(s, currProgress+1, size, nil); err != nil {
			return err
		}

		if s.state.stateSyncTaskQueue.Len() != int64(size) {
			return fmt.Errorf("loading block hashes in task queue failed, len doesn't match")
		}

		// Download blocks.
		verifyAllSig := true //TODO: move it to configs
		if err = b.downloadBlocks(s, verifyAllSig, nil); err != nil {
			return nil
		}
		// save blocks and update current progress
		if currProgress, err = b.saveDownloadedBlocks(s, currProgress, nil); err != nil {
			return err
		}

		// currProgress += size
		// if err = b.saveProgress(s, currProgress, tx); err != nil {
		// 	return err
		// }

		//calculating block speed
		dt := time.Now().Sub(startTime).Seconds()
		speed := float64(0)
		if (dt>0) {
			speed = float64(currProgress - startBlock) / dt
		}
		blockSpeed := fmt.Sprintf("%.2f", speed)

		fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
		fmt.Println("downloading blocks progress:", currProgress, "/", targetHeight, "(", blockSpeed,"blocks/s",")")
	}

	return nil
}

func (b *StageBodies) clearBlocksBucket(tx kv.RwTx, isBeacon bool) error {
	useExternalTx := tx != nil
	if !useExternalTx {
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

	if !useExternalTx {
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
					utils.Logger().Error().Err(err).Msg("[STAGED_SYNC] downloadBlocks: ss.stateSyncTaskQueue poll timeout")
					break
				}
				payload, err := peerConfig.GetBlocks(tasks.blockHashes())
				if err != nil {
					count++
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
					ss.syncConfig.RemovePeer(peerConfig)
					if count > downloadBlocksRetryLimit {
						break
					}
					return
				}
				if len(payload) == 0 {
					count++
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
					if count > downloadBlocksRetryLimit {
						break
					}
					continue
				}

				failedTasks, err := b.handleBlockSyncResult(s, payload, tasks, verifyAllSig, nil)
				if err != nil {
					if err := taskQueue.put(tasks); err != nil {
						utils.Logger().Warn().
							Err(err).
							Interface("taskIndexes", tasks.indexes()).
							Interface("taskBlockes", tasks.blockHashesStr()).
							Msg("downloadBlocks: cannot add task")
					}
					ss.syncConfig.RemovePeer(peerConfig)
					return
				}

				if len(failedTasks) != 0 {
					count++
					if count > downloadBlocksRetryLimit {
						break
					}
					if err := taskQueue.put(failedTasks); err != nil {
						utils.Logger().Warn().
							Err(err).
							Interface("taskIndexes", failedTasks.indexes()).
							Interface("taskBlockes", tasks.blockHashesStr()).
							Msg("cannot add task")
					}
					continue
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
			Err(errors.New("unexpected number of block delivered")).
			Int("expect", len(tasks)).
			Int("got", len(payload))
		return tasks, errors.Errorf("unexpected number of block delivered")
	}

	var failedTasks syncBlockTasks
	if len(payload) < len(tasks) {
		utils.Logger().Warn().
			Err(errors.New("unexpected number of block delivered")).
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
	useExternalTx := tx != nil
	if !useExternalTx {
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
		return fmt.Errorf("saving progress for block bodies stage failed")
	}

	if !useExternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (b *StageBodies) loadBlockHashesToTaskQueue(s *StageState, startIndex uint64, size uint64, tx kv.RwTx) error {
	s.state.stateSyncTaskQueue = queue.New(0)
	if errV := b.configs.db.View(b.configs.ctx, func(etx kv.Tx) error {

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

func (b *StageBodies) saveDownloadedBlocks(s *StageState, progress uint64, tx kv.RwTx) (p uint64, err error) {
	p = progress

	useExternalTx := tx != nil
	if !useExternalTx {
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
		return p, fmt.Errorf("save downloaded block bodies failed")
	}
	// save progress
	if err = s.Update(tx, p); err != nil {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving progress for block bodies stage failed: %v", err)
		return p, fmt.Errorf("saving progress for block bodies stage failed")
	}
	if !useExternalTx {
		if err := tx.Commit(); err != nil {
			return p, err
		}
	}
	return p, nil
}

func (b *StageBodies) Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
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
	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (b *StageBodies) Prune(firstCycle bool, p *PruneState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = b.configs.db.BeginRw(b.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
