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

	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/ledgerwatch/log/v3"
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
	ExtraBlockHashesBucket       = "ExtraBlockHashes" //extra block hashes for backgound process

	// cache db keys
	StartBlockHeight = "StartBlockHeight"
	StartBlockHash   = "StartBlockHash"
	LastBlockHeight  = "LastBlockHeight"
	LastBlockHash    = "LastBlockHash"
)

var Buckets = []string{
	BlockHashesBucket,
	BeaconBlockHashesBucket,
	DownloadedBlocksBucket,
	BeaconDownloadedBlocksBucket,
	LastMileBlocksBucket,
	StageProgressBucket,
	ExtraBlockHashesBucket,
}

func CreateStagedSync(
	ip string,
	port string,
	peerHash [20]byte,
	bc core.BlockChain,
	role nodeconfig.Role,
	isBeacon bool,
	isExplorer bool,
	TurboMode bool,
	UseMemDB bool,
	doubleCheckBlockHashes bool,
	maxBlocksPerCycle uint64,
) (*StagedSync, error) {

	ctx := context.Background()

	var db kv.RwDB
	if UseMemDB {
		db = memdb.New()
	} else {
		if isBeacon {
			db = mdbx.NewMDBX(log.New()).Path("cache_beacon_db").MustOpen()
		} else {
			db = mdbx.NewMDBX(log.New()).Path("cache_shard_db").MustOpen()
		}
		return nil, fmt.Errorf("Staged sync doesn't support disk yet")
	}

	if errInitDB := initDB(ctx, db); errInitDB != nil {
		return nil, errInitDB
	}

	headsCfg := NewStageHeadersCfg(ctx, bc, db)
	blockHashesCfg := NewStageBlockHashesCfg(ctx, bc, db, isBeacon, TurboMode)
	bodiesCfg := NewStageBodiesCfg(ctx, bc, db, isBeacon, TurboMode)
	statesCfg := NewStageStatesCfg(ctx, bc, db)
	lastMileCfg := NewStageLastMileCfg(ctx, bc, db)
	finishCfg := NewStageFinishCfg(ctx, db)

	stages := DefaultStages(ctx,
		headsCfg,
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
		TurboMode,
		UseMemDB,
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
		// // create bucket for prune
		// if err := tx.CreateBucket(GetStageName(name, false, true)); err != nil {
		// 	return err
		// }
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to initiate db: %w", err)
	}
	return nil
}

// SyncLoop will keep syncing with peers until catches up
func (s *StagedSync) SyncLoop(bc core.BlockChain, worker *worker.Worker, isBeacon bool, consensus *consensus.Consensus, loopMinTime time.Duration) {

	utils.Logger().Info().Msgf("staged sync is executing ...")

	if !s.IsBeacon() {
		s.RegisterNodeInfo()
	}

	// canRunCycleInOneTransaction := s.MaxBlocksPerSyncCycle > 0 && s.MaxBlocksPerSyncCycle <= 8192
	// var tx kv.RwTx
	// if canRunCycleInOneTransaction {
	// 	var err error
	// 	if tx, err = s.DB().BeginRw(context.Background()); err != nil {
	// 		return
	// 	}
	// 	defer tx.Rollback()
	// }

	// Do one step of staged sync
	startTime := time.Now()
	startHead := bc.CurrentBlock().NumberU64()
	initialCycle := true
	syncErr := s.Run(s.DB(), nil, initialCycle)
	if syncErr != nil {
		utils.Logger().Error().Err(syncErr).
			Msgf("[STAGED_SYNC] Sync loop failed (isBeacon: %t, ShardID: %d, error: %s)",
				s.IsBeacon(), s.Blockchain().ShardID(), syncErr)
		s.purgeOldBlocksFromCache()
	}
	// calculating sync speed (blocks/second)
	currHead := bc.CurrentBlock().NumberU64()
	if currHead-startHead > 0 {
		dt := time.Now().Sub(startTime).Seconds()
		speed := float64(0)
		if dt > 0 {
			speed = float64(currHead-startHead) / dt
		}
		syncSpeed := fmt.Sprintf("%.2f", speed)
		fmt.Println("sync speed:", syncSpeed, "blocks/s")
	}

	if loopMinTime != 0 {
		waitTime := loopMinTime - time.Since(startTime)
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
	// 	errTx := tx.Commit()
	// 	if errTx != nil {
	// 		return
	// 	}
	// }

	// defer close(waitForDone)

	if consensus != nil {
		if err := s.addConsensusLastMile(s.Blockchain(), consensus); err != nil {
			utils.Logger().Error().Err(err).Msg("[STAGED_SYNC] Add consensus last mile")
		}
		// TODO: move this to explorer handler code.
		if s.isExplorer {
			consensus.UpdateConsensusInformation()
		}
	}
	utils.Logger().Info().Msgf("staged sync loop executed")
}
