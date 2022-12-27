package stagedstreamsync

import (
	"context"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
)

type StageEpoch struct {
	configs StageEpochCfg
}

type StageEpochCfg struct {
	ctx context.Context
	bc  core.BlockChain
	db  kv.RwDB
}

func NewStageEpoch(cfg StageEpochCfg) *StageEpoch {
	return &StageEpoch{
		configs: cfg,
	}
}

func NewStageEpochCfg(ctx context.Context, bc core.BlockChain, db kv.RwDB) StageEpochCfg {
	return StageEpochCfg{
		ctx: ctx,
		bc:  bc,
		db:  db,
	}
}

func (sr *StageEpoch) SetStageContext(ctx context.Context) {
	sr.configs.ctx = ctx
}

func (sr *StageEpoch) Exec(firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) error {

	// no need to update epoch chain if we are redoing the stages because of bad block
	if invalidBlockRevert {
		return nil
	}
	// for long range sync, skip this stage
	if s.state.initSync {
		return nil
	}

	if _, ok := sr.configs.bc.(*core.EpochChain); !ok {
		return nil
	}

	// doShortRangeSyncForEpochSync
	n, err := sr.doShortRangeSyncForEpochSync(s)
	s.state.inserted = n
	if err != nil {
		return err
	}

	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = sr.configs.db.BeginRw(sr.configs.ctx)
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

func (sr *StageEpoch) doShortRangeSyncForEpochSync(s *StageState) (int, error) {

	numShortRangeCounterVec.With(s.state.promLabels()).Inc()

	srCtx, cancel := context.WithTimeout(s.state.ctx, ShortRangeTimeout)
	defer cancel()

	//TODO: merge srHelper with StageEpochConfig
	sh := &srHelper{
		syncProtocol: s.state.protocol,
		ctx:          srCtx,
		config:       s.state.config,
		logger:       utils.Logger().With().Str("mode", "epoch chain short range").Logger(),
	}

	if err := sh.checkPrerequisites(); err != nil {
		return 0, errors.Wrap(err, "prerequisite")
	}
	curBN := s.state.bc.CurrentBlock().NumberU64()
	bns := make([]uint64, 0, BlocksPerRequest)
	// in epoch chain, we have only the last block of each epoch, so, the current
	// block's epoch number shows the last epoch we have. We should start
	// from next epoch then
	loopEpoch := s.state.bc.CurrentHeader().Epoch().Uint64() + 1
	for len(bns) < BlocksPerRequest {
		blockNum := shard.Schedule.EpochLastBlock(loopEpoch)
		if blockNum > curBN {
			bns = append(bns, blockNum)
		}
		loopEpoch = loopEpoch + 1
	}

	if len(bns) == 0 {
		return 0, nil
	}

	////////////////////////////////////////////////////////
	hashChain, whitelist, err := sh.getHashChain(bns)
	if err != nil {
		return 0, errors.Wrap(err, "getHashChain")
	}
	if len(hashChain) == 0 {
		// short circuit for no sync is needed
		return 0, nil
	}
	blocks, streamID, err := sh.getBlocksByHashes(hashChain, whitelist)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("epoch sync getBlocksByHashes failed")
		if !errors.Is(err, context.Canceled) {
			sh.removeStreams(whitelist) // Remote nodes cannot provide blocks with target hashes
		}
		return 0, errors.Wrap(err, "epoch sync getBlocksByHashes")
	}
	///////////////////////////////////////////////////////
	// TODO: check this
	// blocks, streamID, err := sh.getBlocksChain(bns)
	// if err != nil {
	// 	return 0, errors.Wrap(err, "getHashChain")
	// }
	///////////////////////////////////////////////////////
	if len(blocks) == 0 {
		// short circuit for no sync is needed
		return 0, nil
	}

	n, err := s.state.bc.InsertChain(blocks, true)
	numBlocksInsertedShortRangeHistogramVec.With(s.state.promLabels()).Observe(float64(n))
	if err != nil {
		utils.Logger().Info().Err(err).Int("blocks inserted", n).Msg("Insert block failed")
		sh.removeStreams(streamID) // Data provided by remote nodes is corrupted
		return n, err
	}
	if n > 0 {
		utils.Logger().Info().Int("blocks inserted", n).Msg("Insert block success")
	}
	return n, nil
}

func (sr *StageEpoch) Revert(firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = sr.configs.db.BeginRw(context.Background())
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

func (sr *StageEpoch) CleanUp(firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = sr.configs.db.BeginRw(context.Background())
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
