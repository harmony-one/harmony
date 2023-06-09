package stagedstreamsync

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
)

type StageStates struct {
	configs StageStatesCfg
}
type StageStatesCfg struct {
	bc          core.BlockChain
	db          kv.RwDB
	blockDBs    []kv.RwDB
	concurrency int
	logger      zerolog.Logger
	logProgress bool
}

func NewStageStates(cfg StageStatesCfg) *StageStates {
	return &StageStates{
		configs: cfg,
	}
}

func NewStageStatesCfg(
	bc core.BlockChain,
	db kv.RwDB,
	blockDBs []kv.RwDB,
	concurrency int,
	logger zerolog.Logger,
	logProgress bool) StageStatesCfg {

	return StageStatesCfg{
		bc:          bc,
		db:          db,
		blockDBs:    blockDBs,
		concurrency: concurrency,
		logger:      logger,
		logProgress: logProgress,
	}
}

// Exec progresses States stage in the forward direction
func (stg *StageStates) Exec(ctx context.Context, firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) (err error) {
	// for short range sync, skip this step
	if !s.state.initSync {
		return nil
	}

	maxHeight := s.state.status.targetBN
	currentHead := stg.configs.bc.CurrentBlock().NumberU64()
	if currentHead >= maxHeight {
		return nil
	}
	currProgress := stg.configs.bc.CurrentBlock().NumberU64()
	targetHeight := s.state.currentCycle.TargetHeight
	if currProgress >= targetHeight {
		return nil
	}
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = stg.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// isLastCycle := targetHeight >= maxHeight
	startTime := time.Now()
	startBlock := currProgress
	pl := s.state.promLabels()
	gbm := s.state.gbm

	// prepare db transactions
	txs := make([]kv.RwTx, stg.configs.concurrency)
	for i := 0; i < stg.configs.concurrency; i++ {
		txs[i], err = stg.configs.blockDBs[i].BeginRw(ctx)
		if err != nil {
			return err
		}
	}

	defer func() {
		for i := 0; i < stg.configs.concurrency; i++ {
			txs[i].Rollback()
		}
	}()

	if stg.configs.logProgress {
		fmt.Print("\033[s") // save the cursor position
	}

	for i := currProgress + 1; i <= targetHeight; i++ {
		blkKey := marshalData(i)
		loopID, streamID := gbm.GetDownloadDetails(i)

		blockBytes, err := txs[loopID].GetOne(BlocksBucket, blkKey)
		if err != nil {
			return err
		}
		sigBytes, err := txs[loopID].GetOne(BlockSignaturesBucket, blkKey)
		if err != nil {
			return err
		}

		// if block size is invalid, we have to break the updating state loop
		// we don't need to do rollback, because the latest batch haven't added to chain yet
		sz := len(blockBytes)
		if sz <= 1 {
			utils.Logger().Error().
				Uint64("block number", i).
				Msg("block size invalid")
			invalidBlockHash := common.Hash{}
			s.state.protocol.StreamFailed(streamID, "zero bytes block is received from stream")
			reverter.RevertTo(stg.configs.bc.CurrentBlock().NumberU64(), i, invalidBlockHash, streamID)
			return ErrInvalidBlockBytes
		}

		var block *types.Block
		if err := rlp.DecodeBytes(blockBytes, &block); err != nil {
			utils.Logger().Error().
				Uint64("block number", i).
				Msg("block size invalid")
			s.state.protocol.StreamFailed(streamID, "invalid block is received from stream")
			invalidBlockHash := common.Hash{}
			reverter.RevertTo(stg.configs.bc.CurrentBlock().NumberU64(), i, invalidBlockHash, streamID)
			return ErrInvalidBlockBytes
		}
		if sigBytes != nil {
			block.SetCurrentCommitSig(sigBytes)
		}

		if block.NumberU64() != i {
			s.state.protocol.StreamFailed(streamID, "invalid block with unmatched number is received from stream")
			invalidBlockHash := block.Hash()
			reverter.RevertTo(stg.configs.bc.CurrentBlock().NumberU64(), i, invalidBlockHash, streamID)
			return ErrInvalidBlockNumber
		}

		if err := verifyAndInsertBlock(stg.configs.bc, block); err != nil {
			stg.configs.logger.Warn().Err(err).Uint64("cycle target block", targetHeight).
				Uint64("block number", block.NumberU64()).
				Msg(WrapStagedSyncMsg("insert blocks failed in long range"))
			s.state.protocol.StreamFailed(streamID, "unverifiable invalid block is received from stream")
			invalidBlockHash := block.Hash()
			reverter.RevertTo(stg.configs.bc.CurrentBlock().NumberU64(), block.NumberU64(), invalidBlockHash, streamID)
			pl["error"] = err.Error()
			longRangeFailInsertedBlockCounterVec.With(pl).Inc()
			return err
		}

		if invalidBlockRevert {
			if s.state.invalidBlock.Number == i {
				s.state.invalidBlock.resolve()
			}
		}

		s.state.inserted++
		longRangeSyncedBlockCounterVec.With(pl).Inc()

		utils.Logger().Info().
			Uint64("blockHeight", block.NumberU64()).
			Uint64("blockEpoch", block.Epoch().Uint64()).
			Str("blockHex", block.Hash().Hex()).
			Uint32("ShardID", block.ShardID()).
			Msg("[STAGED_STREAM_SYNC] New Block Added to Blockchain")

		// update cur progress
		currProgress = stg.configs.bc.CurrentBlock().NumberU64()

		for i, tx := range block.StakingTransactions() {
			utils.Logger().Info().
				Msgf(
					"StakingTxn %d: %s, %v", i, tx.StakingType().String(), tx.StakingMessage(),
				)
		}

		// log the stage progress in console
		if stg.configs.logProgress {
			//calculating block speed
			dt := time.Now().Sub(startTime).Seconds()
			speed := float64(0)
			if dt > 0 {
				speed = float64(currProgress-startBlock) / dt
			}
			blockSpeed := fmt.Sprintf("%.2f", speed)
			fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
			fmt.Println("insert blocks progress:", currProgress, "/", targetHeight, "(", blockSpeed, "blocks/s", ")")
		}

	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (stg *StageStates) saveProgress(ctx context.Context, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = stg.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// save progress
	if err = s.Update(tx, stg.configs.bc.CurrentBlock().NumberU64()); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] saving progress for block States stage failed")
		return ErrSaveStateProgressFail
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (stg *StageStates) Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = stg.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
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

func (stg *StageStates) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = stg.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if useInternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
