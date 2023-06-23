package stagedstreamsync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	syncproto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type InvalidBlock struct {
	Active   bool
	Number   uint64
	Hash     common.Hash
	IsLogged bool
	StreamID []sttypes.StreamID
}

func (ib *InvalidBlock) set(num uint64, hash common.Hash, resetBadStreams bool) {
	ib.Active = true
	ib.IsLogged = false
	ib.Number = num
	ib.Hash = hash
	if resetBadStreams {
		ib.StreamID = make([]sttypes.StreamID, 0)
	}
}

func (ib *InvalidBlock) resolve() {
	ib.Active = false
	ib.IsLogged = false
	ib.Number = 0
	ib.Hash = common.Hash{}
	ib.StreamID = ib.StreamID[:0]
}

func (ib *InvalidBlock) addBadStream(bsID sttypes.StreamID) {
	// only add uniques IDs
	for _, stID := range ib.StreamID {
		if stID == bsID {
			return
		}
	}
	ib.StreamID = append(ib.StreamID, bsID)
}

type StagedStreamSync struct {
	bc           core.BlockChain
	isBeacon     bool
	isExplorer   bool
	db           kv.RwDB
	protocol     syncProtocol
	isBeaconNode bool
	gbm          *blockDownloadManager // initialized when finished get block number
	inserted     int
	config       Config
	logger       zerolog.Logger
	status       *status //TODO: merge this with currentSyncCycle
	initSync     bool    // if sets to true, node start long range syncing
	UseMemDB     bool

	revertPoint     *uint64 // used to run stages
	prevRevertPoint *uint64 // used to get value from outside of staged sync after cycle (for example to notify RPCDaemon)
	invalidBlock    InvalidBlock
	currentStage    uint
	LogProgress     bool
	currentCycle    SyncCycle // current cycle
	stages          []*Stage
	revertOrder     []*Stage
	pruningOrder    []*Stage
	timings         []Timing
	logPrefixes     []string

	evtDownloadFinished           event.Feed // channel for each download task finished
	evtDownloadFinishedSubscribed bool
	evtDownloadStarted            event.Feed // channel for each download has started
	evtDownloadStartedSubscribed  bool
}

type Timing struct {
	isRevert  bool
	isCleanUp bool
	stage     SyncStageID
	took      time.Duration
}

type SyncCycle struct {
	Number       uint64
	TargetHeight uint64
	lock         sync.RWMutex
}

func (s *StagedStreamSync) Len() int                    { return len(s.stages) }
func (s *StagedStreamSync) Blockchain() core.BlockChain { return s.bc }
func (s *StagedStreamSync) DB() kv.RwDB                 { return s.db }
func (s *StagedStreamSync) IsBeacon() bool              { return s.isBeacon }
func (s *StagedStreamSync) IsExplorer() bool            { return s.isExplorer }
func (s *StagedStreamSync) LogPrefix() string {
	if s == nil {
		return ""
	}
	return s.logPrefixes[s.currentStage]
}
func (s *StagedStreamSync) PrevRevertPoint() *uint64 { return s.prevRevertPoint }

func (s *StagedStreamSync) NewRevertState(id SyncStageID, revertPoint uint64) *RevertState {
	return &RevertState{id, revertPoint, s}
}

func (s *StagedStreamSync) CleanUpStageState(ctx context.Context, id SyncStageID, forwardProgress uint64, tx kv.Tx, db kv.RwDB) (*CleanUpState, error) {
	var pruneProgress uint64
	var err error

	if errV := CreateView(ctx, db, tx, func(tx kv.Tx) error {
		pruneProgress, err = GetStageCleanUpProgress(tx, id, s.isBeacon)
		if err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return nil, errV
	}

	return &CleanUpState{id, forwardProgress, pruneProgress, s}, nil
}

func (s *StagedStreamSync) NextStage() {
	if s == nil {
		return
	}
	s.currentStage++
}

// IsBefore returns true if stage1 goes before stage2 in staged sync
func (s *StagedStreamSync) IsBefore(stage1, stage2 SyncStageID) bool {
	idx1 := -1
	idx2 := -1
	for i, stage := range s.stages {
		if stage.ID == stage1 {
			idx1 = i
		}

		if stage.ID == stage2 {
			idx2 = i
		}
	}

	return idx1 < idx2
}

// IsAfter returns true if stage1 goes after stage2 in staged sync
func (s *StagedStreamSync) IsAfter(stage1, stage2 SyncStageID) bool {
	idx1 := -1
	idx2 := -1
	for i, stage := range s.stages {
		if stage.ID == stage1 {
			idx1 = i
		}

		if stage.ID == stage2 {
			idx2 = i
		}
	}

	return idx1 > idx2
}

// RevertTo sets the revert point
func (s *StagedStreamSync) RevertTo(revertPoint uint64, invalidBlockNumber uint64, invalidBlockHash common.Hash, invalidBlockStreamID sttypes.StreamID) {
	utils.Logger().Info().
		Uint64("invalidBlockNumber", invalidBlockNumber).
		Interface("invalidBlockHash", invalidBlockHash).
		Interface("invalidBlockStreamID", invalidBlockStreamID).
		Uint64("revertPoint", revertPoint).
		Msgf(WrapStagedSyncMsg("Reverting blocks"))
	s.revertPoint = &revertPoint
	if invalidBlockNumber > 0 || invalidBlockHash != (common.Hash{}) {
		resetBadStreams := !s.invalidBlock.Active
		s.invalidBlock.set(invalidBlockNumber, invalidBlockHash, resetBadStreams)
		s.invalidBlock.addBadStream(invalidBlockStreamID)
	}
}

func (s *StagedStreamSync) Done() {
	s.currentStage = uint(len(s.stages))
	s.revertPoint = nil
}

// IsDone returns true if last stage have been done
func (s *StagedStreamSync) IsDone() bool {
	return s.currentStage >= uint(len(s.stages)) && s.revertPoint == nil
}

// SetCurrentStage sets the current stage to a given stage id
func (s *StagedStreamSync) SetCurrentStage(id SyncStageID) error {
	for i, stage := range s.stages {
		if stage.ID == id {
			s.currentStage = uint(i)
			return nil
		}
	}

	return ErrStageNotFound
}

// StageState retrieves the latest stage state from db
func (s *StagedStreamSync) StageState(ctx context.Context, stage SyncStageID, tx kv.Tx, db kv.RwDB) (*StageState, error) {
	var blockNum uint64
	var err error
	if errV := CreateView(ctx, db, tx, func(rtx kv.Tx) error {
		blockNum, err = GetStageProgress(rtx, stage, s.isBeacon)
		if err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return nil, errV
	}

	return &StageState{s, stage, blockNum}, nil
}

// cleanUp cleans up the stage by calling pruneStage
func (s *StagedStreamSync) cleanUp(ctx context.Context, fromStage int, db kv.RwDB, tx kv.RwTx, firstCycle bool) error {
	found := false
	for i := 0; i < len(s.pruningOrder); i++ {
		if s.pruningOrder[i].ID == s.stages[fromStage].ID {
			found = true
		}
		if !found || s.pruningOrder[i] == nil || s.pruningOrder[i].Disabled {
			continue
		}
		if err := s.pruneStage(ctx, firstCycle, s.pruningOrder[i], db, tx); err != nil {
			panic(err)
		}
	}
	return nil
}

// New creates a new StagedStreamSync instance
func New(
	bc core.BlockChain,
	db kv.RwDB,
	stagesList []*Stage,
	isBeacon bool,
	protocol syncProtocol,
	isBeaconNode bool,
	useMemDB bool,
	config Config,
	logger zerolog.Logger,
) *StagedStreamSync {

	revertStages := make([]*Stage, len(stagesList))
	for i, stageIndex := range DefaultRevertOrder {
		for _, s := range stagesList {
			if s.ID == stageIndex {
				revertStages[i] = s
				break
			}
		}
	}
	pruneStages := make([]*Stage, len(stagesList))
	for i, stageIndex := range DefaultCleanUpOrder {
		for _, s := range stagesList {
			if s.ID == stageIndex {
				pruneStages[i] = s
				break
			}
		}
	}

	logPrefixes := make([]string, len(stagesList))
	for i := range stagesList {
		logPrefixes[i] = fmt.Sprintf("%d/%d %s", i+1, len(stagesList), stagesList[i].ID)
	}

	status := newStatus()

	return &StagedStreamSync{
		bc:           bc,
		isBeacon:     isBeacon,
		db:           db,
		protocol:     protocol,
		isBeaconNode: isBeaconNode,
		gbm:          nil,
		status:       &status,
		inserted:     0,
		config:       config,
		logger:       logger,
		stages:       stagesList,
		currentStage: 0,
		revertOrder:  revertStages,
		pruningOrder: pruneStages,
		logPrefixes:  logPrefixes,
		UseMemDB:     useMemDB,
	}
}

// doGetCurrentNumberRequest returns estimated current block number and corresponding stream
func (s *StagedStreamSync) doGetCurrentNumberRequest(ctx context.Context) (uint64, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	bn, stid, err := s.protocol.GetCurrentBlockNumber(ctx, syncproto.WithHighPriority())
	if err != nil {
		return 0, stid, err
	}
	return bn, stid, nil
}

// promLabels returns a prometheus labels for current shard id
func (s *StagedStreamSync) promLabels() prometheus.Labels {
	sid := s.bc.ShardID()
	return prometheus.Labels{"ShardID": fmt.Sprintf("%d", sid)}
}

// checkHaveEnoughStreams checks whether node is connected to certain number of streams
func (s *StagedStreamSync) checkHaveEnoughStreams() error {
	numStreams := s.protocol.NumStreams()
	if numStreams < s.config.MinStreams {
		return fmt.Errorf("number of streams smaller than minimum: %v < %v",
			numStreams, s.config.MinStreams)
	}
	return nil
}

// Run runs a full cycle of stages
func (s *StagedStreamSync) Run(ctx context.Context, db kv.RwDB, tx kv.RwTx, firstCycle bool) error {
	s.prevRevertPoint = nil
	s.timings = s.timings[:0]

	for !s.IsDone() {
		if s.revertPoint != nil {
			s.prevRevertPoint = s.revertPoint
			s.revertPoint = nil
			if !s.invalidBlock.Active {
				for j := 0; j < len(s.revertOrder); j++ {
					if s.revertOrder[j] == nil || s.revertOrder[j].Disabled {
						continue
					}
					if err := s.revertStage(ctx, firstCycle, s.revertOrder[j], db, tx); err != nil {
						utils.Logger().Error().
							Err(err).
							Interface("stage id", s.revertOrder[j].ID).
							Msgf(WrapStagedSyncMsg("revert stage failed"))
						return err
					}
				}
			}
			if err := s.SetCurrentStage(s.stages[0].ID); err != nil {
				return err
			}
			firstCycle = false
		}

		stage := s.stages[s.currentStage]

		if stage.Disabled {
			utils.Logger().Trace().
				Msgf(WrapStagedSyncMsg(fmt.Sprintf("%s disabled. %s", stage.ID, stage.DisabledDescription)))

			s.NextStage()
			continue
		}

		if err := s.runStage(ctx, stage, db, tx, firstCycle, s.invalidBlock.Active); err != nil {
			utils.Logger().Error().
				Err(err).
				Interface("stage id", stage.ID).
				Msgf(WrapStagedSyncMsg("stage failed"))
			return err
		}
		s.NextStage()
	}

	if err := s.cleanUp(ctx, 0, db, tx, firstCycle); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf(WrapStagedSyncMsg("stages cleanup failed"))
		return err
	}
	if err := s.SetCurrentStage(s.stages[0].ID); err != nil {
		return err
	}
	if err := printLogs(tx, s.timings); err != nil {
		utils.Logger().Warn().Err(err).Msg("print timing logs failed")
	}
	s.currentStage = 0
	return nil
}

// CreateView creates a view for a given db
func CreateView(ctx context.Context, db kv.RwDB, tx kv.Tx, f func(tx kv.Tx) error) error {
	if tx != nil {
		return f(tx)
	}
	return db.View(ctx, func(etx kv.Tx) error {
		return f(etx)
	})
}

// printLogs prints all timing logs
func printLogs(tx kv.RwTx, timings []Timing) error {
	var logCtx []interface{}
	count := 0
	for i := range timings {
		if timings[i].took < 50*time.Millisecond {
			continue
		}
		count++
		if count == 50 {
			break
		}
		if timings[i].isRevert {
			logCtx = append(logCtx, "Revert "+string(timings[i].stage), timings[i].took.Truncate(time.Millisecond).String())
		} else if timings[i].isCleanUp {
			logCtx = append(logCtx, "CleanUp "+string(timings[i].stage), timings[i].took.Truncate(time.Millisecond).String())
		} else {
			logCtx = append(logCtx, string(timings[i].stage), timings[i].took.Truncate(time.Millisecond).String())
		}
	}
	if len(logCtx) > 0 {
		timingLog := fmt.Sprintf("Timings (slower than 50ms) %v", logCtx)
		utils.Logger().Info().Msgf(WrapStagedSyncMsg(timingLog))
	}

	if tx == nil {
		return nil
	}

	if len(logCtx) > 0 { // also don't print this logs if everything is fast
		buckets := Buckets
		bucketSizes := make([]interface{}, 0, 2*len(buckets))
		for _, bucket := range buckets {
			sz, err1 := tx.BucketSize(bucket)
			if err1 != nil {
				return err1
			}
			bucketSizes = append(bucketSizes, bucket, ByteCount(sz))
		}
		utils.Logger().Info().
			Msgf(WrapStagedSyncMsg(fmt.Sprintf("Tables %v", bucketSizes...)))
	}
	tx.CollectMetrics()
	return nil
}

// runStage executes stage
func (s *StagedStreamSync) runStage(ctx context.Context, stage *Stage, db kv.RwDB, tx kv.RwTx, firstCycle bool, invalidBlockRevert bool) (err error) {
	start := time.Now()
	stageState, err := s.StageState(ctx, stage.ID, tx, db)
	if err != nil {
		return err
	}

	if err = stage.Handler.Exec(ctx, firstCycle, invalidBlockRevert, stageState, s, tx); err != nil {
		utils.Logger().Error().
			Err(err).
			Interface("stage id", stage.ID).
			Msgf(WrapStagedSyncMsg("stage failed"))
		return fmt.Errorf("[%s] %w", s.LogPrefix(), err)
	}

	took := time.Since(start)
	if took > 60*time.Second {
		logPrefix := s.LogPrefix()
		utils.Logger().Info().
			Msgf(WrapStagedSyncMsg(fmt.Sprintf("%s:  DONE in %d", logPrefix, took)))

	}
	s.timings = append(s.timings, Timing{stage: stage.ID, took: took})
	return nil
}

// revertStage reverts stage
func (s *StagedStreamSync) revertStage(ctx context.Context, firstCycle bool, stage *Stage, db kv.RwDB, tx kv.RwTx) error {
	start := time.Now()
	stageState, err := s.StageState(ctx, stage.ID, tx, db)
	if err != nil {
		return err
	}

	revert := s.NewRevertState(stage.ID, *s.revertPoint)

	if stageState.BlockNumber <= revert.RevertPoint {
		return nil
	}

	if err = s.SetCurrentStage(stage.ID); err != nil {
		return err
	}

	err = stage.Handler.Revert(ctx, firstCycle, revert, stageState, tx)
	if err != nil {
		return fmt.Errorf("[%s] %w", s.LogPrefix(), err)
	}

	took := time.Since(start)
	if took > 60*time.Second {
		logPrefix := s.LogPrefix()
		utils.Logger().Info().
			Msgf(WrapStagedSyncMsg(fmt.Sprintf("%s: Revert done in %d", logPrefix, took)))
	}
	s.timings = append(s.timings, Timing{isRevert: true, stage: stage.ID, took: took})
	return nil
}

// pruneStage cleans up the stage and logs the timing
func (s *StagedStreamSync) pruneStage(ctx context.Context, firstCycle bool, stage *Stage, db kv.RwDB, tx kv.RwTx) error {
	start := time.Now()

	stageState, err := s.StageState(ctx, stage.ID, tx, db)
	if err != nil {
		return err
	}

	prune, err := s.CleanUpStageState(ctx, stage.ID, stageState.BlockNumber, tx, db)
	if err != nil {
		return err
	}
	if err = s.SetCurrentStage(stage.ID); err != nil {
		return err
	}

	err = stage.Handler.CleanUp(ctx, firstCycle, prune, tx)
	if err != nil {
		return fmt.Errorf("[%s] %w", s.LogPrefix(), err)
	}

	took := time.Since(start)
	if took > 60*time.Second {
		logPrefix := s.LogPrefix()
		utils.Logger().Info().
			Msgf(WrapStagedSyncMsg(fmt.Sprintf("%s: CleanUp done in %d", logPrefix, took)))
	}
	s.timings = append(s.timings, Timing{isCleanUp: true, stage: stage.ID, took: took})
	return nil
}

// DisableAllStages disables all stages including their reverts
func (s *StagedStreamSync) DisableAllStages() []SyncStageID {
	var backupEnabledIds []SyncStageID
	for i := range s.stages {
		if !s.stages[i].Disabled {
			backupEnabledIds = append(backupEnabledIds, s.stages[i].ID)
		}
	}
	for i := range s.stages {
		s.stages[i].Disabled = true
	}
	return backupEnabledIds
}

// DisableStages disables stages by a set of given stage IDs
func (s *StagedStreamSync) DisableStages(ids ...SyncStageID) {
	for i := range s.stages {
		for _, id := range ids {
			if s.stages[i].ID != id {
				continue
			}
			s.stages[i].Disabled = true
		}
	}
}

// EnableStages enables stages by a set of given stage IDs
func (s *StagedStreamSync) EnableStages(ids ...SyncStageID) {
	for i := range s.stages {
		for _, id := range ids {
			if s.stages[i].ID != id {
				continue
			}
			s.stages[i].Disabled = false
		}
	}
}
