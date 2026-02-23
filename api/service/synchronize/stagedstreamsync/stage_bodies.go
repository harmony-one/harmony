package stagedstreamsync

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p/stream/common/requestmanager"
	syncProto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type StageBodies struct {
	configs StageBodiesCfg
}

type StageBodiesCfg struct {
	bc                   core.BlockChain
	db                   kv.RwDB
	blockDBs             []kv.RwDB
	concurrency          int
	protocol             syncProtocol
	extractReceiptHashes bool
	logProgress          bool
	logger               zerolog.Logger
}

type blockTask struct {
	bns    []uint64
	hashes []common.Hash
}

func NewStageBodies(cfg StageBodiesCfg) *StageBodies {
	return &StageBodies{
		configs: cfg,
	}
}

func NewStageBodiesCfg(bc core.BlockChain, db kv.RwDB, blockDBs []kv.RwDB, concurrency int, protocol syncProtocol, extractReceiptHashes bool, logger zerolog.Logger, logProgress bool) StageBodiesCfg {
	return StageBodiesCfg{
		bc:                   bc,
		db:                   db,
		blockDBs:             blockDBs,
		concurrency:          concurrency,
		protocol:             protocol,
		extractReceiptHashes: extractReceiptHashes,
		logger: logger.With().
			Str("stage", "StageBodies").
			Str("mode", "long range").
			Logger(),
		logProgress: logProgress,
	}
}

// Exec progresses Bodies stage in the forward direction
func (b *StageBodies) Exec(ctx context.Context, firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) (err error) {

	useInternalTx := tx == nil

	// for short range sync, skip this stage
	if !s.state.initSync {
		return nil
	}

	// shouldn't execute for epoch chain
	if s.state.isEpochChain {
		return nil
	}

	if invalidBlockRevert {
		return b.redownloadBadBlock(ctx, s)
	}

	if useInternalTx {
		var err error
		tx, err = b.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	maxHeight := s.state.status.GetTargetBN()
	currentHead := s.state.CurrentBlockNumber()
	if currentHead >= maxHeight {
		return nil
	}
	currProgress := uint64(0)
	targetHeight := s.state.currentCycle.GetTargetHeight()

	if errV := CreateView(ctx, b.configs.db, tx, func(etx kv.Tx) error {
		if currProgress, err = s.CurrentStageProgress(etx); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	// if currProgress is 0, reset to currentHead
	if currProgress == 0 {
		currProgress = currentHead
		// update progress in db
		if err := s.Update(tx, currProgress); err != nil {
			return err
		}
	}

	// if currProgress is not equal to currentHead, clean all block DBs
	// because it means stage was interrupted and we need to start from scratch
	// this is to prevent the case where the block bodies are not saved in the cache databases or are corrupted
	// we can't validate the block bodies from currentHead+1 to currProgress because the block bodies are per stream and
	// download details are not available
	if currProgress != currentHead {
		b.configs.logger.Info().
			Uint64("currProgress", currProgress).
			Uint64("currentHead", currentHead).
			Msg("[STAGED_STREAM_SYNC] block bodies validation failed, clearing all block DBs and resetting progress to currentHead")
		if err := b.cleanAllBlockDBs(ctx); err != nil {
			b.configs.logger.Error().
				Err(err).
				Msg("[STAGED_STREAM_SYNC] clear all block DBs failed")
			return err
		}
		currProgress = currentHead
		// update progress in db
		if err := s.Update(tx, currProgress); err != nil {
			return err
		}
	}

	// currProgress is already equal to currentHead
	// so if it's already caught up to targetHeight, it must skip the download loop
	if currProgress >= targetHeight {
		return nil
	}

	startTime := time.Now()
	// startBlock := currProgress
	if b.configs.logProgress {
		fmt.Print("\033[s") // save the cursor position
	}

	// Fetch blocks from neighbors
	s.state.gbm = newDownloadManager(b.configs.bc, currProgress, targetHeight, BlocksPerRequest, s.state.logger)

	// Identify available valid streams.
	// Use a timeout so a dead/slow whitelisted stream cannot stall the entire sync.
	identifyCtx, identifyCtxCancel := context.WithTimeout(context.Background(), IdentifyStreamsTimeout)
	defer identifyCtxCancel()
	whitelistStreams, err := b.identifySyncedStreams(identifyCtx, s, targetHeight, []sttypes.StreamID{})
	if err != nil {
		b.configs.logger.Error().
			Err(err).
			Uint64("targetHeight", targetHeight).
			Msg(WrapStagedSyncMsg("identifying synced streams failed"))
		return err
	}

	b.runDownloadLoop(ctx, tx, s.state.gbm, s, whitelistStreams, currProgress, startTime)

	if err := b.saveProgress(ctx, s, targetHeight, tx); err != nil {
		b.configs.logger.Error().
			Err(err).
			Uint64("targetHeight", targetHeight).
			Msg(WrapStagedSyncMsg("save progress failed"))
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

// identifySyncedStreams queries all available streams for their current block number
// and returns those at or above targetHeight.
// Results map: streamID → error (nil = synced, non-nil = failure reason).
// Streams below target or with context errors are not recorded.
// Failed streams are only punished when synced streams exist; otherwise the
// stream pool is preserved to avoid cascading removal during systemic issues.
func (b *StageBodies) identifySyncedStreams(ctx context.Context, s *StageState, targetHeight uint64, excludeIDs []sttypes.StreamID) (streams []sttypes.StreamID, err error) {
	results := sttypes.NewSafeMap[sttypes.StreamID, error]()
	var (
		wg          sync.WaitGroup
		syncedCount int32
		failedCount int32
	)

	numStreams := b.configs.protocol.NumStreams()
	streamIDs := b.configs.protocol.GetStreamIDs()

	for i := 0; i < numStreams; i++ {
		excluded := false
		if len(excludeIDs) > 0 {
			for _, excludedStreamID := range excludeIDs {
				if excludedStreamID == streamIDs[i] {
					excluded = true
					break
				}
			}
		}
		if excluded {
			continue
		}
		stID := streamIDs[i]
		wg.Add(1)
		go func(stid sttypes.StreamID, targetHeight uint64) {
			defer wg.Done()

			var bn uint64
			var err error
			if s.state.bnCache != nil {
				bn, err = s.state.bnCache.GetBlockNumber(ctx, stid, targetHeight)
			} else {
				bn, _, err = b.configs.protocol.GetCurrentBlockNumber(ctx, syncProto.WithWhitelist([]sttypes.StreamID{stid}))
			}
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				results.Set(stid, err)
				atomic.AddInt32(&failedCount, 1)
				return
			}

			if bn >= targetHeight {
				results.Set(stid, nil)
				atomic.AddInt32(&syncedCount, 1)
			}
		}(stID, targetHeight)
	}

	wg.Wait()

	if syncedCount == 0 {
		if failedCount > 0 {
			b.configs.logger.Warn().
				Int32("failedStreams", failedCount).
				Msg(WrapStagedSyncMsg("[identifySyncedStreams] no synced streams found; skipping punishment to preserve stream pool"))
		}
		return nil, ErrZeroBlockResponse
	}

	streams = make([]sttypes.StreamID, 0, syncedCount)
	results.Iterate(func(stid sttypes.StreamID, err error) {
		if err == nil {
			streams = append(streams, stid)
		} else {
			b.handleIdentifyStreamFailure(s, stid, err)
		}
	})

	return streams, nil
}

func (b *StageBodies) handleIdentifyStreamFailure(s *StageState, stid sttypes.StreamID, err error) {
	severity := requestmanager.ClassifyRequestError(err)

	switch severity {
	case requestmanager.RequestErrorSkip:
		b.configs.logger.Debug().Err(err).Str("streamID", string(stid)).
			Msg(WrapStagedSyncMsg("[identifySyncedStreams] skipping non-stream error"))

	case requestmanager.RequestErrorCritical:
		b.configs.logger.Warn().Err(err).Str("streamID", string(stid)).
			Msg(WrapStagedSyncMsg("[identifySyncedStreams] removing stream due to critical error"))
		if s.state.bnCache != nil {
			s.state.bnCache.RemoveStream(stid)
		}
		b.configs.protocol.RemoveStream(stid, "identifySyncedStreams: critical protocol error")

	default:
		b.configs.logger.Info().Err(err).Str("streamID", string(stid)).
			Msg(WrapStagedSyncMsg("[identifySyncedStreams] marking stream as failed"))
		if s.state.bnCache != nil {
			s.state.bnCache.InvalidateStream(stid)
		}
		b.configs.protocol.StreamFailed(stid, "identifySyncedStreams: request failed")
	}
}

func (b *StageBodies) runDownloadLoop(ctx context.Context, tx kv.RwTx, gbm *downloadManager, s *StageState, wl []sttypes.StreamID, startBlockNumber uint64, startTime time.Time) {
	currentBlock := startBlockNumber
	concurrency := s.state.config.Concurrency
	if b.configs.protocol.NumStreams() < concurrency {
		concurrency = b.configs.protocol.NumStreams()
	}

	taskChan := make(chan blockTask)
	allWorkersDone := make(chan struct{})
	var once sync.Once

	var wg sync.WaitGroup
	activeWorkers := int32(concurrency)
	busyWorkers := int32(0)

	handleTask := func(workerID int, task blockTask) error {
		atomic.AddInt32(&busyWorkers, +1)
		defer atomic.AddInt32(&busyWorkers, -1)
		if err := b.runBlockWorker(ctx, gbm, task.bns, task.hashes, workerID, wl); err != nil {
			return err
		}
		return nil
	}

	// Worker function
	worker := func(workerID int) {
		defer wg.Done()
		defer func() {
			atomic.AddInt32(&activeWorkers, -1)
			if atomic.LoadInt32(&activeWorkers) == 0 {
				once.Do(func() { close(allWorkersDone) }) // Ensure only one workers close all
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case task, ok := <-taskChan:
				if !ok {
					return
				}
				if err := handleTask(workerID, task); err != nil {
					continue
				}
			}
		}
	}

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(i)
	}

	// Cleanup
	defer func() {
		close(taskChan)
		wg.Wait()
	}()

	// Dispatcher loop
	noWorkCounter := 0
	for atomic.LoadInt32(&activeWorkers) > 0 {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		// check if workers are done
		if atomic.LoadInt32(&activeWorkers) == 0 {
			return
		}

		// Get next batch
		batch := gbm.GetNextBatch()
		if len(batch) == 0 {
			noWorkCounter++
			// No more work - check if all workers are available
			if noWorkCounter >= 3 && atomic.LoadInt32(&busyWorkers) == 0 {
				return
			}

			select {
			case <-time.After(100 * time.Millisecond):
				continue
			case <-ctx.Done():
				return
			}
		}
		noWorkCounter = 0 // Reset counter if work is found

		// Prepare task
		hashes, err := b.fetchBlockHashes(ctx, tx, batch)
		if err != nil {
			panic(fmt.Errorf("fetchBlockHashes failed: %w", err))
		}

		// Send to worker (blocks until worker is available)
		select {
		case taskChan <- blockTask{bns: batch, hashes: hashes}:
		case <-ctx.Done():
			return
		case <-allWorkersDone:
			return
		}

		// Logging progress
		if b.configs.logProgress {
			lastBlockInBatch := batch[len(batch)-1]
			if lastBlockInBatch > currentBlock {
				currentBlock = lastBlockInBatch
			}
			//calculating block download speed
			dt := time.Since(startTime).Seconds()
			speed := float64(0)
			numBlocks := uint64(len(gbm.details))

			if dt > 0 {
				speed = float64(numBlocks) / dt
			}
			blockSpeed := fmt.Sprintf("%.2f", speed)

			fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
			fmt.Println("downloaded blocks:", currentBlock, "/", int(gbm.targetBN), "(", blockSpeed, "blocks/s", ")")
		}
	}
}

// runBlockWorker downloads and processes a single batch of blocks
func (b *StageBodies) runBlockWorker(ctx context.Context,
	gbm *downloadManager,
	bns []uint64,
	hashes []common.Hash,
	workerID int,
	wl []sttypes.StreamID) error {

	if len(hashes) == 0 {
		return errors.New("empty hashes")
	}

	var blockBytes, sigBytes [][]byte
	var stid sttypes.StreamID
	var err error

	if len(wl) > 0 {
		blockBytes, sigBytes, stid, err = b.configs.protocol.GetRawBlocksByHashes(ctx, hashes, syncProto.WithWhitelist(wl))
	} else {
		blockBytes, sigBytes, stid, err = b.configs.protocol.GetRawBlocksByHashes(ctx, hashes)
	}

	if err != nil {
		// could be Protocol/decoding, Empty request, Remote error response, Exceeds cap, Length mismatch
		// check if the error is due to context cancelation or deadline exceeded. if not, mark the stream as failed
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) && stid != "" {
			b.configs.protocol.StreamFailed(stid, "downloadRawBlocks failed")
		}
		// try with other streams
		if len(wl) > 0 {
			utils.Logger().Warn().
				Interface("block numbers", bns).
				Msg("downloadRawBlocks failed with whitelist, will try with other streams")
			return b.runBlockWorker(ctx, gbm, bns, hashes, workerID, []sttypes.StreamID{})
		}
		utils.Logger().Error().
			Err(err).
			Str("stream", string(stid)).
			Interface("block numbers", bns).
			Msg(WrapStagedSyncMsg("downloadRawBlocks failed"))
		err = errors.Wrap(err, "request error")
		gbm.HandleRequestError(bns, err, stid)
		return err
	} else if blockBytes == nil { // Protocol/decoding, Empty request, Remote error response, Exceeds cap, Length mismatch
		utils.Logger().Warn().
			Str("stream", string(stid)).
			Interface("block numbers", bns).
			Msg(WrapStagedSyncMsg("downloadRawBlocks failed, received invalid (nil) blockBytes"))
		err := errors.New("downloadRawBlocks received invalid (nil) blockBytes")
		gbm.HandleRequestError(bns, err, stid)
		b.configs.protocol.StreamFailed(stid, "downloadRawBlocks received nil blockBytes")
		return err
	} else if len(blockBytes) == 0 { // All blocks missing
		utils.Logger().Warn().
			Str("stream", string(stid)).
			Interface("block numbers", bns).
			Msg(WrapStagedSyncMsg("downloadRawBlocks failed, received empty blockBytes, remote peer is not fully synced"))
		err := errors.New("downloadRawBlocks received empty blockBytes")
		gbm.HandleRequestError(bns, err, stid)
		b.configs.protocol.RemoveStream(stid, "downloadRawBlocks received empty blockBytes")
		return err
	} else if len(blockBytes) != len(bns) { // in this case, the blockBytes would be nil, So technically it should not happen
		utils.Logger().Warn().
			Str("stream", string(stid)).
			Interface("block numbers", bns).
			Msg(WrapStagedSyncMsg("downloadRawBlocks failed, received blockBytes length is not match with requested block numbers"))
		err := errors.New("downloadRawBlocks received blockBytes length is not match with requested block numbers")
		gbm.HandleRequestError(bns, err, stid)
		b.configs.protocol.RemoveStream(stid, "downloadRawBlocks received unexpected blockBytes")
		return err
	} else {
		// save valid blockBytes to db
		invalidBlocks, failedToSaveBlocks, savedBlocks, err := b.saveBlocks(ctx, nil, bns, blockBytes, sigBytes, workerID, stid)
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Str("stream", string(stid)).
				Interface("block numbers", bns).
				Msg(WrapStagedSyncMsg("saveBlocks failed"))
			panic(ErrSaveBlocksToDbFailed)
		}
		if len(invalidBlocks) > 0 {
			utils.Logger().Warn().
				Str("stream", string(stid)).
				Interface("block numbers", bns).
				Interface("invalid blocks", invalidBlocks).
				Msg(WrapStagedSyncMsg("saveBlocks failed, there are some blocks that are not valid"))
			if len(failedToSaveBlocks) == len(bns) { // all blocks are invalid
				b.configs.protocol.RemoveStream(stid, "downloadRawBlocks received blockBytes are not valid")
			} else { // some blocks are invalid
				b.configs.protocol.StreamFailed(stid, "downloadRawBlocks received blockBytes are not valid")
			}
			gbm.HandleRequestError(invalidBlocks, err, stid)
		}
		if len(failedToSaveBlocks) > 0 {
			utils.Logger().Warn().
				Str("stream", string(stid)).
				Interface("block numbers", bns).
				Interface("failed to save blocks", failedToSaveBlocks).
				Msg(WrapStagedSyncMsg("saveBlocks failed, it should be db issue"))
			gbm.HandleRequestError(failedToSaveBlocks, err, sttypes.StreamID(""))
		}
		if len(savedBlocks) > 0 {
			gbm.HandleRequestResult(savedBlocks, blockBytes, sigBytes, workerID, stid)
		}
		return nil
	}
}

func (b *StageBodies) verifyBlockAndExtractReceiptsData(batchBlockBytes [][]byte, batchSigBytes [][]byte, s *StageState) error {
	for i := uint64(0); i < uint64(len(batchBlockBytes)); i++ {
		blockBytes := batchBlockBytes[i]
		sigBytes := batchSigBytes[i]
		if blockBytes == nil {
			continue
		}
		block, err := core.RlpDecodeBlockOrBlockWithSig(blockBytes)
		if err != nil {
			b.configs.logger.Error().
				Uint64("block number", i).
				Err(err).
				Msg("block RLP decode failed")
			return ErrInvalidBlockBytes
		}
		// Set signature from response if available
		if sigBytes != nil {
			block.SetCurrentCommitSig(sigBytes)
		}

		// if block.NumberU64() != i {
		// 	return ErrInvalidBlockNumber
		// }
		if err := verifyBlock(b.configs.bc, block); err != nil {
			return err
		}
	}
	return nil
}

// redownloadBadBlock tries to redownload the bad block from other streams with retry limits and improved error handling.
func (b *StageBodies) redownloadBadBlock(ctx context.Context, s *StageState) error {
	batch := []uint64{s.state.invalidBlock.Number}
	maxRetries := b.configs.protocol.NumStreams()
	retryDelay := 5 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if b.configs.protocol.NumStreams() == 0 {
			b.configs.logger.Error().
				Uint64("bad block number", s.state.invalidBlock.Number).
				Msg("[STAGED_STREAM_SYNC] Not enough streams to re-download bad block")
			return ErrNotEnoughStreams
		}

		reidentifyCtx, reidentifyCtxCancel := context.WithTimeout(context.Background(), IdentifyStreamsTimeout)
		whitelistStreams, err := b.identifySyncedStreams(reidentifyCtx, s, s.state.invalidBlock.Number, s.state.invalidBlock.StreamID)
		reidentifyCtxCancel()
		if len(whitelistStreams) == 0 {
			b.configs.logger.Error().
				Uint64("bad block number", s.state.invalidBlock.Number).
				Interface("excluded streams", s.state.invalidBlock.StreamID).
				Msg("[STAGED_STREAM_SYNC] All available streams have failed for this bad block")
			return ErrNotEnoughStreams
		}

		blockBytes, sigBytes, stid, err := b.configs.protocol.GetRawBlocksByNumber(ctx, batch, syncProto.WithWhitelist(whitelistStreams))
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				b.configs.logger.Warn().Msg("[STAGED_STREAM_SYNC] Re-download canceled or timed out")
				time.Sleep(retryDelay)
				continue
			}

			b.configs.logger.Error().
				Err(err).
				Uint64("bad block number", s.state.invalidBlock.Number).
				Interface("stream ID", stid).
				Msg("[STAGED_STREAM_SYNC] Failed to download bad block, marking stream as failed")
			b.configs.protocol.StreamFailed(stid, "failed to re-download bad block")
			s.state.invalidBlock.addBadStream(stid)
			time.Sleep(retryDelay)
			continue
		}

		if len(blockBytes) <= 1 {
			b.configs.logger.Error().
				Err(errors.New("invalid block bytes")).
				Uint64("bad block number", s.state.invalidBlock.Number).
				Interface("stream ID", stid).
				Msg("[STAGED_STREAM_SYNC] Bad block's blockBytes is invalid, marking stream as failed")
			b.configs.protocol.StreamFailed(stid, "failed to re-download bad block")
			s.state.invalidBlock.addBadStream(stid)
			time.Sleep(retryDelay)
			continue
		}

		// Save block details and persist the re-downloaded data
		s.state.gbm.SetDownloadDetails(batch, 0, stid)
		_, _, savedBlocks, err := b.saveBlocks(ctx, nil, batch, blockBytes, sigBytes, 0, stid)
		if err != nil || len(savedBlocks) != len(batch) {
			b.configs.logger.Error().
				Err(err).
				Uint64("bad block number", s.state.invalidBlock.Number).
				Msg("[STAGED_STREAM_SYNC] Saving re-downloaded bad block to DB failed")
			return errors.Errorf("%s: %s", ErrSaveBlocksToDbFailed.Error(), err.Error())
		}

		b.configs.logger.Info().
			Uint64("bad block number", s.state.invalidBlock.Number).
			Interface("retried on stream", stid).
			Int("attempt", attempt).
			Msg("[STAGED_STREAM_SYNC] Successfully re-downloaded bad block")
		return nil
	}

	return errors.Errorf("[STAGED_STREAM_SYNC] Max retries reached for bad block %d", s.state.invalidBlock.Number)
}

func (b *StageBodies) downloadBlocks(ctx context.Context, bns []uint64) ([]*types.Block, sttypes.StreamID, error) {
	blocks, stid, err := b.configs.protocol.GetBlocksByNumber(ctx, bns)
	if err != nil {
		return nil, stid, err
	}
	if err := validateGetBlocksResult(bns, blocks); err != nil {
		return nil, stid, err
	}
	return blocks, stid, nil
}

// TODO: validate block results
func validateGetBlocksResult(requested []uint64, result []*types.Block) error {
	if len(result) != len(requested) {
		return fmt.Errorf("unexpected number of blocks delivered: %v / %v", len(result), len(requested))
	}
	for i, block := range result {
		if block != nil && block.NumberU64() != requested[i] {
			return fmt.Errorf("block with unexpected number delivered: %v / %v", block.NumberU64(), requested[i])
		}
	}
	return nil
}

func (b *StageBodies) fetchBlockHashes(ctx context.Context, tx kv.RwTx, bns []uint64) ([]common.Hash, error) {
	if len(bns) == 0 {
		return nil, errors.New("empty batch of block numbers")
	}

	hashes := make([]common.Hash, 0, len(bns))

	err := CreateView(ctx, b.configs.db, tx, func(etx kv.Tx) error {
		for _, bn := range bns {

			blkKey := marshalData(bn)
			hashBytes, err := etx.GetOne(BlockHashesBucket, blkKey)
			if err != nil {
				utils.Logger().Error().
					Err(err).
					Uint64("block number", bn).
					Msg("[STAGED_STREAM_SYNC] fetching block hash from db failed")
				return err
			}
			var h common.Hash
			copy(h[:], hashBytes)
			hashes = append(hashes, h)
		}
		return nil
	})

	return hashes, err
}

func (b *StageBodies) downloadRawBlocksByHashes(ctx context.Context, bns []uint64) ([][]byte, [][]byte, sttypes.StreamID, error) {
	if len(bns) == 0 {
		return nil, nil, "", errors.New("empty batch of block numbers")
	}

	tx, err := b.configs.db.BeginRw(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	defer tx.Rollback()

	hashes := make([]common.Hash, 0, len(bns))

	if err := CreateView(ctx, b.configs.db, tx, func(etx kv.Tx) error {
		for _, bn := range bns {
			blkKey := marshalData(bn)
			hashBytes, err := etx.GetOne(BlockHashesBucket, blkKey)
			if err != nil {
				b.configs.logger.Error().
					Err(err).
					Uint64("block number", bn).
					Msg("[STAGED_STREAM_SYNC] fetching block hash from db failed")
				return err
			}
			var h common.Hash
			h.SetBytes(hashBytes)
			hashes = append(hashes, h)
		}
		return nil
	}); err != nil {
		return nil, nil, "", err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, "", err
	}

	// TODO: check the returned blocks are sorted
	return b.configs.protocol.GetRawBlocksByHashes(ctx, hashes)
}

// saveBlocks saves the blocks into db
func (b *StageBodies) saveBlocks(ctx context.Context, tx kv.RwTx, bns []uint64, blockBytes [][]byte, sigBytes [][]byte, workerID int, stid sttypes.StreamID) ([]uint64, []uint64, []uint64, error) {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = b.configs.blockDBs[workerID].BeginRw(ctx)
		if err != nil {
			return nil, nil, nil, err
		}
		defer tx.Rollback()
	}

	var invalidBlocks []uint64
	var failedToSaveBlocks []uint64
	var savedBlocks []uint64

	// The blocks array is sorted by block number
	for i := uint64(0); i < uint64(len(blockBytes)); i++ {
		block := blockBytes[i]
		sig := sigBytes[i]
		if block == nil || len(block) <= 1 || sig == nil || len(sig) <= 1 {
			invalidBlocks = append(invalidBlocks, bns[i])
			continue
		}

		blkKey := marshalData(bns[i])

		if err := tx.Put(BlocksBucket, blkKey, block); err != nil {
			b.configs.logger.Error().
				Err(err).
				Uint64("block height", bns[i]).
				Msg("[STAGED_STREAM_SYNC] adding block to db failed")
			failedToSaveBlocks = append(failedToSaveBlocks, bns[i])
			continue
		}
		// sigKey := []byte("s" + string(bns[i]))
		if err := tx.Put(BlockSignaturesBucket, blkKey, sig); err != nil {
			b.configs.logger.Error().
				Err(err).
				Uint64("block height", bns[i]).
				Msg("[STAGED_STREAM_SYNC] adding block sig to db failed")
			failedToSaveBlocks = append(failedToSaveBlocks, bns[i])
			continue
		}

		savedBlocks = append(savedBlocks, bns[i])
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return nil, nil, nil, err
		}
	}

	return invalidBlocks, failedToSaveBlocks, savedBlocks, nil
}

func (b *StageBodies) saveProgress(ctx context.Context, s *StageState, progress uint64, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = b.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// save progress
	if err = s.Update(tx, progress); err != nil {
		b.configs.logger.Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] saving progress for block bodies stage failed")
		return ErrSavingBodiesProgressFail
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (b *StageBodies) cleanBlocksDB(ctx context.Context, workerID int) (err error) {
	tx, errb := b.configs.blockDBs[workerID].BeginRw(ctx)
	if errb != nil {
		return errb
	}
	defer tx.Rollback()

	// clean block bodies db
	if err = tx.ClearBucket(BlocksBucket); err != nil {
		b.configs.logger.Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] clear blocks bucket after revert failed")
		return err
	}
	// clean block signatures db
	if err = tx.ClearBucket(BlockSignaturesBucket); err != nil {
		b.configs.logger.Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] clear block signatures bucket after revert failed")
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (b *StageBodies) cleanAllBlockDBs(ctx context.Context) (err error) {
	//clean all blocks DBs
	for i := 0; i < b.configs.concurrency; i++ {
		if err := b.cleanBlocksDB(ctx, i); err != nil {
			return err
		}
	}
	return nil
}

func (b *StageBodies) Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {

	//clean all blocks DBs
	if err := b.cleanAllBlockDBs(ctx); err != nil {
		b.configs.logger.Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] StageBodies Revert: clean all block DBs after revert failed")
		return err
	}

	// Clean up download details during revert to prevent memory leaks
	if s.state != nil && s.state.gbm != nil {
		s.state.gbm.CleanupAllDetails()
	}

	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = b.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}
	// save progress
	currentHead := s.state.CurrentBlockNumber()
	if err = s.Update(tx, currentHead); err != nil {
		b.configs.logger.Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] saving progress for block bodies stage after revert failed")
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

func (b *StageBodies) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	//clean all blocks DBs
	if err := b.cleanAllBlockDBs(ctx); err != nil {
		b.configs.logger.Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] StageBodies CleanUp: clean all block DBs after cleanup failed")
		return err
	}

	// Clean up download details during cleanup to prevent memory leaks
	if p.state != nil && p.state.gbm != nil {
		p.state.gbm.CleanupAllDetails()
	}

	return nil
}
