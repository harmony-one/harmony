package stagedstreamsync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Hash frequency and stream tracking
type HashStats struct {
	Count     int
	StreamIDs map[sttypes.StreamID]struct{}
}

type StageBlockHashes struct {
	configs StageBlockHashesCfg
}

type StageBlockHashesCfg struct {
	bc          core.BlockChain
	db          kv.RwDB
	concurrency int
	protocol    syncProtocol
	//bgProcRunning bool
	cachedb     kv.RwDB
	logProgress bool
	logger      zerolog.Logger
}

func NewStageBlockHashes(cfg StageBlockHashesCfg) *StageBlockHashes {
	return &StageBlockHashes{
		configs: cfg,
	}
}

func NewStageBlockHashesCfg(bc core.BlockChain, db kv.RwDB, concurrency int, protocol syncProtocol, logger zerolog.Logger, logProgress bool) StageBlockHashesCfg {
	return StageBlockHashesCfg{
		bc:          bc,
		db:          db,
		concurrency: concurrency,
		protocol:    protocol,
		logProgress: logProgress,
		logger: logger.With().
			Str("stage", "StageBlockHashes").
			Str("mode", "long range").
			Logger(),
	}
}

func (bh *StageBlockHashes) Exec(ctx context.Context, firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) (err error) {

	useInternalTx := tx == nil

	if invalidBlockRevert {
		return nil //bh.redownloadBadBlock(ctx, s)
	}

	// for short range sync, skip this stage
	if !s.state.initSync {
		return nil
	}

	// shouldn't execute for epoch chain
	if s.state.isEpochChain {
		return nil
	}

	// isBeacon := s.state.isBeaconValidator
	maxHeight := s.state.status.GetTargetBN()
	currentHead := s.state.CurrentBlockNumber()
	if currentHead >= maxHeight {
		return nil
	}

	currProgress := uint64(0)
	targetHeight := s.state.currentCycle.GetTargetHeight()

	if useInternalTx {
		var err error
		tx, err = bh.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if errV := CreateView(ctx, bh.configs.db, tx, func(etx kv.Tx) error {
		if currProgress, err = s.CurrentStageProgress(etx); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	if currProgress < currentHead {
		if err := bh.clearBlockHashesBucket(tx); err != nil {
			return err
		}
		currProgress = currentHead
	} else if currProgress >= targetHeight {
		//TODO: validate hashes (from currentHead+1 to targetHeight)
		return nil
	} else if currProgress > currentHead && currProgress < targetHeight {
		// TODO: validate hashes (from currentHead to currProgress)

		// key := strconv.FormatUint(currProgress, 10)
		// bucketName := bh.configs.blockDBs
		// currHash := []byte{}
		// if currHash, err = etx.GetOne(bucketName, []byte(key)); err != nil || len(currHash[:]) == 0 {
		// 	//TODO: currProgress and DB don't match. Either re-download all or verify db and set currProgress to last
		// 	return err
		// }
		// startHash.SetBytes(currHash[:])
	}

	startTime := time.Now()

	// startBlock := currProgress
	if bh.configs.logProgress {
		fmt.Print("\033[s") // save the cursor position
	}

	// create download manager instant
	hdm := newDownloadManager(bh.configs.bc, currProgress, targetHeight, BlockHashesPerRequest, bh.configs.logger)

	// Fetch block hashes from neighbors
	if err := bh.runBlockHashWorkerLoop(ctx, tx, hdm, s, startTime, currentHead, targetHeight); err != nil {
		return err
	}

	// Clean up download details to prevent memory leaks
	hdm.CleanupAllDetails()

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (bh *StageBlockHashes) downloadBlockHashes(ctx context.Context, bns []uint64) ([]common.Hash, sttypes.StreamID, error) {
	hashes, stid, err := bh.configs.protocol.GetBlockHashes(ctx, bns)
	if hashes == nil {
		bh.configs.logger.Warn().
			Interface("bns", bns).
			Msg("[StageBlockHashes] downloadBlockHashes Nil Response")
		bh.configs.protocol.RemoveStream(stid, "nil block hashes")
		return []common.Hash{}, stid, errors.New("nil response for hashes")
	}
	bh.configs.logger.Info().
		Int("request size", len(bns)).
		Int("received size", len(hashes)).
		Interface("stid", stid).
		Msg("[StageBlockHashes] downloadBlockHashes received hashes")
	if len(hashes) > len(bns) {
		bh.configs.logger.Warn().
			Int("request size", len(bns)).
			Int("received size", len(hashes)).
			Interface("stid", stid).
			Msg("[StageBlockHashes] received more blockHashes than requested!")
		return hashes[:len(bns)], stid, nil
	} else if len(hashes) < len(bns) {
		utils.Logger().Warn().
			Int("request size", len(bns)).
			Int("received size", len(hashes)).
			Interface("stid", stid).
			Msg("[StageBlockHashes] received less hashes than requested, target stream is not synced!")
		bh.configs.protocol.RemoveStream(stid, "expected more hashes")
		return []common.Hash{}, stid, errors.New("not synced stream")
	}
	return hashes, stid, err
}

// runBlockHashWorkerLoop downloads block hashes
func (bh *StageBlockHashes) runBlockHashWorkerLoop(ctx context.Context,
	tx kv.RwTx,
	hdm *downloadManager,
	s *StageState,
	startTime time.Time,
	startHeight uint64,
	targetHeight uint64) error {

	currProgress := startHeight

	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = bh.configs.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// Worker loop
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Fetch the next batch of block numbers
		batch := hdm.GetNextBatch()
		if len(batch) == 0 {
			break
		}

		// Map to store block hashes fetched from peers
		peerHashes := sttypes.NewSafeMap[sttypes.StreamID, []common.Hash]()
		var wg sync.WaitGroup

		if bh.configs.protocol.NumStreams() < bh.configs.concurrency {
			return ErrNotEnoughStreams
		}

		// Fetch block hashes concurrently
		for i := 0; i < bh.configs.concurrency; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				// Download block hashes
				hashes, stid, err := bh.downloadBlockHashes(ctx, batch)

				// could be Protocol/decoding error
				if err != nil {
					bh.configs.logger.Warn().
						Err(err).
						Interface("stid", stid).
						Msgf("[STAGED_STREAM_SYNC] downloadBlockHashes failed")
					if stid != "" {
						bh.configs.protocol.StreamFailed(stid, "downloadBlockHashes failed")
					}
					return
				}

				// No error but nil response could be Protocol/decoding error
				if hashes == nil {
					bh.configs.protocol.StreamFailed(stid, "nil block hashes")
					return
				}

				// Check if any hash is zero
				allZero, containsZero := bh.containsZeroHash(hashes)
				if allZero {
					bh.configs.protocol.RemoveStream(stid, "stream response contains all zero hashes") // Remove immediately
					return
				} else if containsZero {
					bh.configs.logger.Warn().
						Msgf("[STAGED_STREAM_SYNC] stream response contains zero hashes")
				}

				// Store valid hashes
				peerHashes.Set(stid, hashes)
			}()
		}

		// Wait for all workers to complete
		wg.Wait()

		// all workers failed
		if peerHashes.Length() == 0 {
			hdm.HandleRequestError(batch, errors.New("workers failed"), sttypes.StreamID(""))
			bh.configs.logger.Warn().
				Msgf("[STAGED_STREAM_SYNC] all block hash workers failed")
			continue
		}

		// Calculate the final block hashes for the batch
		finalBlockHashes, _, errFinalHashCalculations := bh.calculateFinalBlockHashes(peerHashes, batch)
		if errFinalHashCalculations != nil {
			panic(ErrFinalBlockHashesCalculationFailed)
		}
		peerHashes.Clear()

		// check if the final hashes contain zero hashes and if any block number is missing
		if validHashes, err := bh.checkFinalHashes(batch, finalBlockHashes); !validHashes {
			hdm.HandleRequestError(batch, errors.New("invalid final hashes"), sttypes.StreamID(""))
			bh.configs.logger.Warn().
				Err(err).
				Msgf("[STAGED_STREAM_SYNC] final hashes are invalid")
		}

		// save block hashes in db
		if err := bh.saveBlockHashes(ctx, tx, finalBlockHashes); err != nil {
			panic(ErrSaveBlockHashesToDbFailed)
		}
		hdm.HandleHashesRequestResult(batch)

		// update stage progress
		lastBlockInBatch := batch[len(batch)-1]
		if lastBlockInBatch > currProgress {
			currProgress = lastBlockInBatch
		}

		// save progress
		if err := bh.saveProgress(ctx, s, currProgress, tx); err != nil {
			bh.configs.logger.Error().
				Err(err).
				Msgf("[STAGED_STREAM_SYNC] saving progress for block hashes stage failed")
			return err
		}

		// log the stage progress in console
		if bh.configs.logProgress {
			dt := time.Since(startTime).Seconds()
			speed := float64(0)
			if dt > 0 {
				speed = float64(currProgress-startHeight) / dt
			}
			blockSpeed := fmt.Sprintf("%.2f", speed)
			fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
			fmt.Println("downloading block hash progress:", currProgress, "/", targetHeight, "(", blockSpeed, "blocks/s", ")")
		}
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

// containsZeroHash checks if the hashes contain zero hashes and if all hashes are zero
func (bh *StageBlockHashes) containsZeroHash(hashes []common.Hash) (bool, bool) {
	containsZero := false
	allZero := true
	for _, h := range hashes {
		if h == (common.Hash{}) { // Check if hash is all zeros
			containsZero = true
		} else {
			allZero = false
		}
	}
	return allZero, containsZero
}

// checkFinalHashes checks if the final hashes contain zero hashes and if any block number is missing
func (bh *StageBlockHashes) checkFinalHashes(batch []uint64, hashes map[uint64]common.Hash) (bool, error) {
	for _, bn := range batch {
		if h, ok := hashes[bn]; !ok {
			return false, errors.New("final hashes are missing")
		} else if h == (common.Hash{}) {
			return false, errors.New("final hashes contain zero hashes")
		}
	}
	return true, nil
}

// calculateFinalBlockHashes Calculates the most frequent block hashes for a given batch and removes streams with invalid hashes.
// note: final hashes could be zero hashes
func (bh *StageBlockHashes) calculateFinalBlockHashes(
	peerHashes *sttypes.SafeMap[sttypes.StreamID, []common.Hash],
	batch []uint64,
) (map[uint64]common.Hash, map[sttypes.StreamID]struct{}, error) {

	if len(batch) == 0 {
		return nil, nil, errors.New("batch is empty")
	}

	hashFrequency := make(map[uint64]map[common.Hash]*HashStats)
	finalHashes := make(map[uint64]common.Hash)
	invalidStreams := make(map[sttypes.StreamID]struct{})

	// Iterate over peerHashes to populate frequencies
	peerHashes.Iterate(func(stid sttypes.StreamID, hashes []common.Hash) {
		for i, blockNumber := range batch {
			if i >= len(hashes) {
				continue // Skip if the peer returned fewer hashes
			}

			hash := hashes[i]
			if _, ok := hashFrequency[blockNumber]; !ok {
				hashFrequency[blockNumber] = make(map[common.Hash]*HashStats)
			}

			if _, ok := hashFrequency[blockNumber][hash]; !ok {
				hashFrequency[blockNumber][hash] = &HashStats{
					Count:     0,
					StreamIDs: make(map[sttypes.StreamID]struct{}),
				}
			}

			details := hashFrequency[blockNumber][hash]
			details.Count++
			details.StreamIDs[stid] = struct{}{}

			if fh, ok := finalHashes[blockNumber]; !ok {
				finalHashes[blockNumber] = hash
			} else {
				if hashFreq, ok := hashFrequency[blockNumber][fh]; ok {
					if details.Count > len(hashFreq.StreamIDs) {
						finalHashes[blockNumber] = hash
					}
				}
			}
		}
	})

	// Identify invalid streams
	peerHashes.Iterate(func(stid sttypes.StreamID, hashes []common.Hash) {
		for i, blockNumber := range batch {
			if i >= len(hashes) || finalHashes[blockNumber] != hashes[i] {
				invalidStreams[stid] = struct{}{}
				bh.configs.protocol.RemoveStream(stid, "calculateFinalBlockHashes - invalid stream")
				break
			}
		}
	})

	return finalHashes, invalidStreams, nil
}

// saveBlockHashes saves the block hashes into db
func (bh *StageBlockHashes) saveBlockHashes(ctx context.Context, tx kv.RwTx, hashes map[uint64]common.Hash) error {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = bh.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	for bn, hash := range hashes {
		if hash == (common.Hash{}) {
			continue
		}

		blkKey := marshalData(bn)

		if err := tx.Put(BlockHashesBucket, blkKey, hash.Bytes()); err != nil {
			bh.configs.logger.Error().
				Err(err).
				Uint64("block number", bn).
				Msg("[STAGED_STREAM_SYNC] adding block hash to db failed")
			return err
		}
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (bh *StageBlockHashes) saveProgress(ctx context.Context, s *StageState, progress uint64, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = bh.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// save progress
	if err = s.Update(tx, progress); err != nil {
		bh.configs.logger.Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] saving progress for block hashes stage failed")
		return ErrSavingHashesProgressFail
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (bh *StageBlockHashes) clearBlockHashesBucket(tx kv.RwTx) error {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = bh.configs.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}
	if err := tx.ClearBucket(BlockHashesBucket); err != nil {
		return err
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// clearCache removes block hashes from cache db
func (bh *StageBlockHashes) clearCache() error {
	tx, err := bh.configs.cachedb.BeginRw(context.Background())
	if err != nil {
		return nil
	}
	defer tx.Rollback()
	if err := tx.ClearBucket(BlockHashesBucket); err != nil {
		return nil
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (bh *StageBlockHashes) Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = bh.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// // terminate background process in turbo mode
	// if bh.configs.bgProcRunning {
	// 	bh.configs.bgProcRunning = false
	// 	bh.configs.turboModeCh <- struct{}{}
	// 	close(bh.configs.turboModeCh)
	// }

	// clean block hashes db
	if err = tx.ClearBucket(BlockHashesBucket); err != nil {
		bh.configs.logger.Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] clear block hashes bucket after revert failed")
		return err
	}

	// clean cache db as well
	if err := bh.clearCache(); err != nil {
		bh.configs.logger.Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] clear block hashes cache failed")
		return err
	}

	// save progress
	currentHead := bh.configs.bc.CurrentBlock().NumberU64()
	if err = s.Update(tx, currentHead); err != nil {
		bh.configs.logger.Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] saving progress for block hashes stage after revert failed")
		return err
	}

	if err = u.Done(tx); err != nil {
		bh.configs.logger.Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] reset after revert failed")
		return err
	}

	if useInternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (bh *StageBlockHashes) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = bh.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// terminate background process in turbo mode
	// if bh.configs.bgProcRunning {
	// 	bh.configs.bgProcRunning = false
	// 	bh.configs.turboModeCh <- struct{}{}
	// 	close(bh.configs.turboModeCh)
	// }

	tx.ClearBucket(BlockHashesBucket)

	if useInternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
