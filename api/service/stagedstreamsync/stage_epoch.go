package stagedstreamsync

import (
	"context"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/harmony-one/harmony/shard"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
)

type StageEpoch struct {
	configs StageEpochCfg
}

type StageEpochCfg struct {
	bc core.BlockChain
	db kv.RwDB
}

func NewStageEpoch(cfg StageEpochCfg) *StageEpoch {
	return &StageEpoch{
		configs: cfg,
	}
}

func NewStageEpochCfg(bc core.BlockChain, db kv.RwDB) StageEpochCfg {
	return StageEpochCfg{
		bc: bc,
		db: db,
	}
}

func (sr *StageEpoch) Exec(ctx context.Context, firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) error {

	s.state.Debug("shard", sr.configs.bc.ShardID())
	s.state.Debug("long_range", s.state.initSync)

	// no need to update epoch chain if we are redoing the stages because of bad block
	if invalidBlockRevert {
		return nil
	}
	// for long range sync, skip this stage
	if s.state.initSync {
		return nil
	}

	if sr.configs.bc.ShardID() != shard.BeaconChainShardID || s.state.isBeaconNode {
		return nil
	}

	// doShortRangeSyncForEpochSync
	n, err := sr.doShortRangeSyncForEpochSync(ctx, s)
	s.state.Debug("doShortRangeSyncForEpochSync/n", n)
	s.state.Debug("doShortRangeSyncForEpochSync/err", err)
	s.state.inserted = n
	if err != nil {
		s.state.Debug("doShortRangeSyncForEpochSync/error", err)
		return err
	}

	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = sr.configs.db.BeginRw(ctx)
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

func (sr *StageEpoch) doShortRangeSyncForEpochSync(ctx context.Context, s *StageState) (int, error) {

	numShortRangeCounterVec.With(s.state.promLabels()).Inc()

	ctx, cancel := context.WithTimeout(ctx, ShortRangeTimeout)
	defer cancel()

	//TODO: merge srHelper with StageEpochConfig
	sh := &srHelper{
		syncProtocol: s.state.protocol,
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
	s.state.Debug("block numbers", bns)

	blocks, streamID, err := sh.getBlocksChain(ctx, bns)
	if err != nil {
		s.state.Debug("getBlocksChain/error", err)
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return 0, nil
		}
		return 0, errors.Wrap(err, "getHashChain")
	}
	if len(blocks) == 0 {
		s.state.Debug("getBlocksChain", "returns empty blocks array")
		// short circuit for no sync is needed
		return 0, nil
	}

	n, err := s.state.bc.InsertChain(blocks, true)
	numBlocksInsertedShortRangeHistogramVec.With(s.state.promLabels()).Observe(float64(n))
	if err != nil {
		utils.Logger().Info().Err(err).Int("blocks inserted", n).Msg("Insert block failed")
		sh.streamsFailed([]sttypes.StreamID{streamID}, "corrupted data")
		return n, err
	}
	if n > 0 {
		s.state.Debug("epoch_blocks_inserted", n)
		utils.Logger().Info().Int("blocks inserted", n).Msg("Insert block success")
	}
	return n, nil
}

func (sr *StageEpoch) Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = sr.configs.db.BeginRw(ctx)
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

func (sr *StageEpoch) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = sr.configs.db.BeginRw(ctx)
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
