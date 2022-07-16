package stagedsync

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/pkg/errors"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
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
		if currProgress, err = GetStageProgress(etx, Bodies, isBeacon); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	if currProgress >= targetHeight {
		return nil
	}

	size := uint64(0)

	//fmt.Print("\033[s") // save the cursor position

	for ok := true; ok; ok = currProgress < targetHeight {
		// size = targetHeight - currProgress
		// if size > uint64(SyncLoopBatchSize) {
		// 	size = uint64(SyncLoopBatchSize)
		// }
		maxSize := targetHeight - currProgress
		size = uint64(downloadTaskBatch * len(s.state.syncConfig.peers))
		if size > maxSize {
			size = maxSize
		}
		if err = b.loadBlockHashesToTaskQueue(s, currProgress, size, nil); err != nil {
			return err
		}

		if s.state.stateSyncTaskQueue.Len() != int64(size) {
			return fmt.Errorf("loading block hashes in task queue failed, len doesn't match")
		}

		// Download blocks.
		//s.state.downloadBlocks(s.state.Blockchain())
		verifyAllSig := true //TODO: move it to configs
		if err = b.downloadBlocks(s, verifyAllSig, nil); err != nil {
			return nil
		}

		if currProgress, err = b.saveDownloadedBlocks(s, currProgress, nil); err != nil {
			return err
		}

		// currProgress += size

		// if err = b.saveProgress(s, currProgress, tx); err != nil {
		// 	return err
		// }

		// fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
		// fmt.Println("downloading blocks progress:", currProgress, "/", targetHeight)
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
	s.state.downloadedBlocks = make(map[int][]byte)

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
				fmt.Println("tasks: ---------> ", tasks.indexes())
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
		block, err := RlpDecodeBlockOrBlockWithSig(blockBytes)
		if err != nil {
			utils.Logger().Warn().
				Err(err).
				Int("taskIndex", tasks[i].index).
				Str("taskBlock", hex.EncodeToString(tasks[i].blockHash)).
				Msg("download block")
			failedTasks = append(failedTasks, tasks[i])
			continue
		}
		gotHash := block.Hash()
		if !bytes.Equal(gotHash[:], tasks[i].blockHash) {
			utils.Logger().Warn().
				Err(errors.New("wrong block delivery")).
				Str("expectHash", hex.EncodeToString(tasks[i].blockHash)).
				Str("gotHash", hex.EncodeToString(gotHash[:]))
			failedTasks = append(failedTasks, tasks[i])
			continue
		}
		// Verify block signatures
		if block.NumberU64() > 1 {
			// Verify signature every 100 blocks
			haveCurrentSig := len(block.GetCurrentCommitSig()) != 0
			verifySeal := block.NumberU64()%verifyHeaderBatchSize == 0 || verifyAllSig
			verifyCurrentSig := verifyAllSig && haveCurrentSig
			bc := b.configs.bc
			if err = b.verifyBlockSignatures(bc, block, verifyCurrentSig, verifySeal, verifyAllSig); err != nil {
				failedTasks = append(failedTasks, tasks[i])
				continue
			}
		}
		const sz = unsafe.Sizeof(*block)
		blockRawBytes := *(*[sz]byte)(unsafe.Pointer(block))
		k := tasks[i].index //strconv.FormatInt(int64(tasks[i].index), 10)
		s.state.downloadedBlocks[k] = blockRawBytes[:]
	}

	return failedTasks, nil
}

//verifyBlockSignatures verifies block signatures
func (b *StageBodies) verifyBlockSignatures(bc *core.BlockChain, block *types.Block, verifyCurrentSig bool, verifySeal bool, verifyAllSig bool) (err error) {
	if verifyCurrentSig {
		sig, bitmap, err := chain.ParseCommitSigAndBitmap(block.GetCurrentCommitSig())
		if err != nil {
			return errors.Wrap(err, "parse commitSigAndBitmap")
		}

		startTime := time.Now()
		if err := bc.Engine().VerifyHeaderSignature(bc, block.Header(), sig, bitmap); err != nil {
			return errors.Wrapf(err, "verify header signature %v", block.Hash().String())
		}
		utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("[STAGED_SYNC] VerifyHeaderSignature")
	}
	// err = bc.Engine().VerifyHeader(bc, block.Header(), verifySeal)
	// if err == engine.ErrUnknownAncestor {
	// 	fmt.Println("signature failed here ---------------> 5:", err)
	// 	return err
	// } else if err != nil {
	// 	utils.Logger().Error().Err(err).Msgf("[STAGED_SYNC] UpdateBlockAndStatus: failed verifying signatures for new block %d", block.NumberU64())

	// 	if !verifyAllSig {
	// 		utils.Logger().Info().Interface("block", bc.CurrentBlock()).Msg("[STAGED_SYNC] UpdateBlockAndStatus: Rolling back last 99 blocks!")
	// 		for i := uint64(0); i < verifyHeaderBatchSize-1; i++ {
	// 			if rbErr := bc.Rollback([]common.Hash{bc.CurrentBlock().Hash()}); rbErr != nil {
	// 				utils.Logger().Err(rbErr).Msg("[STAGED_SYNC] UpdateBlockAndStatus: failed to rollback")
	// 				return err
	// 			}
	// 		}
	// 	}
	// 	return err
	// }
	return nil
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

	fmt.Println("load block hashes to task queue -----------> from", startIndex, "size", size)

	if errV := b.configs.db.View(b.configs.ctx, func(etx kv.Tx) error {

		for i := startIndex; i < startIndex+size; i++ {
			key := strconv.FormatUint(i, 10)
			id := int(i - startIndex)
			blockHash, err := etx.GetOne(BlockHashesBucket, []byte(key))
			if err != nil {
				return err
			}
			if err := s.state.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash}); err != nil {
				s.state.stateSyncTaskQueue = queue.New(0)
				utils.Logger().Warn().
					Err(err).
					Int("taskIndex", id).
					Str("taskBlock", hex.EncodeToString(blockHash)).
					Msg("[STAGED_SYNC] generateStateSyncTaskQueue: cannot add task")
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
	taskIDs := make([]int, 0, len(s.state.downloadedBlocks))
	for id := range s.state.downloadedBlocks {
		fmt.Println("k---------------->",id)
		taskIDs = append(taskIDs, id)
	}
	sort.Ints(taskIDs)
	for taskID := range taskIDs {
		blockBytes := s.state.downloadedBlocks[taskID]
		fmt.Println("saving block: ", taskID, "----------[D]--------> len: ", len(blockBytes))
		//key := strconv.FormatUint(p+1, 10)
		key := fmt.Sprintf("%020d", p)
		if err := tx.Put(DownloadedBlocksBucket, []byte(key), blockBytes); err != nil {
			utils.Logger().Warn().
				Err(err).
				Uint64("block height", p).
				Msg("[STAGED_SYNC] adding block to db failed")
			return p, err
		}
		p++
	}

	// fmt.Println("old progress---SSS---------->", progress)
	// fmt.Println("new progress---SSS---------->", p)
	// fmt.Println("exp progress---SSS---------->", len(s.state.downloadedBlocks))

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
