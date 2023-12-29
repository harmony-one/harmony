package stagedstreamsync

import (
	"context"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type StageHeads struct {
	configs StageHeadsCfg
}

type StageHeadsCfg struct {
	bc core.BlockChain
	db kv.RwDB
}

func NewStageHeads(cfg StageHeadsCfg) *StageHeads {
	return &StageHeads{
		configs: cfg,
	}
}

func NewStageHeadersCfg(bc core.BlockChain, db kv.RwDB) StageHeadsCfg {
	return StageHeadsCfg{
		bc: bc,
		db: db,
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

	maxHeight := s.state.status.targetBN
	maxBlocksPerSyncCycle := uint64(1024) // TODO: should be in config -> s.state.MaxBlocksPerSyncCycle
	currentHeight := s.state.CurrentBlockNumber()
	s.state.currentCycle.TargetHeight = maxHeight
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
		utils.Logger().Info().
			Uint64("max blocks per sync cycle", maxBlocksPerSyncCycle).
			Uint64("maxPeersHeight", maxHeight).
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
	if s.state.status.cycleSyncMode != FullSync && s.state.status.pivotBlock != nil {
		// set target height on the pivot block
		if !s.state.status.statesSynced && targetHeight > s.state.status.pivotBlock.NumberU64() {
			targetHeight = s.state.status.pivotBlock.NumberU64()
		}
	}

	s.state.currentCycle.TargetHeight = targetHeight

	if err := s.Update(tx, targetHeight); err != nil {
		utils.Logger().Error().
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
