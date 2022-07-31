package stagedsync

import (
	"context"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/ledgerwatch/erigon-lib/kv"
)

// The number of blocks we should be able to re-org sub-second on commodity hardware.
// See https://hackmd.io/TdJtNs0dS56q-In8h-ShSg
const ShortPoSReorgThresholdBlocks = 10

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

func (heads *StageHeads) Exec(firstCycle bool, badBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) error {

	if len(s.state.syncConfig.peers) == 0 {
		return ErrNoConnectedPeers
	}

	useExternalTx := tx != nil
	if !useExternalTx {
		var err error
		tx, err = heads.configs.db.BeginRw(heads.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	otherHeight := uint64(0)
	if errV := CreateView(heads.configs.ctx, heads.configs.db, tx, func(etx kv.Tx) (err error) {
		if otherHeight, err = s.CurrentStageProgress(etx); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	utils.Logger().Info().
		Msgf("[STAGED_SYNC] current block height: %d)", heads.configs.bc.CurrentBlock().NumberU64())

	if otherHeight <= heads.configs.bc.CurrentBlock().NumberU64() {
		maxPeersHeight, err := s.state.getMaxPeerHeight(s.state.IsBeacon())
		if err != nil {
			return err
		}
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] max peers height: %d)", maxPeersHeight)
		if maxPeersHeight <= heads.configs.bc.CurrentBlock().NumberU64() {
			s.state.Done()
			return nil
		}
		otherHeight = maxPeersHeight
	}
	s.state.syncStatus.maxPeersHeight = otherHeight

	currentHeight := heads.configs.bc.CurrentBlock().NumberU64()
	if currentHeight >= otherHeight {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] Node is now IN SYNC! (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
				s.state.IsBeacon(), heads.configs.bc.ShardID(), otherHeight, currentHeight)
		s.state.Done()
		return nil
	}

	maxBlocksPerSyncCycle := s.state.MaxBlocksPerSyncCycle
	utils.Logger().Info().
		Msgf("[STAGED_SYNC] max blocks per sync cycle is: %d)", maxBlocksPerSyncCycle)

	targetHeight := otherHeight
	if maxBlocksPerSyncCycle > 0 && targetHeight-currentHeight > maxBlocksPerSyncCycle {
		targetHeight = currentHeight + maxBlocksPerSyncCycle
	}
	if err := s.Update(tx, targetHeight); err != nil { //SaveStageProgress(tx, Heads, s.state.isBeacon, targetHeight); err != nil {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving progress for headers stage failed: %v", err)
		return err
	}

	if !useExternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (heads *StageHeads) Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = heads.configs.db.BeginRw(context.Background())
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

func (heads *StageHeads) Prune(firstCycle bool, p *PruneState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = heads.configs.db.BeginRw(context.Background())
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
