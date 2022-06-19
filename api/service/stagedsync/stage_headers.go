package stagedsync

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"
)

// The number of blocks we should be able to re-org sub-second on commodity hardware.
// See https://hackmd.io/TdJtNs0dS56q-In8h-ShSg
const ShortPoSReorgThresholdBlocks = 10

type StageHeaders struct {
	configs StageHeadersCfg
}

type StageHeadersCfg struct {
	ctx context.Context
	db  kv.RwDB
}

func NewStageHeders(cfg StageHeadersCfg) *StageHeaders {
	return &StageHeaders{
		configs: cfg,
	}
}

func NewStageHeadersCfg(ctx context.Context, db kv.RwDB) StageHeadersCfg {
	return StageHeadersCfg{
		ctx: ctx,
		db:  db,
	}
}

func (headers *StageHeaders) Exec(firstCycle bool, badBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) error {
	useExternalTx := tx != nil
	if !useExternalTx {
		var err error
		tx, err = headers.configs.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	return nil
}

func (headers *StageHeaders) Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = headers.configs.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}
	// TODO: Delete canonical hashes that are being unwound

	if !useExternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (headers *StageHeaders) Prune(firstCycle bool, p *PruneState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = headers.configs.db.BeginRw(context.Background())
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
