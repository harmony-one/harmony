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
	isBeaconShard bool
	cachedb       kv.RwDB
	logProgress   bool
}

func NewStageBlockHashes(cfg StageBlockHashesCfg) *StageBlockHashes {
	return &StageBlockHashes{
		configs: cfg,
	}
}

func NewStageBlockHashesCfg(bc core.BlockChain, db kv.RwDB, concurrency int, protocol syncProtocol, isBeaconShard bool, logProgress bool) StageBlockHashesCfg {
	return StageBlockHashesCfg{
		bc:            bc,
		db:            db,
		isBeaconShard: isBeaconShard,
		logProgress:   logProgress,
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
	hdm := newDownloadManager(bh.configs.bc, targetHeight, BlockHashesPerRequest, s.state.logger)

	// Fetch block hashes from neighbors
	if err := bh.runBlockHashWorkerLoop(ctx, tx, hdm, s, startTime, currentHead, targetHeight); err != nil {
		return nil
	}

	return nil
}

func (bh *StageBlockHashes) downloadBlockHashes(ctx context.Context, bns []uint64) ([]common.Hash, sttypes.StreamID, error) {
	hashes, stid, err := bh.configs.protocol.GetBlockHashes(ctx, bns)
	if hashes == nil {
		utils.Logger().Warn().
			Interface("bns", bns).
			Msg("[StageBlockHashes] downloadBlockHashes Nil Response")
		bh.configs.protocol.RemoveStream(stid)
		return []common.Hash{}, stid, errors.New("nil response for hashes")
	}
	utils.Logger().Info().
		Int("request size", len(bns)).
		Int("received size", len(hashes)).
		Interface("stid", stid).
		Msg("[StageBlockHashes] downloadBlockHashes received hashes")
	if len(hashes) > len(bns) {
		utils.Logger().Warn().
			Int("request size", len(bns)).
			Int("received size", len(hashes)).
			Interface("stid", stid).
			Msg("[StageBlockHashes] received more blockHashes than requested!")
		return hashes[:len(bns)], stid, nil
	}
	return hashes, stid, err
}

// runBlockhashWorkerLoop downloads block hashes
func (bh *StageBlockHashes) runBlockHashWorkerLoop(ctx context.Context,
	tx kv.RwTx,
	hdm *downloadManager,
	s *StageState,
	startTime time.Time,
	startHeight uint64,
	targetHeight uint64) error {

	currProgress := startHeight
	var wg sync.WaitGroup

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
		batch := hdm.GetNextBatch(currProgress)
		if len(batch) == 0 {
			break
		}

		// Map to store block hashes fetched from peers
		peerHashes := sttypes.NewSafeMap[sttypes.StreamID, []common.Hash]()

		// Fetch block hashes concurrently
		for i := 0; i < bh.configs.concurrency; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				// Download block hashes
				hashes, stid, err := bh.downloadBlockHashes(ctx, batch)
				if err != nil {
					return
				}

				// Store the result in peerHashes
				peerHashes.Set(stid, hashes)
			}()
		}

		// Wait for all workers to complete
		wg.Wait()

		// Calculate the final block hashes for the batch
		finalBlockHashes, _, errFinalHashCaclulations := bh.calculateFinalBlockHashes(peerHashes, batch)
		if errFinalHashCaclulations != nil {
			panic(ErrFinalBlockHashesCalculationFailed)
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
		if err := s.Update(tx, currProgress); err != nil {
			utils.Logger().Error().
				Err(err).
				Msgf("[STAGED_STREAM_SYNC] saving progress for block hashes stage failed")
			return err
		}

		// log the stage progress in console
		if bh.configs.logProgress {
			//calculating block hash speed
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

// calculateFinalBlockHashes Calculates the most frequent block hashes for a given batch and removes streams with invalid hashes.
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

			// Update the final hash for the block if this hash has higher frequency
			if details.Count > len(hashFrequency[blockNumber][finalHashes[blockNumber]].StreamIDs) {
				finalHashes[blockNumber] = hash
			}
		}
	})

	// Identify invalid streams
	peerHashes.Iterate(func(stid sttypes.StreamID, hashes []common.Hash) {
		for i, blockNumber := range batch {
			if i >= len(hashes) || finalHashes[blockNumber] != hashes[i] {
				invalidStreams[stid] = struct{}{}
				bh.configs.protocol.RemoveStream(stid)
				break
			}
		}
	})

	return finalHashes, invalidStreams, nil
}

// saveBlocks saves the blocks into db
func (b *StageBlockHashes) saveBlockHashes(ctx context.Context, tx kv.RwTx, hashes map[uint64]common.Hash) error {

	if tx == nil {
		var err error
		tx, err = b.configs.db.BeginRw(ctx)
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
			utils.Logger().Error().
				Err(err).
				Uint64("block number", bn).
				Msg("[STAGED_STREAM_SYNC] adding block hash to db failed")
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
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
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] clear block hashes bucket after revert failed")
		return err
	}

	// clean cache db as well
	if err := bh.clearCache(); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] clear block hashes cache failed")
		return err
	}

	// save progress
	currentHead := bh.configs.bc.CurrentBlock().NumberU64()
	if err = s.Update(tx, currentHead); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] saving progress for block hashes stage after revert failed")
		return err
	}

	if err = u.Done(tx); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] reset after revert failed")
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
