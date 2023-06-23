package stagedstreamsync

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/harmony-one/harmony/shard"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/ledgerwatch/log/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

const (
	BlocksBucket          = "BlockBodies"
	BlockSignaturesBucket = "BlockSignatures"
	StageProgressBucket   = "StageProgress"

	// cache db keys
	LastBlockHeight = "LastBlockHeight"
	LastBlockHash   = "LastBlockHash"
)

var Buckets = []string{
	BlocksBucket,
	BlockSignaturesBucket,
	StageProgressBucket,
}

// CreateStagedSync creates an instance of staged sync
func CreateStagedSync(ctx context.Context,
	bc core.BlockChain,
	dbDir string,
	UseMemDB bool,
	isBeaconNode bool,
	protocol syncProtocol,
	config Config,
	logger zerolog.Logger,
	logProgress bool,
) (*StagedStreamSync, error) {

	isBeacon := bc.ShardID() == shard.BeaconChainShardID

	var mainDB kv.RwDB
	dbs := make([]kv.RwDB, config.Concurrency)
	if UseMemDB {
		mainDB = memdb.New(getMemDbTempPath(dbDir, -1))
		for i := 0; i < config.Concurrency; i++ {
			dbs[i] = memdb.New(getMemDbTempPath(dbDir, i))
		}
	} else {
		mainDB = mdbx.NewMDBX(log.New()).Path(getBlockDbPath(isBeacon, -1, dbDir)).MustOpen()
		for i := 0; i < config.Concurrency; i++ {
			dbPath := getBlockDbPath(isBeacon, i, dbDir)
			dbs[i] = mdbx.NewMDBX(log.New()).Path(dbPath).MustOpen()
		}
	}

	if errInitDB := initDB(ctx, mainDB, dbs, config.Concurrency); errInitDB != nil {
		return nil, errInitDB
	}

	stageHeadsCfg := NewStageHeadersCfg(bc, mainDB)
	stageShortRangeCfg := NewStageShortRangeCfg(bc, mainDB)
	stageSyncEpochCfg := NewStageEpochCfg(bc, mainDB)
	stageBodiesCfg := NewStageBodiesCfg(bc, mainDB, dbs, config.Concurrency, protocol, isBeacon, logProgress)
	stageStatesCfg := NewStageStatesCfg(bc, mainDB, dbs, config.Concurrency, logger, logProgress)
	stageFinishCfg := NewStageFinishCfg(mainDB)

	stages := DefaultStages(ctx,
		stageHeadsCfg,
		stageSyncEpochCfg,
		stageShortRangeCfg,
		stageBodiesCfg,
		stageStatesCfg,
		stageFinishCfg,
	)

	return New(
		bc,
		mainDB,
		stages,
		isBeacon,
		protocol,
		isBeaconNode,
		UseMemDB,
		config,
		logger,
	), nil
}

// initDB inits the sync loop main database and create buckets
func initDB(ctx context.Context, mainDB kv.RwDB, dbs []kv.RwDB, concurrency int) error {

	// create buckets for mainDB
	tx, errRW := mainDB.BeginRw(ctx)
	if errRW != nil {
		return errRW
	}
	defer tx.Rollback()

	for _, name := range Buckets {
		if err := tx.CreateBucket(GetStageName(name, false, false)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// create buckets for block cache DBs
	for _, db := range dbs {
		tx, errRW := db.BeginRw(ctx)
		if errRW != nil {
			return errRW
		}

		if err := tx.CreateBucket(BlocksBucket); err != nil {
			return err
		}
		if err := tx.CreateBucket(BlockSignaturesBucket); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

// getMemDbTempPath returns the path of the temporary cache database for memdb
func getMemDbTempPath(dbDir string, dbIndex int) string {
	if dbIndex >= 0 {
		return fmt.Sprintf("%s_%d", filepath.Join(dbDir, "cache/memdb/db"), dbIndex)
	}
	return filepath.Join(dbDir, "cache/memdb/db_main")
}

// getBlockDbPath returns the path of the cache database which stores blocks
func getBlockDbPath(beacon bool, loopID int, dbDir string) string {
	if beacon {
		if loopID >= 0 {
			return fmt.Sprintf("%s_%d", filepath.Join(dbDir, "cache/beacon_blocks_db"), loopID)
		} else {
			return filepath.Join(dbDir, "cache/beacon_blocks_db_main")
		}
	} else {
		if loopID >= 0 {
			return fmt.Sprintf("%s_%d", filepath.Join(dbDir, "cache/blocks_db"), loopID)
		} else {
			return filepath.Join(dbDir, "cache/blocks_db_main")
		}
	}
}

// doSync does the long range sync.
// One LongRangeSync consists of several iterations.
// For each iteration, estimate the current block number, then fetch block & insert to blockchain
func (s *StagedStreamSync) doSync(downloaderContext context.Context, initSync bool) (int, error) {

	var totalInserted int

	s.initSync = initSync

	if err := s.checkPrerequisites(); err != nil {
		return 0, err
	}

	var estimatedHeight uint64
	if initSync {
		if h, err := s.estimateCurrentNumber(downloaderContext); err != nil {
			return 0, err
		} else {
			estimatedHeight = h
			//TODO: use directly currentCycle var
			s.status.setTargetBN(estimatedHeight)
		}
		if curBN := s.bc.CurrentBlock().NumberU64(); estimatedHeight <= curBN {
			s.logger.Info().Uint64("current number", curBN).Uint64("target number", estimatedHeight).
				Msg(WrapStagedSyncMsg("early return of long range sync"))
			return 0, nil
		}

		s.startSyncing()
		defer s.finishSyncing()
	}

	for {
		ctx, cancel := context.WithCancel(downloaderContext)

		n, err := s.doSyncCycle(ctx, initSync)
		if err != nil {
			pl := s.promLabels()
			pl["error"] = err.Error()
			numFailedDownloadCounterVec.With(pl).Inc()

			cancel()
			return totalInserted + n, err
		}
		cancel()

		totalInserted += n

		// if it's not long range sync, skip loop
		if n < LastMileBlocksThreshold || !initSync {
			return totalInserted, nil
		}
	}

}

func (s *StagedStreamSync) doSyncCycle(ctx context.Context, initSync bool) (int, error) {

	// TODO: initSync=true means currentCycleNumber==0, so we can remove initSync

	var totalInserted int

	s.inserted = 0
	startHead := s.bc.CurrentBlock().NumberU64()
	canRunCycleInOneTransaction := false

	var tx kv.RwTx
	if canRunCycleInOneTransaction {
		var err error
		if tx, err = s.DB().BeginRw(ctx); err != nil {
			return totalInserted, err
		}
		defer tx.Rollback()
	}

	startTime := time.Now()

	// Do one cycle of staged sync
	initialCycle := s.currentCycle.Number == 0
	if err := s.Run(ctx, s.DB(), tx, initialCycle); err != nil {
		utils.Logger().Error().
			Err(err).
			Bool("isBeacon", s.isBeacon).
			Uint32("shard", s.bc.ShardID()).
			Uint64("currentHeight", startHead).
			Msgf(WrapStagedSyncMsg("sync cycle failed"))
		return totalInserted, err
	}

	totalInserted += s.inserted

	s.currentCycle.lock.Lock()
	s.currentCycle.Number++
	s.currentCycle.lock.Unlock()

	// calculating sync speed (blocks/second)
	if s.LogProgress && s.inserted > 0 {
		dt := time.Now().Sub(startTime).Seconds()
		speed := float64(0)
		if dt > 0 {
			speed = float64(s.inserted) / dt
		}
		syncSpeed := fmt.Sprintf("%.2f", speed)
		fmt.Println("sync speed:", syncSpeed, "blocks/s")
	}

	return totalInserted, nil
}

func (s *StagedStreamSync) startSyncing() {
	s.status.startSyncing()
	if s.evtDownloadStartedSubscribed {
		s.evtDownloadStarted.Send(struct{}{})
	}
}

func (s *StagedStreamSync) finishSyncing() {
	s.status.finishSyncing()
	if s.evtDownloadFinishedSubscribed {
		s.evtDownloadFinished.Send(struct{}{})
	}
}

func (s *StagedStreamSync) checkPrerequisites() error {
	return s.checkHaveEnoughStreams()
}

// estimateCurrentNumber roughly estimates the current block number.
// The block number does not need to be exact, but just a temporary target of the iteration
func (s *StagedStreamSync) estimateCurrentNumber(ctx context.Context) (uint64, error) {
	var (
		cnResults = make(map[sttypes.StreamID]uint64)
		lock      sync.Mutex
		wg        sync.WaitGroup
	)
	wg.Add(s.config.Concurrency)
	for i := 0; i != s.config.Concurrency; i++ {
		go func() {
			defer wg.Done()
			bn, stid, err := s.doGetCurrentNumberRequest(ctx)
			if err != nil {
				s.logger.Err(err).Str("streamID", string(stid)).
					Msg(WrapStagedSyncMsg("getCurrentNumber request failed"))
				if !errors.Is(err, context.Canceled) {
					s.protocol.StreamFailed(stid, "getCurrentNumber request failed")
				}
				return
			}
			lock.Lock()
			cnResults[stid] = bn
			lock.Unlock()
		}()
	}
	wg.Wait()

	if len(cnResults) == 0 {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		return 0, ErrZeroBlockResponse
	}
	bn := computeBlockNumberByMaxVote(cnResults)
	return bn, nil
}
