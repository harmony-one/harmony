package stagedsync

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/shard"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/log/v3"
)

const (
	BlockHashesBucket            = "BlockHashes"
	BeaconBlockHashesBucket      = "BeaconBlockHashes"
	DownloadedBlocksBucket       = "BlockBodies"
	BeaconDownloadedBlocksBucket = "BeaconBlockBodies" // Beacon Block bodies are downloaded, TxHash and UncleHash are getting verified
	LastMileBlocksBucket         = "LastMileBlocks"    // last mile blocks to catch up with the consensus
	StageProgressBucket          = "StageProgress"

	// cache db keys
	LastBlockHeight = "LastBlockHeight"
	LastBlockHash   = "LastBlockHash"

	// cache db  names
	BlockHashesCacheDB = "cache_block_hashes"
	BlockCacheDB       = "cache_blocks"
)

var Buckets = []string{
	BlockHashesBucket,
	BeaconBlockHashesBucket,
	DownloadedBlocksBucket,
	BeaconDownloadedBlocksBucket,
	LastMileBlocksBucket,
	StageProgressBucket,
}

// CreateStagedSync creates an instance of staged sync
func CreateStagedSync(
	ip string,
	port string,
	peerHash [20]byte,
	bc core.BlockChain,
	dbDir string,
	role nodeconfig.Role,
	isExplorer bool,
	TurboMode bool,
	UseMemDB bool,
	doubleCheckBlockHashes bool,
	maxBlocksPerCycle uint64,
	maxBackgroundBlocks uint64,
	maxMemSyncCycleSize uint64,
	verifyAllSig bool,
	verifyHeaderBatchSize uint64,
	insertChainBatchSize int,
	logProgress bool,
	debugMode bool,
) (*StagedSync, error) {

	ctx := context.Background()
	isBeacon := bc.ShardID() == shard.BeaconChainShardID

	var db kv.RwDB
	if UseMemDB {
		// maximum Blocks in memory is maxMemSyncCycleSize + maxBackgroundBlocks
		var dbMapSize datasize.ByteSize
		if isBeacon {
			// for memdb, maximum 512 kb for beacon chain each block (in average) should be enough
			dbMapSize = datasize.ByteSize(maxMemSyncCycleSize+maxBackgroundBlocks) * 512 * datasize.KB
		} else {
			// for memdb, maximum 256 kb for each shard chains block (in average) should be enough
			dbMapSize = datasize.ByteSize(maxMemSyncCycleSize+maxBackgroundBlocks) * 256 * datasize.KB
		}
		// we manually create memory db because "db = memdb.New()" sets the default map size (64 MB) which is not enough for some cases
		db = mdbx.NewMDBX(log.New()).MapSize(dbMapSize).InMem("cache_db").MustOpen()
	} else {
		if isBeacon {
			dbPath := filepath.Join(dbDir, "cache_beacon_db")
			db = mdbx.NewMDBX(log.New()).Path(dbPath).MustOpen()
		} else {
			dbPath := filepath.Join(dbDir, "cache_shard_db")
			db = mdbx.NewMDBX(log.New()).Path(dbPath).MustOpen()
		}
	}

	if errInitDB := initDB(ctx, db); errInitDB != nil {
		return nil, errInitDB
	}

	headsCfg := NewStageHeadersCfg(ctx, bc, db)
	blockHashesCfg := NewStageBlockHashesCfg(ctx, bc, dbDir, db, isBeacon, TurboMode, logProgress)
	bodiesCfg := NewStageBodiesCfg(ctx, bc, dbDir, db, isBeacon, TurboMode, logProgress)
	statesCfg := NewStageStatesCfg(ctx, bc, db, logProgress)
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
		DefaultRevertOrder,
		DefaultCleanUpOrder,
		TurboMode,
		UseMemDB,
		doubleCheckBlockHashes,
		maxBlocksPerCycle,
		maxBackgroundBlocks,
		maxMemSyncCycleSize,
		verifyAllSig,
		verifyHeaderBatchSize,
		insertChainBatchSize,
		logProgress,
		debugMode,
	), nil
}

// initDB inits sync loop main database and create buckets
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
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// SyncLoop will keep syncing with peers until catches up
func (s *StagedSync) SyncLoop(bc core.BlockChain, worker *worker.Worker, isBeacon bool, consensus *consensus.Consensus, loopMinTime time.Duration) {

	utils.Logger().Info().
		Uint64("current height", bc.CurrentBlock().NumberU64()).
		Msgf("staged sync is executing ... ")

	if !s.IsBeacon() {
		s.RegisterNodeInfo()
	}

	// get max peers height
	maxPeersHeight, err := s.getMaxPeerHeight()
	if err != nil {
		return
	}
	utils.Logger().Info().
		Uint64("maxPeersHeight", maxPeersHeight).
		Msgf("[STAGED_SYNC] max peers height")
	s.syncStatus.MaxPeersHeight = maxPeersHeight

	for {
		if len(s.syncConfig.peers) < NumPeersLowBound {
			// TODO: try to use reserved nodes
			utils.Logger().Warn().
				Int("num peers", len(s.syncConfig.peers)).
				Msgf("[STAGED_SYNC] Not enough connected peers")
			break
		}
		startHead := bc.CurrentBlock().NumberU64()

		if startHead >= maxPeersHeight {
			utils.Logger().Info().
				Bool("isBeacon", isBeacon).
				Uint32("shard", bc.ShardID()).
				Uint64("maxPeersHeight", maxPeersHeight).
				Uint64("currentHeight", startHead).
				Msgf("[STAGED_SYNC] Node is now IN SYNC!")
			break
		}
		startTime := time.Now()

		if err := s.runSyncCycle(bc, worker, isBeacon, consensus, maxPeersHeight); err != nil {
			utils.Logger().Error().
				Err(err).
				Bool("isBeacon", isBeacon).
				Uint32("shard", bc.ShardID()).
				Uint64("currentHeight", startHead).
				Msgf("[STAGED_SYNC] sync cycle failed")
			break
		}

		if loopMinTime != 0 {
			waitTime := loopMinTime - time.Since(startTime)
			utils.Logger().Debug().
				Bool("isBeacon", isBeacon).
				Uint32("shard", bc.ShardID()).
				Interface("duration", waitTime).
				Msgf("[STAGED SYNC] Node is syncing ..., it's waiting a few seconds until next loop")
			c := time.After(waitTime)
			select {
			case <-s.Context().Done():
				return
			case <-c:
			}
		}

		// calculating sync speed (blocks/second)
		currHead := bc.CurrentBlock().NumberU64()
		if s.LogProgress && currHead-startHead > 0 {
			dt := time.Now().Sub(startTime).Seconds()
			speed := float64(0)
			if dt > 0 {
				speed = float64(currHead-startHead) / dt
			}
			syncSpeed := fmt.Sprintf("%.2f", speed)
			fmt.Println("sync speed:", syncSpeed, "blocks/s (", currHead, "/", maxPeersHeight, ")")
		}

		s.syncStatus.currentCycle.lock.Lock()
		s.syncStatus.currentCycle.Number++
		s.syncStatus.currentCycle.lock.Unlock()

	}

	if consensus != nil {
		if err := s.addConsensusLastMile(s.Blockchain(), consensus); err != nil {
			utils.Logger().Error().
				Err(err).
				Msg("[STAGED_SYNC] Add consensus last mile")
		}
		// TODO: move this to explorer handler code.
		if s.isExplorer {
			consensus.UpdateConsensusInformation()
		}
	}
	s.purgeAllBlocksFromCache()
	utils.Logger().Info().
		Uint64("new height", bc.CurrentBlock().NumberU64()).
		Msgf("staged sync is executed")
	return
}

// runSyncCycle will run one cycle of staged syncing
func (s *StagedSync) runSyncCycle(bc core.BlockChain, worker *worker.Worker, isBeacon bool, consensus *consensus.Consensus, maxPeersHeight uint64) error {
	canRunCycleInOneTransaction := s.MaxBlocksPerSyncCycle > 0 && s.MaxBlocksPerSyncCycle <= s.MaxMemSyncCycleSize
	var tx kv.RwTx
	if canRunCycleInOneTransaction {
		var err error
		if tx, err = s.DB().BeginRw(context.Background()); err != nil {
			return err
		}
		defer tx.Rollback()
	}
	// Do one cycle of staged sync
	initialCycle := s.syncStatus.currentCycle.Number == 0
	syncErr := s.Run(s.DB(), tx, initialCycle)
	if syncErr != nil {
		utils.Logger().Error().
			Err(syncErr).
			Bool("isBeacon", s.IsBeacon()).
			Uint32("shard", s.Blockchain().ShardID()).
			Msgf("[STAGED_SYNC] Sync loop failed")
		s.purgeOldBlocksFromCache()
		return syncErr
	}
	if tx != nil {
		errTx := tx.Commit()
		if errTx != nil {
			return errTx
		}
	}
	return nil
}
