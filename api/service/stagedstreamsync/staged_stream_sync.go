package stagedstreamsync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/utils"
	syncproto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/harmony-one/harmony/shard"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
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
	bc                core.BlockChain
	consensus         *consensus.Consensus
	db                kv.RwDB
	protocol          syncProtocol
	gbm               *downloadManager // initialized when finished get block number
	lastMileBlocks    []*types.Block   // last mile blocks to catch up with the consensus
	lastMileMux       sync.Mutex
	isEpochChain      bool
	isBeaconValidator bool
	isBeaconShard     bool
	isExplorer        bool
	isValidator       bool
	joinConsensus     bool
	inserted          int
	config            Config
	logger            zerolog.Logger
	status            *status //TODO: merge this with currentSyncCycle
	initSync          bool    // if sets to true, node start long range syncing
	UseMemDB          bool
	revertPoint       *uint64 // used to run stages
	prevRevertPoint   *uint64 // used to get value from outside of staged sync after cycle (for example to notify RPCDaemon)
	invalidBlock      InvalidBlock
	currentStage      uint
	LogProgress       bool
	currentCycle      SyncCycle // current cycle
	stages            []*Stage
	revertOrder       []*Stage
	pruningOrder      []*Stage
	timings           []Timing
	logPrefixes       []string

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
	BlockNumber  uint64
	TargetHeight uint64
	lock         sync.RWMutex
}

// GetBlockNumber returns the current sync block number.
func (sc *SyncCycle) GetBlockNumber() uint64 {
	sc.lock.RLock()
	defer sc.lock.RUnlock()
	return sc.BlockNumber
}

// SetBlockNumber sets the sync block number
func (sc *SyncCycle) SetBlockNumber(number uint64) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	sc.BlockNumber = number
}

// AddBlockNumber adds inc to the sync block number
func (sc *SyncCycle) AddBlockNumber(inc uint64) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	sc.BlockNumber += inc
}

// GetTargetHeight returns the current target height
func (sc *SyncCycle) GetTargetHeight() uint64 {
	sc.lock.RLock()
	defer sc.lock.RUnlock()
	return sc.TargetHeight
}

// SetTargetHeight sets the target height
func (sc *SyncCycle) SetTargetHeight(height uint64) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	sc.TargetHeight = height
}

func (sss *StagedStreamSync) Len() int                    { return len(sss.stages) }
func (sss *StagedStreamSync) Blockchain() core.BlockChain { return sss.bc }
func (sss *StagedStreamSync) DB() kv.RwDB                 { return sss.db }
func (sss *StagedStreamSync) IsBeacon() bool              { return sss.isBeaconShard }
func (sss *StagedStreamSync) IsExplorer() bool            { return sss.isExplorer }
func (sss *StagedStreamSync) LogPrefix() string {
	if sss == nil {
		return ""
	}
	return sss.logPrefixes[sss.currentStage]
}
func (sss *StagedStreamSync) PrevRevertPoint() *uint64 { return sss.prevRevertPoint }

func (sss *StagedStreamSync) NewRevertState(id SyncStageID, revertPoint uint64) *RevertState {
	return &RevertState{id, revertPoint, sss}
}

func (sss *StagedStreamSync) CleanUpStageState(ctx context.Context, id SyncStageID, forwardProgress uint64, tx kv.Tx, db kv.RwDB) (*CleanUpState, error) {
	var pruneProgress uint64
	var err error

	if errV := CreateView(ctx, db, tx, func(tx kv.Tx) error {
		pruneProgress, err = GetStageCleanUpProgress(tx, id, sss.isBeaconShard)
		if err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return nil, errV
	}

	return &CleanUpState{id, forwardProgress, pruneProgress, sss}, nil
}

func (sss *StagedStreamSync) NextStage() {
	if sss == nil {
		return
	}
	sss.currentStage++
}

// IsBefore returns true if stage1 goes before stage2 in staged sync
func (sss *StagedStreamSync) IsBefore(stage1, stage2 SyncStageID) bool {
	idx1 := -1
	idx2 := -1
	for i, stage := range sss.stages {
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
func (sss *StagedStreamSync) IsAfter(stage1, stage2 SyncStageID) bool {
	idx1 := -1
	idx2 := -1
	for i, stage := range sss.stages {
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
func (sss *StagedStreamSync) RevertTo(revertPoint uint64, invalidBlockNumber uint64, invalidBlockHash common.Hash, invalidBlockStreamID sttypes.StreamID) {
	sss.logger.Info().
		Uint64("invalidBlockNumber", invalidBlockNumber).
		Interface("invalidBlockHash", invalidBlockHash).
		Interface("invalidBlockStreamID", invalidBlockStreamID).
		Uint64("revertPoint", revertPoint).
		Msgf(WrapStagedSyncMsg("Reverting blocks"))
	sss.revertPoint = &revertPoint
	if invalidBlockNumber > 0 || invalidBlockHash != (common.Hash{}) {
		resetBadStreams := !sss.invalidBlock.Active
		sss.invalidBlock.set(invalidBlockNumber, invalidBlockHash, resetBadStreams)
		sss.invalidBlock.addBadStream(invalidBlockStreamID)
	}
}

func (sss *StagedStreamSync) Done() {
	sss.currentStage = uint(len(sss.stages))
	sss.revertPoint = nil
}

// IsDone returns true if last stage have been done
func (sss *StagedStreamSync) IsDone() bool {
	return sss.currentStage >= uint(len(sss.stages)) && sss.revertPoint == nil
}

// SetCurrentStage sets the current stage to a given stage id
func (sss *StagedStreamSync) SetCurrentStage(id SyncStageID) error {
	for i, stage := range sss.stages {
		if stage.ID == id {
			sss.currentStage = uint(i)
			return nil
		}
	}

	return ErrStageNotFound
}

// StageState retrieves the latest stage state from db
func (sss *StagedStreamSync) StageState(ctx context.Context, stage SyncStageID, tx kv.Tx, db kv.RwDB) (*StageState, error) {
	var blockNum uint64
	var err error
	if errV := CreateView(ctx, db, tx, func(rtx kv.Tx) error {
		blockNum, err = GetStageProgress(rtx, stage, sss.isBeaconShard)
		if err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return nil, errV
	}

	return &StageState{sss, stage, blockNum}, nil
}

// cleanUp cleans up the stage by calling pruneStage
func (sss *StagedStreamSync) cleanUp(ctx context.Context, fromStage int, db kv.RwDB, tx kv.RwTx, firstCycle bool) error {
	found := false
	for i := 0; i < len(sss.pruningOrder); i++ {
		if sss.pruningOrder[i].ID == sss.stages[fromStage].ID {
			found = true
		}
		if !found || sss.pruningOrder[i] == nil || sss.pruningOrder[i].Disabled {
			continue
		}
		if err := sss.pruneStage(ctx, firstCycle, sss.pruningOrder[i], db, tx); err != nil {
			sss.logger.Error().Err(err).
				Interface("stage id", sss.pruningOrder[i].ID).
				Msgf(WrapStagedSyncMsg("stage cleanup failed"))
			panic(err)
		}
	}
	return nil
}

// New creates a new StagedStreamSync instance
func New(
	bc core.BlockChain,
	consensus *consensus.Consensus,
	db kv.RwDB,
	stagesList []*Stage,
	isEpochChain bool,
	isBeaconShard bool,
	protocol syncProtocol,
	isBeaconValidator bool,
	isExplorer bool,
	isValidator bool,
	joinConsensus bool,
	config Config,
	logger zerolog.Logger,
) *StagedStreamSync {

	forwardStages := make([]*Stage, len(StagesForwardOrder))
	for i, stageIndex := range StagesForwardOrder {
		for _, s := range stagesList {
			if s.ID == stageIndex {
				forwardStages[i] = s
				break
			}
		}
	}

	revertStages := make([]*Stage, len(StagesRevertOrder))
	for i, stageIndex := range StagesRevertOrder {
		for _, s := range stagesList {
			if s.ID == stageIndex {
				revertStages[i] = s
				break
			}
		}
	}

	pruneStages := make([]*Stage, len(StagesCleanUpOrder))
	for i, stageIndex := range StagesCleanUpOrder {
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

	status := NewStatus()

	return &StagedStreamSync{
		bc:                bc,
		consensus:         consensus,
		isBeaconShard:     isBeaconShard,
		db:                db,
		protocol:          protocol,
		isEpochChain:      isEpochChain,
		isBeaconValidator: isBeaconValidator,
		isExplorer:        isExplorer,
		isValidator:       isValidator,
		joinConsensus:     joinConsensus,
		lastMileBlocks:    []*types.Block{},
		gbm:               nil,
		status:            status,
		inserted:          0,
		config:            config,
		logger:            logger,
		stages:            forwardStages,
		currentStage:      0,
		revertOrder:       revertStages,
		pruningOrder:      pruneStages,
		logPrefixes:       logPrefixes,
		UseMemDB:          config.UseMemDB,
	}
}

// doGetCurrentNumberRequest returns estimated current block number and corresponding stream
func (sss *StagedStreamSync) doGetCurrentNumberRequest(ctx context.Context) (uint64, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	bn, stid, err := sss.protocol.GetCurrentBlockNumber(ctx, syncproto.WithHighPriority())
	if err != nil {
		return 0, stid, err
	}
	return bn, stid, nil
}

// doGetBlockByNumberRequest returns block by its number and corresponding stream
func (sss *StagedStreamSync) doGetBlockByNumberRequest(ctx context.Context, bn uint64) (*types.Block, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	blocks, stid, err := sss.protocol.GetBlocksByNumber(ctx, []uint64{bn}, syncproto.WithHighPriority())
	if err != nil || len(blocks) != 1 {
		return nil, stid, err
	}
	return blocks[0], stid, nil
}

// promLabels returns a prometheus labels for current shard id
func (sss *StagedStreamSync) promLabels() prometheus.Labels {
	sid := sss.bc.ShardID()
	return prometheus.Labels{"ShardID": fmt.Sprintf("%d", sid)}
}

// checkHaveEnoughStreams checks whether node is connected to certain number of streams
func (sss *StagedStreamSync) checkHaveEnoughStreams() error {
	numStreams := sss.protocol.NumStreams()
	if numStreams < sss.config.MinStreams {
		sss.logger.Debug().Msgf("number of streams smaller than minimum: %v < %v",
			numStreams, sss.config.MinStreams)
		return ErrNotEnoughStreams
	}
	return nil
}

// Run runs a full cycle of stages
func (sss *StagedStreamSync) Run(ctx context.Context, db kv.RwDB, tx kv.RwTx, firstCycle bool) error {
	sss.prevRevertPoint = nil
	sss.timings = sss.timings[:0]

	for !sss.IsDone() {
		if sss.revertPoint != nil {
			sss.prevRevertPoint = sss.revertPoint
			sss.revertPoint = nil
			if !sss.invalidBlock.Active {
				for j := 0; j < len(sss.revertOrder); j++ {
					if sss.revertOrder[j] == nil || sss.revertOrder[j].Disabled {
						continue
					}
					if err := sss.revertStage(ctx, firstCycle, sss.revertOrder[j], db, tx); err != nil {
						sss.logger.Error().
							Err(err).
							Interface("stage id", sss.revertOrder[j].ID).
							Msgf(WrapStagedSyncMsg("revert stage failed"))
						return err
					}
				}
			}
			if err := sss.SetCurrentStage(sss.stages[0].ID); err != nil {
				return err
			}
			firstCycle = false
		}

		stage := sss.stages[sss.currentStage]

		if stage.Disabled {
			sss.logger.Trace().
				Msgf(WrapStagedSyncMsg(fmt.Sprintf("%s disabled. %s", stage.ID, stage.DisabledDescription)))

			sss.NextStage()
			continue
		}

		// TODO: enable this part after make sure all works well
		// if !sss.canExecute(stage) {
		// 	continue
		// }

		if err := sss.runStage(ctx, stage, db, tx, firstCycle, sss.invalidBlock.Active); err != nil {
			sss.logger.Error().
				Err(err).
				Interface("stage id", stage.ID).
				Msgf(WrapStagedSyncMsg("stage failed"))
			return err
		}
		sss.NextStage()
	}

	if err := sss.cleanUp(ctx, 0, db, tx, firstCycle); err != nil {
		sss.logger.Error().
			Err(err).
			Msgf(WrapStagedSyncMsg("stages cleanup failed"))
		return err
	}
	if err := sss.SetCurrentStage(sss.stages[0].ID); err != nil {
		return err
	}
	if err := printLogs(tx, sss.timings); err != nil {
		sss.logger.Warn().Err(err).Msg("print timing logs failed")
	}
	sss.currentStage = 0
	return nil
}

func (sss *StagedStreamSync) canExecute(stage *Stage) bool {
	// check range mode
	if stage.RangeMode != LongRangeAndShortRange {
		isLongRange := sss.initSync
		switch stage.RangeMode {
		case OnlyLongRange:
			if !isLongRange {
				return false
			}
		case OnlyShortRange:
			if isLongRange {
				return false
			}
		default:
			return false
		}
	}

	// check chain execution
	if stage.ChainExecutionMode != AllChains {
		shardID := sss.bc.ShardID()
		isBeaconValidator := sss.isBeaconValidator
		isShardChain := shardID != shard.BeaconChainShardID
		isEpochChain := sss.isEpochChain
		switch stage.ChainExecutionMode {
		case AllChainsExceptEpochChain:
			if isEpochChain {
				return false
			}
		case OnlyBeaconNode:
			if !isBeaconValidator {
				return false
			}
		case OnlyShardChain:
			if !isShardChain {
				return false
			}
		case OnlyEpochChain:
			if !isEpochChain {
				return false
			}
		default:
			return false
		}
	}

	return true
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
func (sss *StagedStreamSync) runStage(ctx context.Context, stage *Stage, db kv.RwDB, tx kv.RwTx, firstCycle bool, invalidBlockRevert bool) (err error) {
	start := time.Now()
	stageState, err := sss.StageState(ctx, stage.ID, tx, db)
	if err != nil {
		return err
	}
	if err = stage.Handler.Exec(ctx, firstCycle, invalidBlockRevert, stageState, sss, tx); err != nil {
		sss.logger.Error().
			Err(err).
			Interface("stage id", stage.ID).
			Msgf(WrapStagedSyncMsg("stage failed"))
		return fmt.Errorf("[%s] %w", sss.LogPrefix(), err)
	}

	took := time.Since(start)
	if took > 60*time.Second {
		logPrefix := sss.LogPrefix()
		sss.logger.Info().
			Msgf(WrapStagedSyncMsg(fmt.Sprintf("%s:  DONE in %d", logPrefix, took)))

	}
	sss.timings = append(sss.timings, Timing{stage: stage.ID, took: took})
	return nil
}

// revertStage reverts stage
func (sss *StagedStreamSync) revertStage(ctx context.Context, firstCycle bool, stage *Stage, db kv.RwDB, tx kv.RwTx) error {
	start := time.Now()
	stageState, err := sss.StageState(ctx, stage.ID, tx, db)
	if err != nil {
		return err
	}

	revert := sss.NewRevertState(stage.ID, *sss.revertPoint)

	if stageState.BlockNumber <= revert.RevertPoint {
		return nil
	}

	if err = sss.SetCurrentStage(stage.ID); err != nil {
		return err
	}

	err = stage.Handler.Revert(ctx, firstCycle, revert, stageState, tx)
	if err != nil {
		return fmt.Errorf("[%s] %w", sss.LogPrefix(), err)
	}

	took := time.Since(start)
	if took > 60*time.Second {
		logPrefix := sss.LogPrefix()
		sss.logger.Info().
			Msgf(WrapStagedSyncMsg(fmt.Sprintf("%s: Revert done in %d", logPrefix, took)))
	}
	sss.timings = append(sss.timings, Timing{isRevert: true, stage: stage.ID, took: took})
	return nil
}

// pruneStage cleans up the stage and logs the timing
func (sss *StagedStreamSync) pruneStage(ctx context.Context, firstCycle bool, stage *Stage, db kv.RwDB, tx kv.RwTx) error {
	start := time.Now()

	stageState, err := sss.StageState(ctx, stage.ID, tx, db)
	if err != nil {
		return err
	}

	prune, err := sss.CleanUpStageState(ctx, stage.ID, stageState.BlockNumber, tx, db)
	if err != nil {
		return err
	}
	if err = sss.SetCurrentStage(stage.ID); err != nil {
		return err
	}

	err = stage.Handler.CleanUp(ctx, firstCycle, prune, tx)
	if err != nil {
		return fmt.Errorf("[%s] %w", sss.LogPrefix(), err)
	}

	took := time.Since(start)
	if took > 60*time.Second {
		logPrefix := sss.LogPrefix()
		sss.logger.Info().
			Msgf(WrapStagedSyncMsg(fmt.Sprintf("%s: CleanUp done in %d", logPrefix, took)))
	}
	sss.timings = append(sss.timings, Timing{isCleanUp: true, stage: stage.ID, took: took})
	return nil
}

// DisableAllStages disables all stages including their reverts
func (sss *StagedStreamSync) DisableAllStages() []SyncStageID {
	var backupEnabledIds []SyncStageID
	for i := range sss.stages {
		if !sss.stages[i].Disabled {
			backupEnabledIds = append(backupEnabledIds, sss.stages[i].ID)
		}
	}
	for i := range sss.stages {
		sss.stages[i].Disabled = true
	}
	return backupEnabledIds
}

// DisableStages disables stages by a set of given stage IDs
func (sss *StagedStreamSync) DisableStages(ids ...SyncStageID) {
	for i := range sss.stages {
		for _, id := range ids {
			if sss.stages[i].ID != id {
				continue
			}
			sss.stages[i].Disabled = true
		}
	}
}

// EnableStages enables stages by a set of given stage IDs
func (sss *StagedStreamSync) EnableStages(ids ...SyncStageID) {
	for i := range sss.stages {
		for _, id := range ids {
			if sss.stages[i].ID != id {
				continue
			}
			sss.stages[i].Disabled = false
		}
	}
}

func (sss *StagedStreamSync) purgeLastMileBlocksFromCache() {
	sss.lastMileMux.Lock()
	sss.lastMileBlocks = nil
	sss.lastMileMux.Unlock()
}

// AddLastMileBlock adds the latest a few block into queue for syncing
// only keep the latest blocks with size capped by LastMileBlocksSize
func (sss *StagedStreamSync) AddLastMileBlock(block *types.Block) {
	sss.lastMileMux.Lock()
	defer sss.lastMileMux.Unlock()
	if sss.lastMileBlocks != nil {
		if len(sss.lastMileBlocks) >= LastMileBlocksSize {
			sss.lastMileBlocks = sss.lastMileBlocks[1:]
		}
		sss.lastMileBlocks = append(sss.lastMileBlocks, block)
	}
}

func (sss *StagedStreamSync) getBlockFromLastMileBlocksByParentHash(parentHash common.Hash) *types.Block {
	sss.lastMileMux.Lock()
	defer sss.lastMileMux.Unlock()
	for _, block := range sss.lastMileBlocks {
		ph := block.ParentHash()
		if ph == parentHash {
			return block
		}
	}
	return nil
}

func (sss *StagedStreamSync) addConsensusLastMile(bc core.BlockChain, cs *consensus.Consensus) ([]common.Hash, error) {
	curNumber := bc.CurrentBlock().NumberU64()
	var hashes []common.Hash

	err := cs.GetLastMileBlockIter(curNumber+1, func(blockIter *consensus.LastMileBlockIter) error {
		for {
			block := blockIter.Next()
			if block == nil {
				break
			}
			_, err := bc.InsertChain(types.Blocks{block}, true)
			switch {
			case errors.Is(err, core.ErrKnownBlock):
			case errors.Is(err, core.ErrNotLastBlockInEpoch):
			case err != nil:
				return errors.Wrap(err, "failed to InsertChain")
			default:
				hashes = append(hashes, block.Header().Hash())
			}
		}
		return nil
	})
	return hashes, err
}

func (sss *StagedStreamSync) RollbackLastMileBlocks(ctx context.Context, hashes []common.Hash) error {
	if len(hashes) == 0 {
		return nil
	}
	sss.logger.Info().
		Interface("block", sss.bc.CurrentBlock()).
		Msg("[STAGED_STREAM_SYNC] Rolling back last mile blocks")
	if err := sss.bc.Rollback(hashes); err != nil {
		sss.logger.Error().Err(err).
			Msg("[STAGED_STREAM_SYNC] failed to rollback last mile blocks")
		return err
	}
	return nil
}

// UpdateBlockAndStatus updates block and its status in db
func (sss *StagedStreamSync) UpdateBlockAndStatus(block *types.Block, bc core.BlockChain, verifyAllSig bool) error {
	if block.NumberU64() != bc.CurrentBlock().NumberU64()+1 {
		sss.logger.Debug().
			Uint64("curBlockNum", bc.CurrentBlock().NumberU64()).
			Uint64("receivedBlockNum", block.NumberU64()).
			Msg("[STAGED_STREAM_SYNC] Inappropriate block number, ignore!")
		return nil
	}

	haveCurrentSig := len(block.GetCurrentCommitSig()) != 0
	// Verify block signatures
	if block.NumberU64() > 1 {
		// Verify signature every N blocks (which N is verifyHeaderBatchSize and can be adjusted in configs)
		verifySeal := block.NumberU64()%VerifyHeaderBatchSize == 0 || verifyAllSig
		verifyCurrentSig := verifyAllSig && haveCurrentSig
		if verifyCurrentSig {
			sig, bitmap, err := chain.ParseCommitSigAndBitmap(block.GetCurrentCommitSig())
			if err != nil {
				return errors.Wrap(err, "parse commitSigAndBitmap")
			}

			startTime := time.Now()
			if err := bc.Engine().VerifyHeaderSignature(bc, block.Header(), sig, bitmap); err != nil {
				return errors.Wrapf(err, "verify header signature %v", block.Hash().String())
			}
			sss.logger.Debug().
				Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).
				Msg("[STAGED_STREAM_SYNC] VerifyHeaderSignature")
		}
		err := bc.Engine().VerifyHeader(bc, block.Header(), verifySeal)
		if err == engine.ErrUnknownAncestor {
			return nil
		} else if err != nil {
			sss.logger.Error().
				Err(err).
				Uint64("block number", block.NumberU64()).
				Msgf("[STAGED_STREAM_SYNC] UpdateBlockAndStatus: failed verifying signatures for new block")
			return err
		}
	}

	_, err := bc.InsertChain([]*types.Block{block}, false /* verifyHeaders */)
	switch {
	case errors.Is(err, core.ErrKnownBlock):
	case err != nil:
		sss.logger.Error().
			Err(err).
			Uint64("block number", block.NumberU64()).
			Uint32("shard", block.ShardID()).
			Msgf("[STAGED_STREAM_SYNC] UpdateBlockAndStatus: Error adding new block to blockchain")
		return err
	default:
	}
	sss.logger.Info().
		Uint64("blockHeight", block.NumberU64()).
		Uint64("blockEpoch", block.Epoch().Uint64()).
		Str("blockHex", block.Hash().Hex()).
		Msg("[STAGED_STREAM_SYNC] UpdateBlockAndStatus: New Block Added to Blockchain")

	for i, tx := range block.StakingTransactions() {
		sss.logger.Info().Msgf("StakingTxn %d: %s, %v", i, tx.StakingType().String(), tx.StakingMessage())
	}
	return nil
}
