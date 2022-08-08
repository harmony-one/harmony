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

func (heads *StageHeads) Exec(firstCycle bool, invalidBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) error {

	if len(s.state.syncConfig.peers) == 0 {
		return ErrNoConnectedPeers
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

	targetHeight := uint64(0)
	if errV := CreateView(heads.configs.ctx, heads.configs.db, tx, func(etx kv.Tx) (err error) {
		if targetHeight, err = s.CurrentStageProgress(etx); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	utils.Logger().Info().
		Msgf("[STAGED_SYNC] current block height: %d)", heads.configs.bc.CurrentBlock().NumberU64())

	if targetHeight <= heads.configs.bc.CurrentBlock().NumberU64() {
		maxPeersHeight := s.state.syncStatus.MaxPeersHeight
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] max peers height: %d)", maxPeersHeight)
		if maxPeersHeight <= heads.configs.bc.CurrentBlock().NumberU64() {
			s.state.Done()
			return nil
		}
		targetHeight = maxPeersHeight
	}
	s.state.syncStatus.currentCycle.TargetHeight = targetHeight

	currentHeight := heads.configs.bc.CurrentBlock().NumberU64()
	if currentHeight >= targetHeight {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] Node is now IN SYNC! (isBeacon: %t, ShardID: %d, targetHeight: %d, currentHeight: %d)",
				s.state.IsBeacon(), heads.configs.bc.ShardID(), targetHeight, currentHeight)
		s.state.Done()
		return nil
	}

	maxBlocksPerSyncCycle := s.state.MaxBlocksPerSyncCycle
	utils.Logger().Info().
		Msgf("[STAGED_SYNC] max blocks per sync cycle is: %d)", maxBlocksPerSyncCycle)

	if maxBlocksPerSyncCycle > 0 && targetHeight-currentHeight > maxBlocksPerSyncCycle {
		targetHeight = currentHeight + maxBlocksPerSyncCycle
	}
	if err := s.Update(tx, targetHeight); err != nil { //SaveStageProgress(tx, Heads, s.state.isBeacon, targetHeight); err != nil {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving progress for headers stage failed: %v", err)
		return err
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (heads *StageHeads) Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) (err error) {
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
