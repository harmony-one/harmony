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

	// cache db  names
	Block_Hashes_Cache_DB = "cache_block_hashes"
	Block_Cache_DB        = "cache_blocks"
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

// CreateStagedSync creates an instance of staged sync
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
	maxBackgroundBlocks uint64,
	maxMemSyncCycleSize uint64,
	insertChainBatchSize int,
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
		maxBackgroundBlocks,
		maxMemSyncCycleSize,
		insertChainBatchSize,
	), nil
}

// init sync loop main database and create buckets
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

	canRunCycleInOneTransaction := s.MaxBlocksPerSyncCycle > 0 && s.MaxBlocksPerSyncCycle <= s.MaxMemSyncCycleSize
	var tx kv.RwTx
	if canRunCycleInOneTransaction {
		var err error
		if tx, err = s.DB().BeginRw(context.Background()); err != nil {
			return
		}
		defer tx.Rollback()
	}

	// Do one cycle of staged sync
	startTime := time.Now()
	startHead := bc.CurrentBlock().NumberU64()
	initialCycle := true //TODO: should be based on cycle number
	syncErr := s.Run(s.DB(), tx, initialCycle)
	if syncErr != nil {
		utils.Logger().Error().Err(syncErr).
			Msgf("[STAGED_SYNC] Sync loop failed (isBeacon: %t, ShardID: %d, error: %s)",
				s.IsBeacon(), s.Blockchain().ShardID(), syncErr)
		s.purgeOldBlocksFromCache()
	}
	if tx != nil {
		errTx := tx.Commit()
		if errTx != nil {
			return
		}
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
