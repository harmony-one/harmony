package stagedstreamsync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/harmony-one/harmony/shard"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
)

type StageBodies struct {
	configs StageBodiesCfg
}

type StageBodiesCfg struct {
	bc          core.BlockChain
	db          kv.RwDB
	blockDBs    []kv.RwDB
	concurrency int
	protocol    syncProtocol
	isBeacon    bool
	logProgress bool
}

func NewStageBodies(cfg StageBodiesCfg) *StageBodies {
	return &StageBodies{
		configs: cfg,
	}
}

func NewStageBodiesCfg(bc core.BlockChain, db kv.RwDB, blockDBs []kv.RwDB, concurrency int, protocol syncProtocol, isBeacon bool, logProgress bool) StageBodiesCfg {
	return StageBodiesCfg{
		bc:          bc,
		db:          db,
		blockDBs:    blockDBs,
		concurrency: concurrency,
		protocol:    protocol,
		isBeacon:    isBeacon,
		logProgress: logProgress,
	}
}

// Exec progresses Bodies stage in the forward direction
func (b *StageBodies) Exec(ctx context.Context, firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) (err error) {

	useInternalTx := tx == nil

	if invalidBlockRevert {
		return b.redownloadBadBlock(ctx, s)
	}

	// for short range sync, skip this stage
	if !s.state.initSync {
		return nil
	}

	// shouldn't execute for epoch chain
	if b.configs.bc.ShardID() == shard.BeaconChainShardID && !s.state.isBeaconNode {
		return nil
	}

	maxHeight := s.state.status.targetBN
	currentHead := b.configs.bc.CurrentBlock().NumberU64()
	if currentHead >= maxHeight {
		return nil
	}
	currProgress := uint64(0)
	targetHeight := s.state.currentCycle.TargetHeight

	if errV := CreateView(ctx, b.configs.db, tx, func(etx kv.Tx) error {
		if currProgress, err = s.CurrentStageProgress(etx); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	if currProgress <= currentHead {
		if err := b.cleanAllBlockDBs(ctx); err != nil {
			return err
		}
		currProgress = currentHead
	}

	if currProgress >= targetHeight {
		return nil
	}

	// size := uint64(0)
	startTime := time.Now()
	// startBlock := currProgress
	if b.configs.logProgress {
		fmt.Print("\033[s") // save the cursor position
	}

	if useInternalTx {
		var err error
		tx, err = b.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// Fetch blocks from neighbors
	s.state.gbm = newBlockDownloadManager(tx, b.configs.bc, targetHeight, s.state.logger)

	// Setup workers to fetch blocks from remote node
	var wg sync.WaitGroup

	for i := 0; i != s.state.config.Concurrency; i++ {
		wg.Add(1)
		go b.runBlockWorkerLoop(ctx, s.state.gbm, &wg, i, startTime)
	}

	wg.Wait()

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

// runBlockWorkerLoop creates a work loop for download blocks
func (b *StageBodies) runBlockWorkerLoop(ctx context.Context, gbm *blockDownloadManager, wg *sync.WaitGroup, loopID int, startTime time.Time) {

	currentBlock := int(b.configs.bc.CurrentBlock().NumberU64())

	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		batch := gbm.GetNextBatch()
		if len(batch) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				return
			}
		}

		blockBytes, sigBytes, stid, err := b.downloadRawBlocks(ctx, batch)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				b.configs.protocol.StreamFailed(stid, "downloadRawBlocks failed")
			}
			utils.Logger().Error().
				Err(err).
				Str("stream", string(stid)).
				Interface("block numbers", batch).
				Msg(WrapStagedSyncMsg("downloadRawBlocks failed"))
			err = errors.Wrap(err, "request error")
			gbm.HandleRequestError(batch, err, stid)
		} else if blockBytes == nil || len(blockBytes) == 0 {
			utils.Logger().Warn().
				Str("stream", string(stid)).
				Interface("block numbers", batch).
				Msg(WrapStagedSyncMsg("downloadRawBlocks failed, received empty blockBytes"))
			err := errors.New("downloadRawBlocks received empty blockBytes")
			gbm.HandleRequestError(batch, err, stid)
		} else {
			if err = b.saveBlocks(ctx, gbm.tx, batch, blockBytes, sigBytes, loopID, stid); err != nil {
				panic(ErrSaveBlocksToDbFailed)
			}
			gbm.HandleRequestResult(batch, blockBytes, sigBytes, loopID, stid)
			if b.configs.logProgress {
				//calculating block download speed
				dt := time.Now().Sub(startTime).Seconds()
				speed := float64(0)
				if dt > 0 {
					speed = float64(len(gbm.bdd)) / dt
				}
				blockSpeed := fmt.Sprintf("%.2f", speed)

				fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
				fmt.Println("downloaded blocks:", currentBlock+len(gbm.bdd), "/", int(gbm.targetBN), "(", blockSpeed, "blocks/s", ")")
			}
		}
	}
}

// redownloadBadBlock tries to redownload the bad block from other streams
func (b *StageBodies) redownloadBadBlock(ctx context.Context, s *StageState) error {

	batch := make([]uint64, 1)
	batch = append(batch, s.state.invalidBlock.Number)

	for {
		if b.configs.protocol.NumStreams() == 0 {
			return errors.Errorf("re-download bad block from all streams failed")
		}
		blockBytes, sigBytes, stid, err := b.downloadRawBlocks(ctx, batch)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				b.configs.protocol.StreamFailed(stid, "tried to re-download bad block from this stream, but downloadRawBlocks failed")
			}
			continue
		}
		isOneOfTheBadStreams := false
		for _, id := range s.state.invalidBlock.StreamID {
			if id == stid {
				b.configs.protocol.StreamFailed(stid, "re-download bad block from this stream failed")
				isOneOfTheBadStreams = true
				break
			}
		}
		if isOneOfTheBadStreams {
			continue
		}
		s.state.gbm.SetDownloadDetails(batch, 0, stid)
		if errU := b.configs.blockDBs[0].Update(ctx, func(tx kv.RwTx) error {
			if err = b.saveBlocks(ctx, tx, batch, blockBytes, sigBytes, 0, stid); err != nil {
				return errors.Errorf("[STAGED_STREAM_SYNC] saving re-downloaded bad block to db failed.")
			}
			return nil
		}); errU != nil {
			continue
		}
		break
	}
	return nil
}

func (b *StageBodies) downloadBlocks(ctx context.Context, bns []uint64) ([]*types.Block, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	blocks, stid, err := b.configs.protocol.GetBlocksByNumber(ctx, bns)
	if err != nil {
		return nil, stid, err
	}
	if err := validateGetBlocksResult(bns, blocks); err != nil {
		return nil, stid, err
	}
	return blocks, stid, nil
}

func (b *StageBodies) downloadRawBlocks(ctx context.Context, bns []uint64) ([][]byte, [][]byte, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return b.configs.protocol.GetRawBlocksByNumber(ctx, bns)
}

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

// saveBlocks saves the blocks into db
func (b *StageBodies) saveBlocks(ctx context.Context, tx kv.RwTx, bns []uint64, blockBytes [][]byte, sigBytes [][]byte, loopID int, stid sttypes.StreamID) error {

	tx, err := b.configs.blockDBs[loopID].BeginRw(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i := uint64(0); i < uint64(len(blockBytes)); i++ {
		block := blockBytes[i]
		sig := sigBytes[i]
		if block == nil {
			continue
		}

		blkKey := marshalData(bns[i])

		if err := tx.Put(BlocksBucket, blkKey, block); err != nil {
			utils.Logger().Error().
				Err(err).
				Uint64("block height", bns[i]).
				Msg("[STAGED_STREAM_SYNC] adding block to db failed")
			return err
		}
		// sigKey := []byte("s" + string(bns[i]))
		if err := tx.Put(BlockSignaturesBucket, blkKey, sig); err != nil {
			utils.Logger().Error().
				Err(err).
				Uint64("block height", bns[i]).
				Msg("[STAGED_STREAM_SYNC] adding block sig to db failed")
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
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

func (b *StageBodies) cleanBlocksDB(ctx context.Context, loopID int) (err error) {
	tx, errb := b.configs.blockDBs[loopID].BeginRw(ctx)
	if errb != nil {
		return errb
	}
	defer tx.Rollback()

	// clean block bodies db
	if err = tx.ClearBucket(BlocksBucket); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_STREAM_SYNC] clear blocks bucket after revert failed")
		return err
	}
	// clean block signatures db
	if err = tx.ClearBucket(BlockSignaturesBucket); err != nil {
		utils.Logger().Error().
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
		return err
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

func (b *StageBodies) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	//clean all blocks DBs
	if err := b.cleanAllBlockDBs(ctx); err != nil {
		return err
	}

	return nil
}
