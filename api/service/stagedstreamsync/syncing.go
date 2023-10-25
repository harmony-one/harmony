package stagedstreamsync

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
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
	consensus *consensus.Consensus,
	dbDir string,
	isBeaconNode bool,
	protocol syncProtocol,
	config Config,
	logger zerolog.Logger,
) (*StagedStreamSync, error) {

	logger.Info().
		Uint32("shard", bc.ShardID()).
		Bool("beaconNode", isBeaconNode).
		Bool("memdb", config.UseMemDB).
		Str("dbDir", dbDir).
		Bool("serverOnly", config.ServerOnly).
		Int("minStreams", config.MinStreams).
		Msg(WrapStagedSyncMsg("creating staged sync"))

	var mainDB kv.RwDB
	dbs := make([]kv.RwDB, config.Concurrency)
	if config.UseMemDB {
		mainDB = memdb.New(getBlockDbPath(bc.ShardID(), isBeaconNode, -1, dbDir))
		for i := 0; i < config.Concurrency; i++ {
			dbPath := getBlockDbPath(bc.ShardID(), isBeaconNode, i, dbDir)
			dbs[i] = memdb.New(dbPath)
		}
	} else {
		logger.Info().
			Str("path", getBlockDbPath(bc.ShardID(), isBeaconNode, -1, dbDir)).
			Msg(WrapStagedSyncMsg("creating main db"))
		mainDB = mdbx.NewMDBX(log.New()).Path(getBlockDbPath(bc.ShardID(), isBeaconNode, -1, dbDir)).MustOpen()
		for i := 0; i < config.Concurrency; i++ {
			dbPath := getBlockDbPath(bc.ShardID(), isBeaconNode, i, dbDir)
			dbs[i] = mdbx.NewMDBX(log.New()).Path(dbPath).MustOpen()
		}
	}

	if errInitDB := initDB(ctx, mainDB, dbs, config.Concurrency); errInitDB != nil {
		logger.Error().Err(errInitDB).Msg("create staged sync instance failed")
		return nil, errInitDB
	}

	stageHeadsCfg := NewStageHeadersCfg(bc, mainDB)
	stageShortRangeCfg := NewStageShortRangeCfg(bc, mainDB)
	stageSyncEpochCfg := NewStageEpochCfg(bc, mainDB)
	stageBodiesCfg := NewStageBodiesCfg(bc, mainDB, dbs, config.Concurrency, protocol, isBeaconNode, config.LogProgress)
	stageStatesCfg := NewStageStatesCfg(bc, mainDB, dbs, config.Concurrency, logger, config.LogProgress)
	lastMileCfg := NewStageLastMileCfg(ctx, bc, mainDB)
	stageFinishCfg := NewStageFinishCfg(mainDB)

	stages := DefaultStages(ctx,
		stageHeadsCfg,
		stageSyncEpochCfg,
		stageShortRangeCfg,
		stageBodiesCfg,
		stageStatesCfg,
		lastMileCfg,
		stageFinishCfg,
	)

	logger.Info().
		Uint32("shard", bc.ShardID()).
		Bool("beaconNode", isBeaconNode).
		Bool("memdb", config.UseMemDB).
		Str("dbDir", dbDir).
		Bool("serverOnly", config.ServerOnly).
		Int("minStreams", config.MinStreams).
		Msg(WrapStagedSyncMsg("staged sync created successfully"))

	return New(
		bc,
		consensus,
		mainDB,
		stages,
		isBeaconNode,
		protocol,
		isBeaconNode,
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
func getBlockDbPath(shardID uint32, beacon bool, loopID int, dbDir string) string {
	if beacon {
		if loopID >= 0 {
			return fmt.Sprintf("%s_%d", filepath.Join(dbDir, "cache/beacon_blocks_db"), loopID)
		} else {
			return filepath.Join(dbDir, "cache/beacon_blocks_db_main")
		}
	} else {
		if loopID >= 0 {
			return fmt.Sprintf("%s_%d_%d", filepath.Join(dbDir, "cache/blocks_db"), shardID, loopID)
		} else {
			return fmt.Sprintf("%s_%d", filepath.Join(dbDir, "cache/blocks_db_main"), shardID)
		}
	}
}

func (s *StagedStreamSync) Debug(source string, msg interface{}) {
	// only log the msg in debug mode
	if !s.config.DebugMode {
		return
	}
	pc, _, _, _ := runtime.Caller(1)
	caller := runtime.FuncForPC(pc).Name()
	callerParts := strings.Split(caller, "/")
	if len(callerParts) > 0 {
		caller = callerParts[len(callerParts)-1]
	}
	src := source
	if src == "" {
		src = "message"
	}
	// SSSD: STAGED STREAM SYNC DEBUG
	if msg == nil {
		fmt.Printf("[SSSD:%s] %s: nil or no error\n", caller, src)
	} else if err, ok := msg.(error); ok {
		fmt.Printf("[SSSD:%s] %s: %s\n", caller, src, err.Error())
	} else if str, ok := msg.(string); ok {
		fmt.Printf("[SSSD:%s] %s: %s\n", caller, src, str)
	} else {
		fmt.Printf("[SSSD:%s] %s: %v\n", caller, src, msg)
	}
}

// doSync does the long range sync.
// One LongRangeSync consists of several iterations.
// For each iteration, estimate the current block number, then fetch block & insert to blockchain
func (s *StagedStreamSync) doSync(downloaderContext context.Context, initSync bool) (uint64, int, error) {

	var totalInserted int

	s.initSync = initSync

	if err := s.checkPrerequisites(); err != nil {
		return 0, 0, err
	}

	var estimatedHeight uint64
	if initSync {
		if h, err := s.estimateCurrentNumber(downloaderContext); err != nil {
			return 0, 0, err
		} else {
			estimatedHeight = h
			//TODO: use directly currentCycle var
			s.status.setTargetBN(estimatedHeight)
		}
		if curBN := s.bc.CurrentBlock().NumberU64(); estimatedHeight <= curBN {
			s.logger.Info().Uint64("current number", curBN).Uint64("target number", estimatedHeight).
				Msg(WrapStagedSyncMsg("early return of long range sync (chain is already ahead of target height)"))
			return estimatedHeight, 0, nil
		}
	}

	s.startSyncing()
	defer s.finishSyncing()

	for {
		ctx, cancel := context.WithCancel(downloaderContext)

		n, err := s.doSyncCycle(ctx, initSync)
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Bool("initSync", s.initSync).
				Bool("isBeacon", s.isBeacon).
				Uint32("shard", s.bc.ShardID()).
				Msg(WrapStagedSyncMsg("sync cycle failed"))

			pl := s.promLabels()
			pl["error"] = err.Error()
			numFailedDownloadCounterVec.With(pl).Inc()

			cancel()
			return estimatedHeight, totalInserted + n, err
		}
		cancel()

		totalInserted += n

		// if it's not long range sync, skip loop
		if n == 0 || !initSync {
			break
		}
	}

	if totalInserted > 0 {
		utils.Logger().Info().
			Bool("initSync", s.initSync).
			Bool("isBeacon", s.isBeacon).
			Uint32("shard", s.bc.ShardID()).
			Int("blocks", totalInserted).
			Msg(WrapStagedSyncMsg("sync cycle blocks inserted successfully"))
	}

	// add consensus last mile blocks
	if s.consensus != nil {
		if hashes, err := s.addConsensusLastMile(s.Blockchain(), s.consensus); err != nil {
			utils.Logger().Error().Err(err).
				Msg("[STAGED_STREAM_SYNC] Add consensus last mile failed")
			s.RollbackLastMileBlocks(downloaderContext, hashes)
			return estimatedHeight, totalInserted, err
		} else {
			totalInserted += len(hashes)
		}
		// TODO: move this to explorer handler code.
		if s.isExplorer {
			s.consensus.UpdateConsensusInformation()
		}
	}
	s.purgeLastMileBlocksFromCache()

	return estimatedHeight, totalInserted, nil
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
