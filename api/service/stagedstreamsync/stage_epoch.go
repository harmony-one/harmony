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
	s.state.inserted = n
	if err != nil {
		utils.Logger().Info().Err(err).Msg("short range for epoch sync failed")
		return err
	}
	if n > 0 {
		utils.Logger().Info().Err(err).Int("blocks inserted", n).Msg("epoch sync short range blocks inserted successfully")
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
		// if error is ErrNotEnoughStreams but still some streams available,
		// it can continue syncing, otherwise return error
		// here we are not doing concurrent processes, so even 1 stream should be enough
		if err != ErrNotEnoughStreams || s.state.protocol.NumStreams() == 0 {
			return 0, errors.Wrap(err, "prerequisite")
		}
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

	blocks, streamID, err := sh.getBlocksChain(ctx, bns)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return 0, nil
		}
		return 0, errors.Wrap(err, "getHashChain")
	}
	if len(blocks) == 0 {
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
