package stagedsync

import (
	"context"
	"fmt"
	"time"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	//"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	//"github.com/ledgerwatch/log/v3"
)

const (
	BlockHashesBucket            = "BlockHashes"
	BeaconBlockHashesBucket      = "BeaconBlockHashes"
	DownloadedBlocksBucket       = "BlockBodies"
	BeaconDownloadedBlocksBucket = "BeaconBlockBodies" // Beacon Block bodies are downloaded, TxHash and UncleHash are getting verified
	LastMileBlocksBucket         = "LastMileBlocks"    // last mile blocks to catch up with the consensus
	StageProgressBucket          = "StageProgress"
)

var Buckets = []string{
	BlockHashesBucket,
	BeaconBlockHashesBucket,
	DownloadedBlocksBucket,
	BeaconDownloadedBlocksBucket,
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
	doubleCheckBlockHashes bool,
	maxBlocksPerCycle uint64,
) (*StagedSync, error) {

	ctx := context.Background()
	db := memdb.New()
	//db := mdbx.NewMDBX(log.New()).Path("./test_db") .MustOpen()

	if errInitDB := initDB(ctx, db); errInitDB != nil {
		return nil, errInitDB
	}

	headersCfg := NewStageHeadersCfg(ctx, bc, db)
	blockHashesCfg := NewStageBlockHashesCfg(ctx, bc, db)
	bodiesCfg := NewStageBodiesCfg(ctx, bc, db)
	statesCfg := NewStageStatesCfg(ctx, bc, db)
	lastMileCfg := NewStageLastMileCfg(ctx, bc, db)
	finishCfg := NewStageFinishCfg(ctx, db)

	stages := DefaultStages(ctx,
		headersCfg,
		blockHashesCfg,
		bodiesCfg,
		statesCfg,
		lastMileCfg,
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
		doubleCheckBlockHashes,
		maxBlocksPerCycle,
	), nil
}

func initDB(ctx context.Context, db kv.RwDB) error {
	tx, errRW := db.BeginRw(ctx)
	if errRW != nil {
		return errRW
	}
	defer tx.Rollback()
	for _, name := range Buckets {
		// create bucket
		if err := tx.CreateBucket(GetStageName(name, false, false)); err != nil {
			return err
		}
		// create bucket for beacon
		if err := tx.CreateBucket(GetStageName(name, true, false)); err != nil {
			return err
		}
		// create bucket for prune
		if err := tx.CreateBucket(GetStageName(name, false, true)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to initiate db: %w", err)
	}
	return nil
}

// SyncLoop will keep syncing with peers until catches up
func (s *StagedSync) SyncLoop(bc *core.BlockChain, worker *worker.Worker, isBeacon bool, consensus *consensus.Consensus, loopMinTime time.Duration) {

	utils.Logger().Info().Msgf("staged sync is executing ...")

	if !s.IsBeacon() {
		s.RegisterNodeInfo()
	}

	// canRunCycleInOneTransaction := true //!initialCycle && highestSeenHeader-origin < 8096 && highestSeenHeader-finishProgressBefore < 8096

	// var tx kv.RwTx // on this variable will run sync cycle.
	// if canRunCycleInOneTransaction {
	// 	tx, err = s.DB().BeginRw(context.Background())
	// 	if err != nil {
	// 		return headBlockHash, err
	// 	}
	// 	defer tx.Rollback()
	// }

	// Do one step of staged sync
	start := time.Now()
	initialCycle := true
	syncErr := s.Run(s.DB(), nil, initialCycle)
	if syncErr != nil {
		utils.Logger().Error().Err(syncErr).
			Msgf("[STAGED_SYNC] Sync loop failed (isBeacon: %t, ShardID: %d, error: %s)",
				s.IsBeacon(), s.Blockchain().ShardID(), syncErr)
		s.purgeOldBlocksFromCache()
	}
	if loopMinTime != 0 {
		waitTime := loopMinTime - time.Since(start)
		utils.Logger().Info().
			Msgf("[STAGED SYNC] Node is syncing ..., it's waiting %d seconds until next loop (isBeacon: %t, ShardID: %d)",
				waitTime, s.IsBeacon(), s.Blockchain().ShardID())
		c := time.After(waitTime)
		select {
		case <-s.Context().Done():
			return
		case <-c:
		}
	}

	// if canRunCycleInOneTransaction {
	// 	commitStart := time.Now()
	// 	errTx := tx.Commit()
	// 	if errTx != nil {
	// 		return headBlockHash, errTx
	// 	}
	// 	log.Info("Commit cycle", "in", time.Since(commitStart))
	// }

	// defer close(waitForDone)

	//initialCycle := true

	// for {
	// 	start := time.Now()
	// 	otherHeight := s.getMaxPeerHeight(s.IsBeacon())
	// 	currentHeight := s.Blockchain().CurrentBlock().NumberU64()
	// 	if currentHeight >= otherHeight {
	// 		utils.Logger().Info().
	// 			Msgf("[STAGED_SYNC] Node is now IN SYNC! (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
	// 				s.IsBeacon(), s.Blockchain().ShardID(), otherHeight, currentHeight)
	// 		break
	// 	}
	// 	var syncErr error
	// 	for {
	// 		currentHeight = s.Blockchain().CurrentBlock().NumberU64()
	// 		if currentHeight >= otherHeight {
	// 			break
	// 		}
	// 		utils.Logger().Info().
	// 			Msgf("[STAGED_SYNC] Node is OUT OF SYNC (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
	// 				s.IsBeacon(), s.Blockchain().ShardID(), otherHeight, currentHeight)

	// 		startHash := s.Blockchain().CurrentBlock().Hash()
	// 		size := uint32(otherHeight - currentHeight)
	// 		if size > SyncLoopBatchSize {
	// 			size = SyncLoopBatchSize
	// 		}

	// 		// Do one step of staged sync
	// 		_, syncErr = s.StartStageLoop(startHash[:], size, initialCycle)

	// 		if syncErr != nil {
	// 			utils.Logger().Error().Err(syncErr).
	// 				Msgf("[STAGED_SYNC] ProcessStateSync failed (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
	// 					s.IsBeacon(), s.Blockchain().ShardID(), otherHeight, currentHeight)
	// 			s.purgeOldBlocksFromCache()
	// 			break
	// 		}
	// 		initialCycle = false
	// 		s.purgeOldBlocksFromCache()

	// 		if loopMinTime != 0 {
	// 			waitTime := loopMinTime - time.Since(start)
	// 			utils.Logger().Info().
	// 				Msgf("[STAGED SYNC] Node is syncing ..., it's waiting %d seconds until next loop (isBeacon: %t, ShardID: %d, currentHeight: %d)",
	// 					waitTime, s.IsBeacon(), s.Blockchain().ShardID(), currentHeight)
	// 			c := time.After(waitTime)
	// 			select {
	// 			case <-s.Context().Done():
	// 				return
	// 			case <-c:
	// 			}
	// 		}
	// 		// if haven't get any new block
	// 		if currentHeight == s.Blockchain().CurrentBlock().NumberU64() {
	// 			break
	// 		}
	// 	}
	// 	if syncErr != nil {
	// 		break
	// 	}
	// }
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
