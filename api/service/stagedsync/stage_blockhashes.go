package stagedsync

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"

	//"github.com/harmony-one/harmony/internal/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
	//"github.com/pkg/errors"
)

type StageBlockHashes struct {
	configs StageBlockHashesCfg
}

type StageBlockHashesCfg struct {
	mtx sync.Mutex
	ctx context.Context
	db  kv.RwDB
}

func NewStageBlockHashes(cfg StageBlockHashesCfg) *StageBlockHashes {
	return &StageBlockHashes{
		configs: cfg,
	}
}

func NewStageBlockHashesCfg(ctx context.Context, db kv.RwDB) StageBlockHashesCfg {
	return StageBlockHashesCfg{
		mtx: sync.Mutex{},
		ctx: ctx,
		db:  db,
	}
}

func (bh *StageBlockHashes) Exec(firstCycle bool, badBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) (err error) {

	currProgress := uint64(0)
	currHash := []byte{}
	targetHeight := uint64(0)
	isBeacon := s.state.isBeacon

	if errV := bh.configs.db.View(bh.configs.ctx, func(etx kv.Tx) error {
		if targetHeight, err = GetStageProgress(etx, Headers, isBeacon); err != nil {
			return err
		}
		if currProgress, err = GetStageProgress(etx, BlockHashes, isBeacon); err != nil {
			return err
		}
		key := strconv.FormatUint(currProgress, 10)
		if currHash, err = etx.GetOne(BlockHashesBucket, []byte(key)); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	startHash := s.state.Blockchain().CurrentBlock().Hash()
	if currProgress > 0 {
		startHash.SetBytes(currHash[:])
	} else {
		currProgress = s.state.Blockchain().CurrentBlock().NumberU64()
	}

	if currProgress >= targetHeight {
		return nil
	}

	size := uint32(0)

	fmt.Print("\033[s") // save the cursor position

	for ok := true; ok; ok = currProgress <= targetHeight {

		size = uint32(targetHeight - currProgress)
		if size > SyncLoopBatchSize {
			size = SyncLoopBatchSize
		}
		// Gets consensus hashes.
		if err := s.state.getConsensusHashes(startHash[:], size, tx); err != nil {
			return errors.Wrap(err, "getConsensusHashes")
		}
		// download block hashes
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
				return errors.Wrap(err, "getBlockHashes: peers haven't sent all requested block hashes")
			}
			s.state.syncConfig.cleanUpInvalidPeers(invalidPeersMap)
		}
		// save the downloaded files to db
		if currProgress, startHash, err = bh.saveDownloadedBlockHashes(s, currProgress, startHash, tx); err != nil {
			return err
		}
		// clean up cache
		s.state.purgeAllBlocksFromCache()

		fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
		fmt.Println("downloading block hash progress:", currProgress, "/", targetHeight)
	}

	return nil
}

func (bh *StageBlockHashes) saveDownloadedBlockHashes(s *StageState, progress uint64, startHash common.Hash, tx kv.RwTx) (p uint64, h common.Hash, err error) {
	p = progress
	h.SetBytes([]byte{})
	// save block hashes to db (map from block heigh to block hash)
	lastAddedID := int(0) // the first id won't be added
	saved := false

	useExternalTx := tx != nil
	if !useExternalTx {
		var err error
		tx, err = bh.configs.db.BeginRw(context.Background())
		if err != nil {
			return p, h, err
		}
		defer tx.Rollback()
	}

	s.state.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		for id, blockHash := range configPeer.blockHashes {
			if id <= lastAddedID {
				continue
			}
			key := strconv.FormatUint(p+1, 10)
			if err := tx.Put(BlockHashesBucket, []byte(key), blockHash); err != nil {
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
		return p, h, fmt.Errorf("saving progress for block hashes stage failed")
	}

	if !saved {
		return p, h, fmt.Errorf("save downloaded block hashes failed")
	}

	if !useExternalTx {
		if err := tx.Commit(); err != nil {
			return p, h, err
		}
	}
	return p, h, nil
}

func (bh *StageBlockHashes) Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = bh.configs.db.BeginRw(bh.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if err = u.Done(tx); err != nil {
		return fmt.Errorf(" reset: %w", err)
	}
	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("failed to write db commit: %w", err)
		}
	}
	return nil
}

func (bh *StageBlockHashes) Prune(firstCycle bool, p *PruneState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = bh.configs.db.BeginRw(bh.configs.ctx)
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
