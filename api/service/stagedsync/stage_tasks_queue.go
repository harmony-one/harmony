package stagedsync

import (
	"context"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/kv"
)

type StageTasksQueue struct {
	configs StageTasksQueueCfg
}
type StageTasksQueueCfg struct {
	ctx context.Context
	db kv.RwDB
}

func NewStageTasksQueue(cfg StageTasksQueueCfg) *StageTasksQueue {
	return &StageTasksQueue{
		configs: cfg,
	}
}

func NewStageTasksQueueCfg(ctx context.Context, db kv.RwDB) StageTasksQueueCfg {
	return StageTasksQueueCfg{
		ctx: ctx,
		db: db,
	}
}

func (tq *StageTasksQueue) Exec(firstCycle bool, badBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = tq.configs.db.BeginRw(tq.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	s.state.generateStateSyncTaskQueue(s.state.Blockchain())

	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (tq *StageTasksQueue) Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = tq.configs.db.BeginRw(tq.configs.ctx)
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

func (tq *StageTasksQueue) Prune(firstCycle bool, p *PruneState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = tq.configs.db.BeginRw(tq.configs.ctx)
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
