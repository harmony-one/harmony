package stagedsync

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
	ctx context.Context
	bc  core.BlockChain
	db  kv.RwDB
}

func NewStageHeads(cfg StageHeadsCfg) *StageHeads {
	return &StageHeads{
		configs: cfg,
	}
}

func NewStageHeadersCfg(ctx context.Context, bc core.BlockChain, db kv.RwDB) StageHeadsCfg {
	return StageHeadsCfg{
		ctx: ctx,
		bc:  bc,
		db:  db,
	}
}

func (heads *StageHeads) Exec(firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) error {

	if len(s.state.syncConfig.peers) < NumPeersLowBound {
		return ErrNotEnoughConnectedPeers
	}

	// no need to update target if we are redoing the stages because of bad block
	if invalidBlockRevert {
		return nil
	}

	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = heads.configs.db.BeginRw(heads.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	maxPeersHeight := s.state.syncStatus.MaxPeersHeight
	maxBlocksPerSyncCycle := s.state.MaxBlocksPerSyncCycle
	currentHeight := heads.configs.bc.CurrentBlock().NumberU64()
	s.state.syncStatus.currentCycle.TargetHeight = maxPeersHeight
	targetHeight := uint64(0)
	if errV := CreateView(heads.configs.ctx, heads.configs.db, tx, func(etx kv.Tx) (err error) {
		if targetHeight, err = s.CurrentStageProgress(etx); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	// if current height is ahead of target height, we need recalculate target height
	if targetHeight <= currentHeight {
		if maxPeersHeight <= currentHeight {
			return nil
		}
		utils.Logger().Info().
			Uint64("max blocks per sync cycle", maxBlocksPerSyncCycle).
			Uint64("maxPeersHeight", maxPeersHeight).
			Msgf("[STAGED_SYNC] current height is ahead of target height, target height is readjusted to max peers height")
		targetHeight = maxPeersHeight
	}

	if targetHeight > maxPeersHeight {
		targetHeight = maxPeersHeight
	}

	if maxBlocksPerSyncCycle > 0 && targetHeight-currentHeight > maxBlocksPerSyncCycle {
		targetHeight = currentHeight + maxBlocksPerSyncCycle
	}

	s.state.syncStatus.currentCycle.TargetHeight = targetHeight

	if err := s.Update(tx, targetHeight); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] saving progress for headers stage failed")
		return err
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (heads *StageHeads) Revert(firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = heads.configs.db.BeginRw(context.Background())
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

func (heads *StageHeads) CleanUp(firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = heads.configs.db.BeginRw(context.Background())
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
