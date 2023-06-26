package stagedstreamsync

import (
	"context"
	"fmt"
	"time"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type StageStateSync struct {
	configs StageStateSyncCfg
}

type StageStateSyncCfg struct {
	bc          core.BlockChain
	db          kv.RwDB
	concurrency int
	protocol    syncProtocol
	logger      zerolog.Logger
	logProgress bool
}

func NewStageStateSync(cfg StageStateSyncCfg) *StageStateSync {
	return &StageStateSync{
		configs: cfg,
	}
}

func NewStageStateSyncCfg(bc core.BlockChain,
	db kv.RwDB,
	concurrency int,
	protocol syncProtocol,
	logger zerolog.Logger,
	logProgress bool) StageStateSyncCfg {

	return StageStateSyncCfg{
		bc:          bc,
		db:          db,
		concurrency: concurrency,
		protocol:    protocol,
		logger:      logger,
		logProgress: logProgress,
	}
}

// Exec progresses States stage in the forward direction
func (stg *StageStateSync) Exec(ctx context.Context, bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) (err error) {

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

	if stg.configs.logProgress {
		fmt.Print("\033[s") // save the cursor position
	}

	for i := currProgress + 1; i <= targetHeight; i++ {
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

func (stg *StageStateSync) insertChain(gbm *blockDownloadManager,
	protocol syncProtocol,
	lbls prometheus.Labels,
	targetBN uint64) {

}

func (stg *StageStateSync) saveProgress(s *StageState, tx kv.RwTx) (err error) {

	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = stg.configs.db.BeginRw(context.Background())
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

func (stg *StageStateSync) Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
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

func (stg *StageStateSync) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
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
