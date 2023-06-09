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

type StageShortRange struct {
	configs StageShortRangeCfg
}

type StageShortRangeCfg struct {
	bc core.BlockChain
	db kv.RwDB
}

func NewStageShortRange(cfg StageShortRangeCfg) *StageShortRange {
	return &StageShortRange{
		configs: cfg,
	}
}

func NewStageShortRangeCfg(bc core.BlockChain, db kv.RwDB) StageShortRangeCfg {
	return StageShortRangeCfg{
		bc: bc,
		db: db,
	}
}

func (sr *StageShortRange) Exec(ctx context.Context, firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) error {
	// no need to do short range if we are redoing the stages because of bad block
	if invalidBlockRevert {
		return nil
	}

	// for long range sync, skip this stage
	if s.state.initSync {
		return nil
	}

	if sr.configs.bc.ShardID() == shard.BeaconChainShardID && !s.state.isBeaconNode {
		return nil
	}

	// do short range sync
	n, err := sr.doShortRangeSync(ctx, s)
	s.state.inserted = n
	if err != nil {
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

// doShortRangeSync does the short range sync.
// Compared with long range sync, short range sync is more focused on syncing to the latest block.
// It consist of 3 steps:
// 1. Obtain the block hashes and compute the longest hash chain..
// 2. Get blocks by hashes from computed hash chain.
// 3. Insert the blocks to blockchain.
func (sr *StageShortRange) doShortRangeSync(ctx context.Context, s *StageState) (int, error) {
	numShortRangeCounterVec.With(s.state.promLabels()).Inc()
	ctx, cancel := context.WithTimeout(ctx, ShortRangeTimeout)
	defer cancel()

	sh := &srHelper{
		syncProtocol: s.state.protocol,
		config:       s.state.config,
		logger:       utils.Logger().With().Str("mode", "short range").Logger(),
	}

	if err := sh.checkPrerequisites(); err != nil {
		return 0, errors.Wrap(err, "prerequisite")
	}
	curBN := sr.configs.bc.CurrentBlock().NumberU64()
	blkNums := sh.prepareBlockHashNumbers(curBN)
	hashChain, whitelist, err := sh.getHashChain(ctx, blkNums)
	if err != nil {
		return 0, errors.Wrap(err, "getHashChain")
	}

	if len(hashChain) == 0 {
		// short circuit for no sync is needed
		return 0, nil
	}

	expEndBN := curBN + uint64(len(hashChain))
	utils.Logger().Info().Uint64("current number", curBN).
		Uint64("target number", expEndBN).
		Interface("hashChain", hashChain).
		Msg("short range start syncing")

	s.state.status.setTargetBN(expEndBN)

	s.state.status.startSyncing()
	defer func() {
		utils.Logger().Info().Msg("short range finished syncing")
		s.state.status.finishSyncing()
	}()

	blocks, stids, err := sh.getBlocksByHashes(ctx, hashChain, whitelist)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("getBlocksByHashes failed")
		if !errors.Is(err, context.Canceled) {
			sh.removeStreams(whitelist) // Remote nodes cannot provide blocks with target hashes
		}
		return 0, errors.Wrap(err, "getBlocksByHashes")
	}

	utils.Logger().Info().Int("num blocks", len(blocks)).Msg("getBlockByHashes result")

	n, err := verifyAndInsertBlocks(sr.configs.bc, blocks)
	numBlocksInsertedShortRangeHistogramVec.With(s.state.promLabels()).Observe(float64(n))
	if err != nil {
		utils.Logger().Warn().Err(err).Int("blocks inserted", n).Msg("Insert block failed")
		if sh.blameAllStreams(blocks, n, err) {
			sh.removeStreams(whitelist) // Data provided by remote nodes is corrupted
		} else {
			// It is the last block gives a wrong commit sig. Blame the provider of the last block.
			st2Blame := stids[len(stids)-1]
			sh.removeStreams([]sttypes.StreamID{st2Blame})
		}
		return n, err
	}
	utils.Logger().Info().Err(err).Int("blocks inserted", n).Msg("Insert block success")

	return n, nil
}

func (sr *StageShortRange) Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
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

func (sr *StageShortRange) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
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
