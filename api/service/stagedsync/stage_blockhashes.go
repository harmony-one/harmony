package stagedsync

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/log/v3"
)

type StageBlockHashes struct {
	configs StageBlockHashesCfg
}

type StageBlockHashesCfg struct {
	ctx           context.Context
	bc            core.BlockChain
	db            kv.RwDB
	turbo         bool
	turboModeCh   chan struct{}
	bgProcRunning bool
	isBeacon      bool
	cachedb       kv.RwDB
}

func NewStageBlockHashes(cfg StageBlockHashesCfg) *StageBlockHashes {
	return &StageBlockHashes{
		configs: cfg,
	}
}

func NewStageBlockHashesCfg(ctx context.Context, bc core.BlockChain, db kv.RwDB, isBeacon bool, turbo bool) StageBlockHashesCfg {
	cachedb, err := initHashesCacheDB(ctx, isBeacon)
	if err != nil {
		panic("can't initialize sync caches")
	}
	return StageBlockHashesCfg{
		ctx:      ctx,
		bc:       bc,
		db:       db,
		turbo:    turbo,
		isBeacon: isBeacon,
		cachedb:  cachedb,
	}
}

func initHashesCacheDB(ctx context.Context, isBeacon bool) (db kv.RwDB, err error) {
	// create caches db
	cachedbName := Block_Hashes_Cache_DB
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
	if err := tx.CreateBucket(BlockHashesBucket); err != nil {
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

func (bh *StageBlockHashes) Exec(firstCycle bool, invalidBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) (err error) {

	currProgress := uint64(0)
	targetHeight := uint64(0)
	isBeacon := s.state.isBeacon
	startHash := bh.configs.bc.CurrentBlock().Hash()
	canRunInTurboMode := bh.configs.turbo

	// retrieve the progress
	if errV := CreateView(bh.configs.ctx, bh.configs.db, tx, func(etx kv.Tx) error {
		if targetHeight, err = GetStageProgress(etx, Heads, isBeacon); err != nil {
			return err
		}
		if currProgress, err = s.CurrentStageProgress(etx); err != nil { //GetStageProgress(etx, BlockHashes, isBeacon); err != nil {
			return err
		}
		if currProgress > 0 {
			key := strconv.FormatUint(currProgress, 10)
			bucketName := GetBucketName(BlockHashesBucket, isBeacon)
			currHash := []byte{}
			if currHash, err = etx.GetOne(bucketName, []byte(key)); err != nil || len(currHash[:]) == 0 {
				//TODO: currProgress and DB don't match. Either re-download all or verify db and set currProgress to last
				return err
			}
			startHash.SetBytes(currHash[:])
		}
		return nil
	}); errV != nil {
		return errV
	}

	if currProgress == 0 {
		if err := bh.clearBlockHashesBucket(tx, s.state.isBeacon); err != nil {
			return err
		}
		startHash = bh.configs.bc.CurrentBlock().Hash()
		currProgress = bh.configs.bc.CurrentBlock().NumberU64()
	}

	if currProgress >= targetHeight {
		if canRunInTurboMode && currProgress < s.state.syncStatus.maxPeersHeight {
			bh.configs.turboModeCh = make(chan struct{})
			go bh.runBackgroundProcess(nil, s, isBeacon, currProgress, s.state.syncStatus.maxPeersHeight, startHash)
		}
		return nil
	}

	// check whether any block hashes after curr height is cached
	if canRunInTurboMode {
		var cacheHash []byte
		if cacheHash, err = bh.getHashFromCache(currProgress + 1); err != nil {
			utils.Logger().Info().
				Msgf("[STAGED_SYNC] fetch cache progress for block hashes stage failed: %v", err)
			return ErrFetchBlockHashProgressFail
		} else {
			if len(cacheHash[:]) > 0 {
				// get blocks from cached db rather than calling peers, and update current progress
				currProgress, startHash, err = bh.loadBlockHashesFromCache(s, currProgress, targetHeight, tx)
				if err != nil {
					utils.Logger().Info().
						Msgf("[STAGED_SYNC] fetch cached block hashes failed: %v", err)
					return ErrFetchCachedBlockHashFail
				}
			}
		}
	}

	if currProgress >= targetHeight {
		if canRunInTurboMode && currProgress < s.state.syncStatus.maxPeersHeight {
			bh.configs.turboModeCh = make(chan struct{})
			go bh.runBackgroundProcess(nil, s, isBeacon, currProgress, s.state.syncStatus.maxPeersHeight, startHash)
		}
		return nil
	}

	size := uint32(0)

	fmt.Print("\033[s") // save the cursor position
	startTime := time.Now()
	startBlock := currProgress
	for ok := true; ok; ok = currProgress < targetHeight {
		size = uint32(targetHeight - currProgress)
		if size > SyncLoopBatchSize {
			size = SyncLoopBatchSize
		}
		// Gets consensus hashes.
		if err := s.state.getConsensusHashes(startHash[:], size); err != nil {
			return err
		}
		// selects the most common peer config based on their block hashes and doing the clean up
		if err := s.state.syncConfig.GetBlockHashesConsensusAndCleanUp(); err != nil {
			return err
		}
		// double check block hashes
		if s.state.DoubleCheckBlockHashes {
			invalidPeersMap, validBlockHashes, err := s.state.getInvalidPeersByBlockHashes(tx)
			if err != nil {
				return err
			}
			if validBlockHashes < int(size) {
				return ErrNotEnoughBlockHashes
			}
			s.state.syncConfig.cleanUpInvalidPeers(invalidPeersMap)
		}
		// save the downloaded files to db
		if currProgress, startHash, err = bh.saveDownloadedBlockHashes(s, currProgress, startHash, tx); err != nil {
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
		fmt.Println("downloading block hash progress:", currProgress, "/", targetHeight, "(", blockSpeed, "blocks/s", ")")
	}

	// continue downloading in background
	if canRunInTurboMode && currProgress < s.state.syncStatus.maxPeersHeight {
		bh.configs.turboModeCh = make(chan struct{})
		go bh.runBackgroundProcess(nil, s, isBeacon, currProgress, s.state.syncStatus.maxPeersHeight, startHash)
	}
	return nil
}

func (bh *StageBlockHashes) runBackgroundProcess(tx kv.RwTx, s *StageState, isBeacon bool, startHeight uint64, targetHeight uint64, startHash common.Hash) error {
	size := uint32(0)
	currProgress := startHeight
	currHash := startHash
	bh.configs.bgProcRunning = true

	// retrieve bg progress and last hash
	errV := bh.configs.cachedb.View(context.Background(), func(rtx kv.Tx) error {

		if progressBytes, err := rtx.GetOne(StageProgressBucket, []byte(LastBlockHeight)); err != nil {
			utils.Logger().Info().
				Msgf("[STAGED_SYNC] retrieving cache progress for block hashes stage failed: %v", err)
			return ErrRetrieveCachedProgressFail
		} else {
			if len(progressBytes[:]) > 0 {
				savedProgress, _ := unmarshalData(progressBytes)
				if savedProgress > startHeight {
					currProgress = savedProgress
					// retrieve start hash
					if lastBlockHash, err := rtx.GetOne(StageProgressBucket, []byte(LastBlockHash)); err != nil {
						utils.Logger().Info().
							Msgf("[STAGED_SYNC] retrieving cache progress for block hashes stage failed: %v", err)
						return ErrRetrieveCachedHashProgressFail
					} else {
						currHash.SetBytes(lastBlockHash[:])
					}
				}
			}
		}
		return nil

	})
	if errV != nil {
		return errV
	}

	for {
		select {
		case <-bh.configs.turboModeCh:
			close(bh.configs.turboModeCh)
			bh.configs.bgProcRunning = false
			return nil
		default:
			if currProgress >= targetHeight {
				close(bh.configs.turboModeCh)
				bh.configs.bgProcRunning = false
				return nil
			}

			size = uint32(targetHeight - currProgress)
			if size > SyncLoopBatchSize {
				size = SyncLoopBatchSize
			}

			// Gets consensus hashes.
			if err := s.state.getConsensusHashes(currHash[:], size); err != nil {
				return err
			}

			// selects the most common peer config based on their block hashes and doing the clean up
			if err := s.state.syncConfig.GetBlockHashesConsensusAndCleanUp(); err != nil {
				return err
			}

			// save the downloaded files to db
			var err error
			if currProgress, currHash, err = bh.saveBlockHashesInCacheDB(s, currProgress, currHash); err != nil {
				return err
			}
		}
		//time.Sleep(1 * time.Millisecond)
	}
	// return nil
}

func (bh *StageBlockHashes) clearBlockHashesBucket(tx kv.RwTx, isBeacon bool) error {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = bh.configs.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}
	bucketName := GetBucketName(BlockHashesBucket, isBeacon)
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

// saves block hashes to db (map from block heigh to block hash)
func (bh *StageBlockHashes) saveDownloadedBlockHashes(s *StageState, progress uint64, startHash common.Hash, tx kv.RwTx) (p uint64, h common.Hash, err error) {
	p = progress
	h.SetBytes(startHash.Bytes())
	lastAddedID := int(0) // the first id won't be added
	saved := false

	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = bh.configs.db.BeginRw(context.Background())
		if err != nil {
			return p, h, err
		}
		defer tx.Rollback()
	}

	s.state.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		if len(configPeer.blockHashes) == 0 {
			saved = true
			brk = true
		}

		for id, blockHash := range configPeer.blockHashes {
			if id <= lastAddedID {
				continue
			}
			key := strconv.FormatUint(p+1, 10)
			bucketName := GetBucketName(BlockHashesBucket, s.state.isBeacon)
			if err := tx.Put(bucketName, []byte(key), blockHash); err != nil {
				utils.Logger().Warn().
					Err(err).
					Int("block hash index", id).
					Str("block hash", hex.EncodeToString(blockHash)).
					Msg("[STAGED_SYNC] adding block hash to db failed")
				return
			}
			p++
			h.SetBytes(blockHash[:])
			lastAddedID = id
		}

		// check if all block hashes are added to db break the loop
		if lastAddedID == len(configPeer.blockHashes)-1 {
			saved = true
			brk = true
		}
		return
	})

	// save progress
	if err = s.Update(tx, p); err != nil {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving progress for block hashes stage failed: %v", err)
		return progress, startHash, ErrSaveBlockHashesProgressFail
	}

	if len(s.state.syncConfig.peers) > 0 && len(s.state.syncConfig.peers[0].blockHashes) > 0 && !saved {
		return progress, startHash, ErrSaveBlockHashesProgressFail
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return progress, startHash, err
		}
	}
	return p, h, nil
}

// save block hashes to cache db (map from block heigh to block hash)
func (bh *StageBlockHashes) saveBlockHashesInCacheDB(s *StageState, progress uint64, startHash common.Hash) (p uint64, h common.Hash, err error) {
	p = progress
	h.SetBytes(startHash[:])
	lastAddedID := int(0) // the first id won't be added
	saved := false

	etx, err := bh.configs.cachedb.BeginRw(context.Background())
	if err != nil {
		return p, h, err
	}
	defer etx.Rollback()

	s.state.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		for id, blockHash := range configPeer.blockHashes {
			if id <= lastAddedID {
				continue
			}
			key := strconv.FormatUint(p+1, 10)
			if err := etx.Put(BlockHashesBucket, []byte(key), blockHash); err != nil {
				utils.Logger().Warn().
					Err(err).
					Int("block hash index", id).
					Str("block hash", hex.EncodeToString(blockHash)).
					Msg("[STAGED_SYNC] adding block hash to db failed")
				return
			}
			p++
			h.SetBytes(blockHash[:])
			lastAddedID = id
		}

		// check if all block hashes are added to db break the loop
		if lastAddedID == len(configPeer.blockHashes)-1 {
			saved = true
			brk = true
		}
		return
	})

	// save cache progress (last block height)
	if err = etx.Put(StageProgressBucket, []byte(LastBlockHeight), marshalData(p)); err != nil {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving cache progress for block hashes stage failed: %v", err)
		return p, h, ErrSaveCachedBlockHashesProgressFail
	}

	// save cache progress
	if err = etx.Put(StageProgressBucket, []byte(LastBlockHash), h.Bytes()); err != nil {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving cache last block hash for block hashes stage failed: %v", err)
		return p, h, ErrSavingCacheLastBlockHashFail
	}

	if len(s.state.syncConfig.peers) > 0 && len(s.state.syncConfig.peers[0].blockHashes) > 0 && !saved {
		return p, h, ErrCachingBlockHashFail
	}

	if err := etx.Commit(); err != nil {
		return p, h, err
	}
	return p, h, nil
}

// save block hashes to cache db (map from block heigh to block hash)
func (bh *StageBlockHashes) getHashFromCache(height uint64) (h []byte, err error) {

	tx, err := bh.configs.cachedb.BeginRw(context.Background())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var cacheHash []byte
	key := strconv.FormatUint(height, 10)
	if cacheHash, err = tx.GetOne(BlockHashesBucket, []byte(key)); err != nil {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] fetch cache progress for block hashes stage failed: %v", err)
		return nil, ErrFetchBlockHashProgressFail
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return cacheHash[:], nil
}

// load block hashes from cache db to main sync db and update the progress
func (bh *StageBlockHashes) loadBlockHashesFromCache(s *StageState, startHeight uint64, targetHeight uint64, tx kv.RwTx) (p uint64, h common.Hash, err error) {

	p = startHeight
	var lastHash []byte
	h.SetBytes(lastHash[:])
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = bh.configs.db.BeginRw(bh.configs.ctx)
		if err != nil {
			return p, h, err
		}
		defer tx.Rollback()
	}

	if errV := bh.configs.cachedb.View(context.Background(), func(rtx kv.Tx) error {
		// load block hashes from cache db and copy them to main sync db
		for ok := true; ok; ok = p < targetHeight {
			key := strconv.FormatUint(p+1, 10)
			lastHash, err = rtx.GetOne(BlockHashesBucket, []byte(key))
			if err != nil {
				utils.Logger().Warn().
					Err(err).
					Str("block height", key).
					Msg("[STAGED_SYNC] retrieve block hash from cache failed")
				return err
			}
			if len(lastHash[:]) == 0 {
				return nil
			}
			bucketName := GetBucketName(BlockHashesBucket, s.state.isBeacon)
			if err = tx.Put(bucketName, []byte(key), lastHash); err != nil {
				return err
			}
			p++
		}
		// load extra block hashes from cache db and copy them to bg db to be downloaded in background by block stage
		pExtraHashes := p
		for ok := true; ok; ok = pExtraHashes < p+s.state.MaxBackgroundBlocks {
			key := strconv.FormatUint(pExtraHashes+1, 10)
			newHash, err := rtx.GetOne(BlockHashesBucket, []byte(key))
			if err != nil {
				utils.Logger().Warn().
					Err(err).
					Str("block height", key).
					Msg("[STAGED_SYNC] retrieve extra block hashes for background process failed")
				break
			}
			if len(newHash[:]) == 0 {
				return nil
			}
			bucketName := GetBucketName(ExtraBlockHashesBucket, s.state.isBeacon)
			if err = tx.Put(bucketName, []byte(key), newHash); err != nil {
				break
			}
			pExtraHashes++
		}
		return nil
	}); errV != nil {
		return startHeight, h, errV
	}

	// set last hash
	h.SetBytes(lastHash[:])

	// save progress
	if err = s.Update(tx, p); err != nil {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving retrieved cached progress for block hashes stage failed: %v", err)
		return startHeight, h, err
	}

	// update the progress
	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return startHeight, h, err
		}
	}
	return p, h, nil
}

func (bh *StageBlockHashes) Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = bh.configs.db.BeginRw(bh.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if err = u.Done(tx); err != nil {
		return fmt.Errorf(" reset: %w", err)
	}

	if useInternalTx {
		if err = tx.Commit(); err != nil {
			return ErrCommitTransactionFail
		}
	}
	return nil
}

func (bh *StageBlockHashes) Prune(firstCycle bool, p *PruneState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = bh.configs.db.BeginRw(bh.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// terminate background process in turbo mode
	if bh.configs.turboModeCh != nil && bh.configs.bgProcRunning {
		bh.configs.turboModeCh <- struct{}{}
	}

	hashesBucketName := GetBucketName(BlockHashesBucket, bh.configs.isBeacon)
	tx.ClearBucket(hashesBucketName)

	extrahashesBucketName := GetBucketName(ExtraBlockHashesBucket, bh.configs.isBeacon)
	tx.ClearBucket(extrahashesBucketName)

	if useInternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
