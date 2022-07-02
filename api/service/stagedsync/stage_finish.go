package stagedsync

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"
)

type StageFinish struct {
	configs StageFinishCfg
}

type StageFinishCfg struct {
	ctx context.Context
	db  kv.RwDB
}

func NewStageFinish(cfg StageFinishCfg) *StageFinish {
	return &StageFinish{
		configs: cfg,
	}
}

func NewStageFinishCfg(ctx context.Context, db kv.RwDB) StageFinishCfg {
	return StageFinishCfg{
		ctx: ctx,
		db:  db,
	}
}

func (finish *StageFinish) Exec(firstCycle bool, badBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) error {
	
	return s.state.generateNewState(s.state.Blockchain())
	// return nil
}

func (finish *StageFinish) Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = finish.configs.db.BeginRw(finish.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if err = u.Done(tx); err != nil {
		return err
	}
	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (finish *StageFinish) Prune(firstCycle bool, p *PruneState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = finish.configs.db.BeginRw(finish.configs.ctx)
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
