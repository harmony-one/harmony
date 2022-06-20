package stagedsync

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
)

const (
	CommonBlocksBucket   = "Blocks"
	BlockHashesBucket    = "BlockHashes"
	LastMileBlocksBucket = "LastMileBlocks" // last mile blocks to catch up with the consensus
	StageProgressBucket  = "StageProgress"
)

var Buckets = []string{
	CommonBlocksBucket,
	LastMileBlocksBucket,
	StageProgressBucket,
}

func CreateStagedSync(
	ip string,
	port string,
	peerHash [20]byte,
	bc *core.BlockChain,
	role nodeconfig.Role,
	isBeacon bool,
	isExplorer bool,
) (*StagedSync, error) {

	ctx := context.Background()
	db := memdb.New()
	if errInitDB := initDB(ctx, db); errInitDB != nil {
		return nil, errInitDB
	}

	headersCfg := NewStageHeadersCfg(ctx, db)
	blockHashesCfg := NewStageBlockHashesCfg(ctx, db)
	taskQueueCfg := NewStageTasksQueueCfg(ctx, db)
	bodiesCfg := NewStageBodiesCfg(ctx, db)
	finishCfg := NewStageFinishCfg(ctx, db)

	stages := DefaultStages(ctx,
		headersCfg,
		blockHashesCfg,
		taskQueueCfg,
		bodiesCfg,
		finishCfg,
	)

	// During Import we don't want other services like header requests, body requests etc. to be running.
	// Hence we run it in the test mode.
	return New(ctx,
		ip,
		port,
		peerHash,
		bc,
		role,
		isBeacon,
		isExplorer,
		db,
		stages,
		DefaultUnwindOrder,
		DefaultPruneOrder,
	), nil
}

func initDB(ctx context.Context, db kv.RwDB) error {
	for _, name := range Buckets {
		tx, errRW := db.BeginRw(ctx)
		if errRW != nil {
			return errRW
		}
		defer tx.Rollback()
		if err := tx.CreateBucket(name); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to initiate db: %w", err)
		}
	}

	return nil
}

// SyncLoop will keep syncing with peers until catches up
func (s *StagedSync) SyncLoop(bc *core.BlockChain, worker *worker.Worker, isBeacon bool, consensus *consensus.Consensus, loopMinTime time.Duration) {

	utils.Logger().Info().Msgf("staged sync is executing ...")

	if !s.IsBeacon() {
		s.RegisterNodeInfo()
	}

	// defer close(waitForDone)

	initialCycle := true

	for {
		start := time.Now()
		otherHeight := s.getMaxPeerHeight(s.IsBeacon())
		currentHeight := s.Blockchain().CurrentBlock().NumberU64()
		if currentHeight >= otherHeight {
			utils.Logger().Info().
				Msgf("[STAGED_SYNC] Node is now IN SYNC! (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
					s.IsBeacon(), s.Blockchain().ShardID(), otherHeight, currentHeight)
			break
		}
		var syncErr error
		for {
			currentHeight = s.Blockchain().CurrentBlock().NumberU64()
			if currentHeight >= otherHeight {
				break
			}
			utils.Logger().Info().
				Msgf("[STAGED_SYNC] Node is OUT OF SYNC (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
					s.IsBeacon(), s.Blockchain().ShardID(), otherHeight, currentHeight)

			startHash := s.Blockchain().CurrentBlock().Hash()
			size := uint32(otherHeight - currentHeight)
			if size > SyncLoopBatchSize {
				size = SyncLoopBatchSize
			}

			// Do one step of staged sync
			_, syncErr = s.StageLoopStep(startHash[:], size, initialCycle)

			if syncErr != nil {
				utils.Logger().Error().Err(syncErr).
					Msgf("[STAGED_SYNC] ProcessStateSync failed (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
						s.IsBeacon(), s.Blockchain().ShardID(), otherHeight, currentHeight)
				s.purgeOldBlocksFromCache()
				break
			}
			initialCycle = false
			s.purgeOldBlocksFromCache()

			if loopMinTime != 0 {
				waitTime := loopMinTime - time.Since(start)
				utils.Logger().Info().
					Msgf("[STAGED SYNC] Node is syncing ..., it's waiting %d seconds until next loop (isBeacon: %t, ShardID: %d, currentHeight: %d)",
						waitTime, s.IsBeacon(), s.Blockchain().ShardID(), currentHeight)
				c := time.After(waitTime)
				select {
				case <-s.Context().Done():
					return
				case <-c:
				}
			}
			// if haven't get any new block
			if currentHeight==s.Blockchain().CurrentBlock().NumberU64() {
				break
			}
		}
		if syncErr != nil {
			break
		}
	}
	if consensus != nil {
		if err := s.addConsensusLastMile(s.Blockchain(), consensus); err != nil {
			utils.Logger().Error().Err(err).Msg("[STAGED_SYNC] Add consensus last mile")
		}
		// TODO: move this to explorer handler code.
		if s.isExplorer {
			consensus.UpdateConsensusInformation()
		}
	}
	utils.Logger().Info().Msgf("staged sync executed")
	s.purgeAllBlocksFromCache()
}

func (s *StagedSync) StageLoopStep(
	startHash []byte,
	size uint32,
	initialCycle bool,
) (headBlockHash common.Hash, err error) {
	// db kv.RwDB,
	// sync *Sync,
	// highestSeenHeader uint64,
	// initialCycle bool,
	// updateHead func(ctx context.Context, head uint64, hash common.Hash, td *uint256.Int),
	// snapshotMigratorFinal func(tx kv.Tx) error,

	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("%+v", rec)
		}
	}() // avoid crash because Erigon's core does many things

	// var origin, finishProgressBefore uint64
	// if err := db.View(ctx, func(tx kv.Tx) error {
	// 	origin, err = stages.GetStageProgress(tx, stages.Headers)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	finishProgressBefore, err = stages.GetStageProgress(tx, stages.Finish)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return nil
	// }); err != nil {
	// 	return headBlockHash, err
	// }

	canRunCycleInOneTransaction := true //!initialCycle && highestSeenHeader-origin < 8096 && highestSeenHeader-finishProgressBefore < 8096

	var tx kv.RwTx // on this variable will run sync cycle.
	if canRunCycleInOneTransaction {
		tx, err = s.DB().BeginRw(context.Background())
		if err != nil {
			return headBlockHash, err
		}
		defer tx.Rollback()
	}

	err = s.Run(s.DB(), tx, initialCycle, startHash[:], size)
	if err != nil {
		return headBlockHash, err
	}
	if canRunCycleInOneTransaction {
		commitStart := time.Now()
		errTx := tx.Commit()
		if errTx != nil {
			return headBlockHash, errTx
		}
		log.Info("Commit cycle", "in", time.Since(commitStart))
	}
	// var rotx kv.Tx
	// if rotx, err = db.BeginRo(ctx); err != nil {
	// 	return headBlockHash, err
	// }
	// defer rotx.Rollback()

	// // Update sentry status for peers to see our sync status
	// var headTd *big.Int
	// var head uint64
	// var headHash common.Hash
	// if head, err = stages.GetStageProgress(rotx, stages.Headers); err != nil {
	// 	return headBlockHash, err
	// }
	// if headHash, err = rawdb.ReadCanonicalHash(rotx, head); err != nil {
	// 	return headBlockHash, err
	// }
	// if headTd, err = rawdb.ReadTd(rotx, headHash, head); err != nil {
	// 	return headBlockHash, err
	// }
	// headBlockHash = rawdb.ReadHeadBlockHash(rotx)

	// if canRunCycleInOneTransaction && snapshotMigratorFinal != nil {
	// 	err = snapshotMigratorFinal(rotx)
	// 	if err != nil {
	// 		log.Error("snapshot migration failed", "err", err)
	// 	}
	// }
	// rotx.Rollback()

	// headTd256, overflow := uint256.FromBig(headTd)
	// if overflow {
	// 	return headBlockHash, fmt.Errorf("headTds higher than 2^256-1")
	// }
	// updateHead(ctx, head, headHash, headTd256)

	return headBlockHash, nil
}
