package stagedsync

import (
	"context"
	"fmt"
	"sync"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
)

type StageBlockHashes struct {
	configs StageBlockHashesCfg
}

type StageBlockHashesCfg struct {
	mtx sync.Mutex
	ctx context.Context
	db  kv.RwDB
}

func NewStageBlockHashes(cfg StageBlockHashesCfg) *StageBlockHashes {
	return &StageBlockHashes{
		configs: cfg,
	}
}

func NewStageBlockHashesCfg(ctx context.Context, db kv.RwDB) StageBlockHashesCfg {
	return StageBlockHashesCfg{
		mtx: sync.Mutex{},
		ctx: ctx,
		db:  db,
	}
}

func (bh *StageBlockHashes) Exec(firstCycle bool, badBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) (err error) {

	curCycle := s.state.syncStatus.CurrentCycle()
	// Gets consensus hashes.
	if err := s.state.getConsensusHashes(curCycle.StartHash, curCycle.Size, tx); err != nil {
		return errors.Wrap(err, "getConsensusHashes")
	}

	if err := s.state.syncConfig.GetBlockHashesConsensusAndCleanUp(); err != nil {
		return err
	}
	// double check block hashes
	if s.state.DoubleCheckBlockHashes {
		invalidPeersMap, validBlockHashes, err := s.state.getInvalidPeersByBlockHashes(tx)
		if err != nil {
			return err
		}
		if validBlockHashes < int(curCycle.Size) {
			return errors.Wrap(err, "getBlockHashes: peers haven't sent all requested block hashes")
		}
		s.state.syncConfig.cleanUpInvalidPeers(invalidPeersMap)
	}

	return nil
}

func (bh *StageBlockHashes) Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = bh.configs.db.BeginRw(bh.configs.ctx)
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

func (bh *StageBlockHashes) Prune(firstCycle bool, p *PruneState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = bh.configs.db.BeginRw(bh.configs.ctx)
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
