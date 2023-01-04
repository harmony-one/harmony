package stagedstreamsync

import (
	"context"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
)

type StageShortRange struct {
	configs StageShortRangeCfg
}

type StageShortRangeCfg struct {
	ctx context.Context
	bc  core.BlockChain
	db  kv.RwDB
}

func NewStageShortRange(cfg StageShortRangeCfg) *StageShortRange {
	return &StageShortRange{
		configs: cfg,
	}
}

func NewStageShortRangeCfg(ctx context.Context, bc core.BlockChain, db kv.RwDB) StageShortRangeCfg {
	return StageShortRangeCfg{
		ctx: ctx,
		bc:  bc,
		db:  db,
	}
}

func (sr *StageShortRange) SetStageContext(ctx context.Context) {
	sr.configs.ctx = ctx
}

func (sr *StageShortRange) Exec(firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) error {

	// no need to do short range if we are redoing the stages because of bad block
	if invalidBlockRevert {
		return nil
	}

	// for long range sync, skip this stage
	if s.state.initSync {
		return nil
	}

	if _, ok := sr.configs.bc.(*core.EpochChain); ok {
		return nil
	}

	// do short range sync
	n, err := sr.doShortRangeSync(s)
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

// doShortRangeSync does the short range sync.
// Compared with long range sync, short range sync is more focused on syncing to the latest block.
// It consist of 3 steps:
// 1. Obtain the block hashes and compute the longest hash chain..
// 2. Get blocks by hashes from computed hash chain.
// 3. Insert the blocks to blockchain.
func (sr *StageShortRange) doShortRangeSync(s *StageState) (int, error) {

	numShortRangeCounterVec.With(s.state.promLabels()).Inc()

	srCtx, cancel := context.WithTimeout(s.state.ctx, ShortRangeTimeout)
	defer cancel()

	sh := &srHelper{
		syncProtocol: s.state.protocol,
		ctx:          srCtx,
		config:       s.state.config,
		logger:       utils.Logger().With().Str("mode", "short range").Logger(),
	}

	if err := sh.checkPrerequisites(); err != nil {
		return 0, errors.Wrap(err, "prerequisite")
	}
	curBN := sr.configs.bc.CurrentBlock().NumberU64()
	blkNums := sh.prepareBlockHashNumbers(curBN)
	hashChain, whitelist, err := sh.getHashChain(blkNums)
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

	blocks, stids, err := sh.getBlocksByHashes(hashChain, whitelist)
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

func (sr *StageShortRange) Revert(firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
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

func (sr *StageShortRange) CleanUp(firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
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
