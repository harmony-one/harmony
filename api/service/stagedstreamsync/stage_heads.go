package stagedstreamsync

import (
	"context"

	"github.com/harmony-one/harmony/core"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
)

type StageHeads struct {
	configs StageHeadsCfg
}

type StageHeadsCfg struct {
	bc     core.BlockChain
	db     kv.RwDB
	logger zerolog.Logger
}

func NewStageHeads(cfg StageHeadsCfg) *StageHeads {
	return &StageHeads{
		configs: cfg,
	}
}

func NewStageHeadersCfg(bc core.BlockChain, db kv.RwDB, logger zerolog.Logger) StageHeadsCfg {
	return StageHeadsCfg{
		bc: bc,
		db: db,
		logger: logger.With().
			Str("stage", "StageHeads").
			Logger(),
	}
}

func (heads *StageHeads) Exec(ctx context.Context, firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) error {
	// no need to update target if we are redoing the stages because of bad block
	if invalidBlockRevert {
		return nil
	}

	// no need for short range sync
	if !s.state.initSync {
		return nil
	}

	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = heads.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	maxHeight := s.state.status.GetTargetBN()
	maxBlocksPerSyncCycle := uint64(1024) // TODO: should be in config -> s.state.MaxBlocksPerSyncCycle
	currentHeight := s.state.CurrentBlockNumber()
	s.state.currentCycle.SetTargetHeight(maxHeight)
	targetHeight := uint64(0)
	if errV := CreateView(ctx, heads.configs.db, tx, func(etx kv.Tx) (err error) {
		if targetHeight, err = s.CurrentStageProgress(etx); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	if currentHeight >= maxHeight {
		return nil
	}

	// if current height is ahead of target height, we need recalculate target height
	if currentHeight >= targetHeight {
		if maxHeight <= currentHeight {
			return nil
		}
		heads.configs.logger.Info().
			Uint64("max blocks per sync cycle", maxBlocksPerSyncCycle).
			Uint64("currentHeight", currentHeight).
			Uint64("maxPeersHeight", maxHeight).
			Uint64("targetHeight", targetHeight).
			Msgf(WrapStagedSyncMsg("current height is ahead of target height, target height is readjusted to max peers height"))
		targetHeight = maxHeight
	}

	if targetHeight > maxHeight {
		targetHeight = maxHeight
	}

	if maxBlocksPerSyncCycle > 0 && targetHeight-currentHeight > maxBlocksPerSyncCycle {
		targetHeight = currentHeight + maxBlocksPerSyncCycle
	}

	// check pivot: if chain hasn't reached to pivot yet
	if !s.state.status.IsFullSyncCycle() && s.state.status.HasPivotBlock() {
		// set target height on the pivot block
		if !s.state.status.IsStatesSynced() && targetHeight > s.state.status.GetPivotBlockNumber() {
			targetHeight = s.state.status.GetPivotBlockNumber()
		}
	}

	s.state.currentCycle.SetTargetHeight(targetHeight)

	if err := s.Update(tx, targetHeight); err != nil {
		heads.configs.logger.Error().
			Err(err).
			Msgf(WrapStagedSyncMsg("saving progress for headers stage failed"))
		return err
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (heads *StageHeads) Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = heads.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if err = u.Done(tx); err != nil {
		return err
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (heads *StageHeads) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = heads.configs.db.BeginRw(ctx)
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
