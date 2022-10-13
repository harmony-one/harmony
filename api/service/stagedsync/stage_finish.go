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

func (finish *StageFinish) Exec(firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) error {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = finish.configs.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// TODO: prepare indices (useful for RPC) and finalize

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (bh *StageFinish) clearBucket(tx kv.RwTx, isBeacon bool) error {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = bh.configs.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}
	bucketName := GetBucketName(BlockHashesBucket, isBeacon)
	if err := tx.ClearBucket(bucketName); err != nil {
		return err
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (finish *StageFinish) Revert(firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = finish.configs.db.BeginRw(finish.configs.ctx)
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

func (finish *StageFinish) CleanUp(firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = finish.configs.db.BeginRw(finish.configs.ctx)
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
