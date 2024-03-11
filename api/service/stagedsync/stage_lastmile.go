package stagedsync

import (
	"context"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type StageLastMile struct {
	configs StageLastMileCfg
}

type StageLastMileCfg struct {
	ctx context.Context
	bc  core.BlockChain
	db  kv.RwDB
}

func NewStageLastMile(cfg StageLastMileCfg) *StageLastMile {
	return &StageLastMile{
		configs: cfg,
	}
}

func NewStageLastMileCfg(ctx context.Context, bc core.BlockChain, db kv.RwDB) StageLastMileCfg {
	return StageLastMileCfg{
		ctx: ctx,
		bc:  bc,
		db:  db,
	}
}

func (lm *StageLastMile) Exec(firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) (err error) {

	maxPeersHeight := s.state.syncStatus.MaxPeersHeight
	targetHeight := s.state.syncStatus.currentCycle.TargetHeight
	isLastCycle := targetHeight >= maxPeersHeight
	if !isLastCycle {
		return nil
	}

	bc := lm.configs.bc
	// update blocks after node start sync
	parentHash := bc.CurrentBlock().Hash()
	for {
		block := s.state.getMaxConsensusBlockFromParentHash(parentHash)
		if block == nil {
			break
		}
		err = s.state.UpdateBlockAndStatus(block, bc)
		if err != nil {
			break
		}
		parentHash = block.Hash()
	}
	// TODO ek – Do we need to hold syncMux now that syncConfig has its own mutex?
	s.state.syncMux.Lock()
	s.state.syncConfig.ForEachPeer(func(peer *SyncPeerConfig) (brk bool) {
		peer.newBlocks = []*types.Block{}
		return
	})
	s.state.syncMux.Unlock()

	// update last mile blocks if any
	parentHash = bc.CurrentBlock().Hash()
	for {
		block := s.state.getBlockFromLastMileBlocksByParentHash(parentHash)
		if block == nil {
			break
		}
		err = s.state.UpdateBlockAndStatus(block, bc)
		if err != nil {
			break
		}
		parentHash = block.Hash()
	}

	return nil
}

func (lm *StageLastMile) Revert(firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = lm.configs.db.BeginRw(lm.configs.ctx)
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

func (lm *StageLastMile) CleanUp(firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = lm.configs.db.BeginRw(lm.configs.ctx)
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
