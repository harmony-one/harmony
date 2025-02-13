package stagedstreamsync

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
)

type StageFinish struct {
	configs StageFinishCfg
}

type StageFinishCfg struct {
	db     kv.RwDB
	logger zerolog.Logger
}

func NewStageFinish(cfg StageFinishCfg) *StageFinish {
	return &StageFinish{
		configs: cfg,
	}
}

func NewStageFinishCfg(db kv.RwDB, logger zerolog.Logger) StageFinishCfg {
	return StageFinishCfg{
		db: db,
		logger: logger.With().
			Str("stage", "StageFinish").
			Logger(),
	}
}

func (finish *StageFinish) Exec(ctx context.Context, firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) error {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = finish.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// TODO: prepare indices (useful for RPC) and finalize

	// switch to Full Sync Mode if the states are synced
	if s.state.status.IsStatesSynced() {
		s.state.status.SetCycleSyncMode(FullSync)
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (finish *StageFinish) clearBucket(ctx context.Context, tx kv.RwTx, isBeaconShard bool) error {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = finish.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (finish *StageFinish) Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = finish.configs.db.BeginRw(ctx)
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

func (finish *StageFinish) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = finish.configs.db.BeginRw(ctx)
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
