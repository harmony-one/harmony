package stagedsync

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/ledgerwatch/erigon-lib/kv"

	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
)

type StagedSync struct {
	selfip             string
	selfport           string
	selfPeerHash       [20]byte // hash of ip and address combination
	commonBlocks       map[int]*types.Block
	downloadedBlocks   map[uint64][]byte
	lastMileBlocks     []*types.Block // last mile blocks to catch up with the consensus
	syncConfig         *SyncConfig
	isExplorer         bool
	stateSyncTaskQueue *queue.Queue
	syncMux            sync.Mutex
	lastMileMux        sync.Mutex
	syncStatus         syncStatus
	lockBlocks         sync.RWMutex

	ctx      context.Context
	bc       core.BlockChain
	isBeacon bool
	db       kv.RwDB

	revertPoint     *uint64 // used to run stages
	prevRevertPoint *uint64 // used to get value from outside of staged sync after cycle (for example to notify RPCDaemon)
	invalidBlock    common.Hash

	stages       []*Stage
	revertOrder  []*Stage
	pruningOrder []*Stage
	currentStage uint
	timings      []Timing
	logPrefixes  []string

	// if set to true, it will double check the block hashes
	// so, only blocks are sent by 2/3 of peers are considered as valid
	DoubleCheckBlockHashes bool
	// Maximum number of blocks per each cycle. if set to zero, all blocks will be
	// downloaded and synced in one full cycle.
	MaxBlocksPerSyncCycle uint64
	// maximum number of blocks which can be downloaded in background.
	MaxBackgroundBlocks uint64
	// max number of blocks to use a single transaction for staged sync
	MaxMemSyncCycleSize uint64
	// number of blocks to build a batch and insert to chain in staged sync
	InsertChainBatchSize int
	// verify signature for all blocks regardless of height or batch size
	VerifyAllSig bool
	// batch size to verify header before insert to chain
	VerifyHeaderBatchSize uint64
	// use mem db for staged sync, set to false to use disk
	UseMemDB bool
	// use turbo mode for staged sync
	StagedSyncTurboMode bool
	// log the full sync progress in console
	LogProgress bool
	// log every single process and error to help to debug the syncing
	// DebugMode is not accessible to the end user and is only an aid for development
	DebugMode bool
}

// BlockWithSig the serialization structure for request DownloaderRequest_BLOCKWITHSIG
// The block is encoded as block + commit signature
type BlockWithSig struct {
	Block              *types.Block
	CommitSigAndBitmap []byte
}

type Timing struct {
	isRevert  bool
	isCleanUp bool
	stage     SyncStageID
	took      time.Duration
}

func (s *StagedSync) Len() int                    { return len(s.stages) }
func (s *StagedSync) Context() context.Context    { return s.ctx }
func (s *StagedSync) IsBeacon() bool              { return s.isBeacon }
func (s *StagedSync) IsExplorer() bool            { return s.isExplorer }
func (s *StagedSync) Blockchain() core.BlockChain { return s.bc }
func (s *StagedSync) DB() kv.RwDB                 { return s.db }
func (s *StagedSync) PrevRevertPoint() *uint64    { return s.prevRevertPoint }

func (s *StagedSync) InitDownloadedBlocksMap() error {
	s.lockBlocks.Lock()
	defer s.lockBlocks.Unlock()
	s.downloadedBlocks = make(map[uint64][]byte)
	return nil
}

func (s *StagedSync) AddBlocks(blks map[uint64][]byte) error {
	s.lockBlocks.Lock()
	defer s.lockBlocks.Unlock()
	for k, blkBytes := range blks {
		s.downloadedBlocks[k] = make([]byte, len(blkBytes))
		copy(s.downloadedBlocks[k], blkBytes[:])
	}
	return nil
}

func (s *StagedSync) GetDownloadedBlocks() map[uint64][]byte {
	d := make(map[uint64][]byte)
	s.lockBlocks.RLock()
	defer s.lockBlocks.RUnlock()
	for k, blkBytes := range s.downloadedBlocks {
		d[k] = make([]byte, len(blkBytes))
		copy(d[k], blkBytes[:])
	}
	return d
}

func (s *StagedSync) NewRevertState(id SyncStageID, revertPoint, currentProgress uint64) *RevertState {
	return &RevertState{id, revertPoint, currentProgress, common.Hash{}, s}
}

func (s *StagedSync) CleanUpStageState(id SyncStageID, forwardProgress uint64, tx kv.Tx, db kv.RwDB) (*CleanUpState, error) {
	var pruneProgress uint64
	var err error

	if errV := CreateView(context.Background(), db, tx, func(tx kv.Tx) error {
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

func (s *StagedSync) NextStage() {
	if s == nil {
		return
	}
	s.currentStage++
}

// IsBefore returns true if stage1 goes before stage2 in staged sync
func (s *StagedSync) IsBefore(stage1, stage2 SyncStageID) bool {
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
func (s *StagedSync) IsAfter(stage1, stage2 SyncStageID) bool {
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

// RevertTo reverts the stage to a specific height
func (s *StagedSync) RevertTo(revertPoint uint64, invalidBlock common.Hash) {
	utils.Logger().Info().
		Interface("invalidBlock", invalidBlock).
		Uint64("revertPoint", revertPoint).
		Msgf("[STAGED_SYNC] Reverting blocks")
	s.revertPoint = &revertPoint
	s.invalidBlock = invalidBlock
}

func (s *StagedSync) Done() {
	s.currentStage = uint(len(s.stages))
	s.revertPoint = nil
}

func (s *StagedSync) IsDone() bool {
	return s.currentStage >= uint(len(s.stages)) && s.revertPoint == nil
}

func (s *StagedSync) LogPrefix() string {
	if s == nil {
		return ""
	}
	return s.logPrefixes[s.currentStage]
}

func (s *StagedSync) SetCurrentStage(id SyncStageID) error {
	for i, stage := range s.stages {
		if stage.ID == id {
			s.currentStage = uint(i)
			return nil
		}
	}
	utils.Logger().Error().
		Interface("stage id", id).
		Msgf("[STAGED_SYNC] stage not found")

	return ErrStageNotFound
}

func New(ctx context.Context,
	ip string,
	port string,
	peerHash [20]byte,
	bc core.BlockChain,
	role nodeconfig.Role,
	isBeacon bool,
	isExplorer bool,
	db kv.RwDB,
	stagesList []*Stage,
	revertOrder RevertOrder,
	pruneOrder CleanUpOrder,
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
	debugMode bool) *StagedSync {

	revertStages := make([]*Stage, len(stagesList))
	for i, stageIndex := range revertOrder {
		for _, s := range stagesList {
			if s.ID == stageIndex {
				revertStages[i] = s
				break
			}
		}
	}
	pruneStages := make([]*Stage, len(stagesList))
	for i, stageIndex := range pruneOrder {
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

	return &StagedSync{
		ctx:                    ctx,
		selfip:                 ip,
		selfport:               port,
		selfPeerHash:           peerHash,
		bc:                     bc,
		isBeacon:               isBeacon,
		isExplorer:             isExplorer,
		db:                     db,
		stages:                 stagesList,
		currentStage:           0,
		revertOrder:            revertStages,
		pruningOrder:           pruneStages,
		logPrefixes:            logPrefixes,
		syncStatus:             NewSyncStatus(role),
		commonBlocks:           make(map[int]*types.Block),
		downloadedBlocks:       make(map[uint64][]byte),
		lastMileBlocks:         []*types.Block{},
		syncConfig:             &SyncConfig{},
		StagedSyncTurboMode:    TurboMode,
		UseMemDB:               UseMemDB,
		DoubleCheckBlockHashes: doubleCheckBlockHashes,
		MaxBlocksPerSyncCycle:  maxBlocksPerCycle,
		MaxBackgroundBlocks:    maxBackgroundBlocks,
		MaxMemSyncCycleSize:    maxMemSyncCycleSize,
		VerifyAllSig:           verifyAllSig,
		VerifyHeaderBatchSize:  verifyHeaderBatchSize,
		InsertChainBatchSize:   insertChainBatchSize,
		LogProgress:            logProgress,
		DebugMode:              debugMode,
	}
}

func (s *StagedSync) StageState(stage SyncStageID, tx kv.Tx, db kv.RwDB) (*StageState, error) {
	var blockNum uint64
	var err error
	if errV := CreateView(context.Background(), db, tx, func(rtx kv.Tx) error {
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

func (s *StagedSync) cleanUp(fromStage int, db kv.RwDB, tx kv.RwTx, firstCycle bool) error {
	found := false
	for i := 0; i < len(s.pruningOrder); i++ {
		if s.pruningOrder[i].ID == s.stages[fromStage].ID {
			found = true
		}
		if !found || s.pruningOrder[i] == nil || s.pruningOrder[i].Disabled {
			continue
		}
		if err := s.pruneStage(firstCycle, s.pruningOrder[i], db, tx); err != nil {
			panic(err)
		}
	}
	return nil
}

func (s *StagedSync) Run(db kv.RwDB, tx kv.RwTx, firstCycle bool) error {
	s.prevRevertPoint = nil
	s.timings = s.timings[:0]

	for !s.IsDone() {
		var invalidBlockRevert bool
		if s.revertPoint != nil {
			for j := 0; j < len(s.revertOrder); j++ {
				if s.revertOrder[j] == nil || s.revertOrder[j].Disabled {
					continue
				}
				if err := s.revertStage(firstCycle, s.revertOrder[j], db, tx); err != nil {
					return err
				}
			}
			s.prevRevertPoint = s.revertPoint
			s.revertPoint = nil
			if s.invalidBlock != (common.Hash{}) {
				invalidBlockRevert = true
			}
			s.invalidBlock = common.Hash{}
			if err := s.SetCurrentStage(s.stages[0].ID); err != nil {
				return err
			}
			firstCycle = false
		}

		stage := s.stages[s.currentStage]

		if stage.Disabled {
			utils.Logger().Trace().
				Msgf("[STAGED_SYNC] %s disabled. %s", stage.ID, stage.DisabledDescription)

			s.NextStage()
			continue
		}

		if err := s.runStage(stage, db, tx, firstCycle, invalidBlockRevert); err != nil {
			return err
		}

		s.NextStage()
	}

	if err := s.cleanUp(0, db, tx, firstCycle); err != nil {
		return err
	}
	if err := s.SetCurrentStage(s.stages[0].ID); err != nil {
		return err
	}
	if err := printLogs(tx, s.timings); err != nil {
		return err
	}
	s.currentStage = 0
	return nil
}

func CreateView(ctx context.Context, db kv.RwDB, tx kv.Tx, f func(tx kv.Tx) error) error {
	if tx != nil {
		return f(tx)
	}
	return db.View(ctx, func(etx kv.Tx) error {
		return f(etx)
	})
}

func ByteCount(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB",
		float64(b)/float64(div), "KMGTPE"[exp])
}

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
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] Timings (slower than 50ms) %v", logCtx...)
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
			Msgf("[STAGED_SYNC] Tables %v", bucketSizes...)
	}
	tx.CollectMetrics()
	return nil
}

func (s *StagedSync) runStage(stage *Stage, db kv.RwDB, tx kv.RwTx, firstCycle bool, invalidBlockRevert bool) (err error) {
	start := time.Now()
	stageState, err := s.StageState(stage.ID, tx, db)
	if err != nil {
		return err
	}

	if err = stage.Handler.Exec(firstCycle, invalidBlockRevert, stageState, s, tx); err != nil {
		utils.Logger().Error().
			Err(err).
			Interface("stage id", stage.ID).
			Msgf("[STAGED_SYNC] stage failed")
		return fmt.Errorf("[%s] %w", s.LogPrefix(), err)
	}
	utils.Logger().Info().
		Msgf("[STAGED_SYNC] stage %s executed successfully", stage.ID)

	took := time.Since(start)
	if took > 60*time.Second {
		logPrefix := s.LogPrefix()
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] [%s] DONE in %d", logPrefix, took)

	}
	s.timings = append(s.timings, Timing{stage: stage.ID, took: took})
	return nil
}

func (s *StagedSync) revertStage(firstCycle bool, stage *Stage, db kv.RwDB, tx kv.RwTx) error {
	start := time.Now()
	utils.Logger().Trace().
		Msgf("[STAGED_SYNC] Revert... stage %s", stage.ID)
	stageState, err := s.StageState(stage.ID, tx, db)
	if err != nil {
		return err
	}

	revert := s.NewRevertState(stage.ID, *s.revertPoint, stageState.BlockNumber)
	revert.InvalidBlock = s.invalidBlock

	if stageState.BlockNumber <= revert.RevertPoint {
		return nil
	}

	if err = s.SetCurrentStage(stage.ID); err != nil {
		return err
	}

	err = stage.Handler.Revert(firstCycle, revert, stageState, tx)
	if err != nil {
		return fmt.Errorf("[%s] %w", s.LogPrefix(), err)
	}

	took := time.Since(start)
	if took > 60*time.Second {
		logPrefix := s.LogPrefix()
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] [%s] Revert done in %d", logPrefix, took)
	}
	s.timings = append(s.timings, Timing{isRevert: true, stage: stage.ID, took: took})
	return nil
}

func (s *StagedSync) pruneStage(firstCycle bool, stage *Stage, db kv.RwDB, tx kv.RwTx) error {
	start := time.Now()
	utils.Logger().Info().
		Msgf("[STAGED_SYNC] CleanUp... stage %s", stage.ID)

	stageState, err := s.StageState(stage.ID, tx, db)
	if err != nil {
		return err
	}

	prune, err := s.CleanUpStageState(stage.ID, stageState.BlockNumber, tx, db)
	if err != nil {
		return err
	}
	if err = s.SetCurrentStage(stage.ID); err != nil {
		return err
	}

	err = stage.Handler.CleanUp(firstCycle, prune, tx)
	if err != nil {
		return fmt.Errorf("[%s] %w", s.LogPrefix(), err)
	}

	took := time.Since(start)
	if took > 60*time.Second {
		logPrefix := s.LogPrefix()
		utils.Logger().Trace().
			Msgf("[STAGED_SYNC] [%s] CleanUp done in %d", logPrefix, took)

		utils.Logger().Info().
			Msgf("[STAGED_SYNC] [%s] CleanUp done in %d", logPrefix, took)
	}
	s.timings = append(s.timings, Timing{isCleanUp: true, stage: stage.ID, took: took})
	return nil
}

// DisableAllStages - including their reverts
func (s *StagedSync) DisableAllStages() []SyncStageID {
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

func (s *StagedSync) DisableStages(ids ...SyncStageID) {
	for i := range s.stages {
		for _, id := range ids {
			if s.stages[i].ID != id {
				continue
			}
			s.stages[i].Disabled = true
		}
	}
}

func (s *StagedSync) EnableStages(ids ...SyncStageID) {
	for i := range s.stages {
		for _, id := range ids {
			if s.stages[i].ID != id {
				continue
			}
			s.stages[i].Disabled = false
		}
	}
}

func (ss *StagedSync) purgeAllBlocksFromCache() {
	ss.lastMileMux.Lock()
	ss.lastMileBlocks = nil
	ss.lastMileMux.Unlock()

	ss.syncMux.Lock()
	defer ss.syncMux.Unlock()
	ss.commonBlocks = make(map[int]*types.Block)

	ss.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		configPeer.blockHashes = nil
		configPeer.newBlocks = nil
		return
	})
}

func (ss *StagedSync) purgeOldBlocksFromCache() {
	ss.syncMux.Lock()
	defer ss.syncMux.Unlock()
	ss.commonBlocks = make(map[int]*types.Block)
	ss.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		configPeer.blockHashes = nil
		return
	})
}

// AddLastMileBlock adds the latest a few block into queue for syncing
// only keep the latest blocks with size capped by LastMileBlocksSize
func (ss *StagedSync) AddLastMileBlock(block *types.Block) {
	ss.lastMileMux.Lock()
	defer ss.lastMileMux.Unlock()
	if ss.lastMileBlocks != nil {
		if len(ss.lastMileBlocks) >= LastMileBlocksSize {
			ss.lastMileBlocks = ss.lastMileBlocks[1:]
		}
		ss.lastMileBlocks = append(ss.lastMileBlocks, block)
	}
}

// AddNewBlock will add newly received block into state syncing queue
func (ss *StagedSync) AddNewBlock(peerHash []byte, block *types.Block) {
	pc := ss.syncConfig.FindPeerByHash(peerHash)
	if pc == nil {
		// Received a block with no active peer; just ignore.
		return
	}
	// TODO ek – we shouldn't mess with SyncPeerConfig's mutex.
	//  Factor this into a method, like pc.AddNewBlock(block)
	pc.mux.Lock()
	defer pc.mux.Unlock()
	pc.newBlocks = append(pc.newBlocks, block)
	utils.Logger().Debug().
		Int("total", len(pc.newBlocks)).
		Uint64("blockHeight", block.NumberU64()).
		Msg("[STAGED_SYNC] new block received")
}

// CreateSyncConfig creates SyncConfig for StateSync object.
func (ss *StagedSync) CreateSyncConfig(peers []p2p.Peer, shardID uint32, selfPeerID libp2p_peer.ID, waitForEachPeerToConnect bool) error {
	// sanity check to ensure no duplicate peers
	if err := checkPeersDuplicity(peers); err != nil {
		return err
	}

	// limit the number of dns peers to connect
	randSeed := time.Now().UnixNano()
	targetSize := ss.syncConfig.SelectRandomPeers(peers, randSeed)

	if len(peers) == 0 || targetSize == 0 {
		return errors.New("[STAGED_SYNC] no peers to connect to")
	}

	utils.Logger().Debug().
		Str("self peer ID", string(selfPeerID)).
		Int("peers count", len(peers)).
		Int("target size", targetSize).
		Msg("[STAGED_SYNC] CreateSyncConfig: len of peers")

	if ss.syncConfig != nil {
		ss.syncConfig.CloseConnections()
	}
	ss.syncConfig = &SyncConfig{
		selfPeerID: selfPeerID,
	}

	var connectedPeers int
	for _, peer := range peers {
		client := downloader.ClientSetup(peer.IP, peer.Port, true)
		if client == nil {
			continue
		}
		peerConfig := &SyncPeerConfig{
			peer:   peer,
			ip:     peer.IP,
			port:   peer.Port,
			client: client,
		}
		ss.syncConfig.AddPeer(peerConfig)
		connectedPeers++
		if connectedPeers >= targetSize+NumPeersReserved {
			break
		}
	}

	if connectedPeers == 0 {
		return errors.New("[STAGED_SYNC] CreateSyncConfig: no ready peers to connect")
	}

	// select reserved peers
	if connectedPeers > targetSize {
		ss.syncConfig.reservedPeers = make([]*SyncPeerConfig, connectedPeers-targetSize)
		copy(ss.syncConfig.reservedPeers, ss.syncConfig.peers[targetSize:])
	}
	// select main peers
	ss.syncConfig.peers = ss.syncConfig.peers[:targetSize]

	utils.Logger().Info().
		Int("len", len(ss.syncConfig.peers)).
		Msg("[STAGED_SYNC] Finished making connection to peers")

	return nil
}

// checkPeersDuplicity checks whether there are duplicates in p2p.Peer
func checkPeersDuplicity(ps []p2p.Peer) error {
	type peerDupID struct {
		ip   string
		port string
	}
	m := make(map[peerDupID]struct{})
	for _, p := range ps {
		dip := peerDupID{p.IP, p.Port}
		if _, ok := m[dip]; ok {
			return fmt.Errorf("duplicate peer [%v:%v]", p.IP, p.Port)
		}
		m[dip] = struct{}{}
	}
	return nil
}

// GetActivePeerNumber returns the number of active peers
func (ss *StagedSync) GetActivePeerNumber() int {
	if ss.syncConfig == nil {
		return 0
	}
	// len() is atomic; no need to hold mutex.
	return len(ss.syncConfig.peers)
}

// getConsensusHashes gets all hashes needed to download.
func (ss *StagedSync) getConsensusHashes(startHash []byte, size uint32, bgMode bool) error {
	var bgModeError error

	var wg sync.WaitGroup
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !peerConfig.client.IsReady() {
				// try to connect
				if ready := peerConfig.client.WaitForConnection(1000 * time.Millisecond); !ready {
					// replace it with reserved peer (in bg mode don't replace because maybe other stages still are using this node)
					if bgMode {
						bgModeError = ErrSomeNodesNotReady
						brk = true //finish whole peers loop
					} else {
						if !peerConfig.client.IsConnecting() {
							ss.syncConfig.ReplacePeerWithReserved(peerConfig, "not ready to download consensus hashes")
						}
					}
					return
				}
			}
			response := peerConfig.client.GetBlockHashes(startHash, size, ss.selfip, ss.selfport)
			if response == nil {
				utils.Logger().Warn().
					Str("peerIP", peerConfig.ip).
					Str("peerPort", peerConfig.port).
					Msg("[STAGED_SYNC] getConsensusHashes Nil Response, will be replaced with reserved node (if any)")
				// replace it with reserved peer (in bg mode don't replace because maybe other stages still are using this node)
				if bgMode {
					bgModeError = ErrSomeNodesBlockHashFail
					brk = true //finish whole peers loop
				} else {
					isBrokenPeer := peerConfig.AddFailedTime(downloadBlocksRetryLimit)
					if isBrokenPeer {
						ss.syncConfig.ReplacePeerWithReserved(peerConfig, "receiving nil response for block hashes")
					}
				}
				return
			}
			utils.Logger().Info().
				Uint32("queried blockHash size", size).
				Int("got blockHashSize", len(response.Payload)).
				Str("PeerIP", peerConfig.ip).
				Bool("background Mode", bgMode).
				Msg("[STAGED_SYNC] GetBlockHashes")

			if len(response.Payload) > int(size+1) {
				utils.Logger().Warn().
					Uint32("requestSize", size).
					Int("respondSize", len(response.Payload)).
					Msg("[STAGED_SYNC] getConsensusHashes: receive more blockHashes than requested!")
				peerConfig.blockHashes = response.Payload[:size+1]
			} else {
				peerConfig.blockHashes = response.Payload
			}

		}()
		return
	})
	wg.Wait()

	return bgModeError
}

// getInvalidPeersByBlockHashes analyzes block hashes and detects invalid peers
func (ss *StagedSync) getInvalidPeersByBlockHashes(tx kv.RwTx) (map[string]bool, int, error) {
	invalidPeers := make(map[string]bool)
	if len(ss.syncConfig.peers) < 3 {
		lb := len(ss.syncConfig.peers[0].blockHashes)
		return invalidPeers, lb, nil
	}

	// confirmations threshold to consider as valid block hash
	th := 2 * int(len(ss.syncConfig.peers)/3)
	if len(ss.syncConfig.peers) == 4 {
		th = 3
	}

	type BlockHashMap struct {
		peers   map[string]bool
		isValid bool
	}

	// populate the block hashes map
	bhm := make(map[string]*BlockHashMap)
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		for _, blkHash := range peerConfig.blockHashes {
			k := string(blkHash)
			if _, ok := bhm[k]; !ok {
				bhm[k] = &BlockHashMap{
					peers: make(map[string]bool),
				}
			}
			peerHash := string(peerConfig.peerHash)
			bhm[k].peers[peerHash] = true
			bhm[k].isValid = true
		}
		return
	})

	var validBlockHashes int

	for blkHash, hmap := range bhm {

		// if block is not confirmed by th% of peers, it is considered as invalid block
		// So, any peer with that block hash will be considered as invalid peer
		if len(hmap.peers) < th {
			bhm[blkHash].isValid = false
			for _, p := range ss.syncConfig.peers {
				hasBlockHash := hmap.peers[string(p.peerHash)]
				if hasBlockHash {
					invalidPeers[string(p.peerHash)] = true
				}
			}
			continue
		}

		// so, block hash is valid, because have been sent by more than th number of peers
		validBlockHashes++

		// if all peers already sent this block hash, then it is considered as valid
		if len(hmap.peers) == len(ss.syncConfig.peers) {
			continue
		}

		//consider invalid peer if it hasn't sent this block hash
		for _, p := range ss.syncConfig.peers {
			hasBlockHash := hmap.peers[string(p.peerHash)]
			if !hasBlockHash {
				invalidPeers[string(p.peerHash)] = true
			}
		}

	}
	fmt.Printf("%d out of %d peers have missed blocks or sent invalid blocks\n", len(invalidPeers), len(ss.syncConfig.peers))
	return invalidPeers, validBlockHashes, nil
}

func (ss *StagedSync) generateStateSyncTaskQueue(bc core.BlockChain, tx kv.RwTx) error {
	ss.stateSyncTaskQueue = queue.New(0)
	allTasksAddedToQueue := false
	ss.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		for id, blockHash := range configPeer.blockHashes {
			if err := ss.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash}); err != nil {
				ss.stateSyncTaskQueue = queue.New(0)
				utils.Logger().Error().
					Err(err).
					Int("taskIndex", id).
					Str("taskBlock", hex.EncodeToString(blockHash)).
					Msg("[STAGED_SYNC] generateStateSyncTaskQueue: cannot add task")
				break
			}
		}
		// check if all block hashes added to task queue
		if ss.stateSyncTaskQueue.Len() == int64(len(configPeer.blockHashes)) {
			allTasksAddedToQueue = true
			brk = true
		}
		return
	})

	if !allTasksAddedToQueue {
		return ErrAddTaskFailed
	}
	utils.Logger().Info().
		Int64("length", ss.stateSyncTaskQueue.Len()).
		Msg("[STAGED_SYNC] generateStateSyncTaskQueue: finished")
	return nil
}

// RlpDecodeBlockOrBlockWithSig decodes payload to types.Block or BlockWithSig.
// Return the block with commitSig if set.
func RlpDecodeBlockOrBlockWithSig(payload []byte) (*types.Block, error) {
	var block *types.Block
	if err := rlp.DecodeBytes(payload, &block); err == nil {
		// received payload as *types.Block
		return block, nil
	}

	var bws BlockWithSig
	if err := rlp.DecodeBytes(payload, &bws); err == nil {
		block := bws.Block
		block.SetCurrentCommitSig(bws.CommitSigAndBitmap)
		return block, nil
	}
	return nil, errors.New("failed to decode to either types.Block or BlockWithSig")
}

// CompareBlockByHash compares two block by hash, it will be used in sort the blocks
func CompareBlockByHash(a *types.Block, b *types.Block) int {
	ha := a.Hash()
	hb := b.Hash()
	return bytes.Compare(ha[:], hb[:])
}

// GetHowManyMaxConsensus will get the most common blocks and the first such blockID
func GetHowManyMaxConsensus(blocks []*types.Block) (int, int) {
	// As all peers are sorted by their blockHashes, all equal blockHashes should come together and consecutively.
	curCount := 0
	curFirstID := -1
	maxCount := 0
	maxFirstID := -1
	for i := range blocks {
		if curFirstID == -1 || CompareBlockByHash(blocks[curFirstID], blocks[i]) != 0 {
			curCount = 1
			curFirstID = i
		} else {
			curCount++
		}
		if curCount > maxCount {
			maxCount = curCount
			maxFirstID = curFirstID
		}
	}
	return maxFirstID, maxCount
}

func (ss *StagedSync) getMaxConsensusBlockFromParentHash(parentHash common.Hash) *types.Block {
	var candidateBlocks []*types.Block

	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		peerConfig.mux.Lock()
		defer peerConfig.mux.Unlock()

		for _, block := range peerConfig.newBlocks {
			ph := block.ParentHash()
			if bytes.Equal(ph[:], parentHash[:]) {
				candidateBlocks = append(candidateBlocks, block)
				break
			}
		}
		return
	})
	if len(candidateBlocks) == 0 {
		return nil
	}
	// Sort by blockHashes.
	sort.Slice(candidateBlocks, func(i, j int) bool {
		return CompareBlockByHash(candidateBlocks[i], candidateBlocks[j]) == -1
	})
	maxFirstID, maxCount := GetHowManyMaxConsensus(candidateBlocks)
	hash := candidateBlocks[maxFirstID].Hash()
	utils.Logger().Debug().
		Hex("parentHash", parentHash[:]).
		Hex("hash", hash[:]).
		Int("maxCount", maxCount).
		Msg("[STAGED_SYNC] Find block with matching parent hash")
	return candidateBlocks[maxFirstID]
}

func (ss *StagedSync) getBlockFromOldBlocksByParentHash(parentHash common.Hash) *types.Block {
	for _, block := range ss.commonBlocks {
		ph := block.ParentHash()
		if bytes.Equal(ph[:], parentHash[:]) {
			return block
		}
	}
	return nil
}

func (ss *StagedSync) getBlockFromLastMileBlocksByParentHash(parentHash common.Hash) *types.Block {
	for _, block := range ss.lastMileBlocks {
		ph := block.ParentHash()
		if bytes.Equal(ph[:], parentHash[:]) {
			return block
		}
	}
	return nil
}

// UpdateBlockAndStatus updates block and its status in db
func (ss *StagedSync) UpdateBlockAndStatus(block *types.Block, bc core.BlockChain) error {
	if block.NumberU64() != bc.CurrentBlock().NumberU64()+1 {
		utils.Logger().Debug().
			Uint64("curBlockNum", bc.CurrentBlock().NumberU64()).
			Uint64("receivedBlockNum", block.NumberU64()).
			Msg("[STAGED_SYNC] Inappropriate block number, ignore!")
		return nil
	}
	verifyAllSig := true

	haveCurrentSig := len(block.GetCurrentCommitSig()) != 0
	// Verify block signatures
	if block.NumberU64() > 1 {
		// Verify signature every N blocks (which N is verifyHeaderBatchSize and can be adjusted in configs)
		verifySeal := block.NumberU64()%ss.VerifyHeaderBatchSize == 0 || verifyAllSig
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
			utils.Logger().Debug().
				Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).
				Msg("[STAGED_SYNC] VerifyHeaderSignature")
		}
		err := bc.Engine().VerifyHeader(bc, block.Header(), verifySeal)
		if err == engine.ErrUnknownAncestor {
			return err
		} else if err != nil {
			utils.Logger().Error().
				Err(err).
				Uint64("block number", block.NumberU64()).
				Msgf("[STAGED_SYNC] UpdateBlockAndStatus: failed verifying signatures for new block")

			if !verifyAllSig {
				utils.Logger().Info().
					Interface("block", bc.CurrentBlock()).
					Msg("[STAGED_SYNC] UpdateBlockAndStatus: Rolling back last 99 blocks!")
				for i := uint64(0); i < ss.VerifyHeaderBatchSize-1; i++ {
					if rbErr := bc.Rollback([]common.Hash{bc.CurrentBlock().Hash()}); rbErr != nil {
						utils.Logger().Error().
							Err(rbErr).
							Msg("[STAGED_SYNC] UpdateBlockAndStatus: failed to rollback")
						return err
					}
				}
			}
			return err
		}
	}

	_, err := bc.InsertChain([]*types.Block{block}, false /* verifyHeaders */)
	switch {
	case errors.Is(err, core.ErrKnownBlock):
	case err != nil:
		utils.Logger().Error().
			Err(err).
			Uint64("block number", block.NumberU64()).
			Uint32("shard", block.ShardID()).
			Msgf("[STAGED_SYNC] UpdateBlockAndStatus: Error adding new block to blockchain")
		return err
	default:
	}
	utils.Logger().Info().
		Uint64("blockHeight", block.NumberU64()).
		Uint64("blockEpoch", block.Epoch().Uint64()).
		Str("blockHex", block.Hash().Hex()).
		Uint32("ShardID", block.ShardID()).
		Msg("[STAGED_SYNC] UpdateBlockAndStatus: New Block Added to Blockchain")

	for i, tx := range block.StakingTransactions() {
		utils.Logger().Info().
			Msgf(
				"StakingTxn %d: %s, %v", i, tx.StakingType().String(), tx.StakingMessage(),
			)
	}
	return nil
}

// RegisterNodeInfo will register node to peers to accept future new block broadcasting
// return number of successful registration
func (ss *StagedSync) RegisterNodeInfo() int {
	registrationNumber := RegistrationNumber
	utils.Logger().Debug().
		Int("registrationNumber", registrationNumber).
		Int("activePeerNumber", len(ss.syncConfig.peers)).
		Msg("[STAGED_SYNC] node registration to peers")

	count := 0
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		logger := utils.Logger().With().Str("peerPort", peerConfig.port).Str("peerIP", peerConfig.ip).Logger()
		if count >= registrationNumber {
			brk = true
			return
		}
		if peerConfig.ip == ss.selfip && peerConfig.port == GetSyncingPort(ss.selfport) {
			logger.Debug().
				Str("selfport", ss.selfport).
				Str("selfsyncport", GetSyncingPort(ss.selfport)).
				Msg("[STAGED_SYNC] skip self")
			return
		}
		err := peerConfig.registerToBroadcast(ss.selfPeerHash[:], ss.selfip, ss.selfport)
		if err != nil {
			logger.Debug().
				Hex("selfPeerHash", ss.selfPeerHash[:]).
				Msg("[STAGED_SYNC] register failed to peer")
			return
		}

		logger.Debug().Msg("[STAGED_SYNC] register success")
		count++
		return
	})
	return count
}

// getMaxPeerHeight returns the maximum blockchain heights from peers
func (ss *StagedSync) getMaxPeerHeight() (uint64, error) {
	maxHeight := uint64(math.MaxUint64)
	var (
		wg   sync.WaitGroup
		lock sync.Mutex
	)

	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := peerConfig.client.GetBlockChainHeight()
			if err != nil {
				utils.Logger().Error().
					Err(err).
					Str("peerIP", peerConfig.ip).
					Str("peerPort", peerConfig.port).
					Msg("[STAGED_SYNC]GetBlockChainHeight failed")
				ss.syncConfig.RemovePeer(peerConfig, "GetBlockChainHeight failed")
				return
			}
			utils.Logger().Info().
				Str("peerIP", peerConfig.ip).
				Uint64("blockHeight", response.BlockHeight).
				Msg("[STAGED_SYNC] getMaxPeerHeight")
			lock.Lock()
			if response != nil {
				if maxHeight == uint64(math.MaxUint64) || maxHeight < response.BlockHeight {
					maxHeight = response.BlockHeight
				}
			}
			lock.Unlock()
		}()
		return
	})
	wg.Wait()

	if maxHeight == uint64(math.MaxUint64) {
		return 0, ErrMaxPeerHeightFail
	}

	return maxHeight, nil
}

// IsSameBlockchainHeight checks whether the node is out of sync from other peers
func (ss *StagedSync) IsSameBlockchainHeight(bc core.BlockChain) (uint64, bool) {
	otherHeight, _ := ss.getMaxPeerHeight()
	currentHeight := bc.CurrentBlock().NumberU64()
	return otherHeight, currentHeight == otherHeight
}

// GetMaxPeerHeight returns maximum block height of connected peers
func (ss *StagedSync) GetMaxPeerHeight() (uint64, error) {
	return ss.getMaxPeerHeight()
}

func (ss *StagedSync) addConsensusLastMile(bc core.BlockChain, cs *consensus.Consensus) error {
	curNumber := bc.CurrentBlock().NumberU64()
	return cs.GetLastMileBlockIter(curNumber+1, func(blockIter *consensus.LastMileBlockIter) error {
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
			}
		}
		return nil
	})
}

// GetSyncingPort returns the syncing port.
func GetSyncingPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-SyncingPortDifference)
	}
	return ""
}

func ParseResult(res SyncCheckResult) (IsSynchronized bool, OtherHeight uint64, HeightDiff uint64) {
	IsSynchronized = res.IsSynchronized
	OtherHeight = res.OtherHeight
	HeightDiff = res.HeightDiff
	return IsSynchronized, OtherHeight, HeightDiff
}

// GetSyncStatus get the last sync status for other modules (E.g. RPC, explorer).
// If the last sync result is not expired, return the sync result immediately.
// If the last result is expired, ask the remote DNS nodes for latest height and return the result.
func (ss *StagedSync) GetSyncStatus() SyncCheckResult {
	return ss.syncStatus.Get(func() SyncCheckResult {
		return ss.isSynchronized(false)
	})
}

func (ss *StagedSync) GetParsedSyncStatus() (IsSynchronized bool, OtherHeight uint64, HeightDiff uint64) {
	res := ss.syncStatus.Get(func() SyncCheckResult {
		return ss.isSynchronized(false)
	})
	return ParseResult(res)
}

func (ss *StagedSync) IsSynchronized() bool {
	result := ss.GetSyncStatus()
	return result.IsSynchronized
}

// GetSyncStatusDoubleChecked returns the sync status when enforcing a immediate query on DNS nodes
// with a double check to avoid false alarm.
func (ss *StagedSync) GetSyncStatusDoubleChecked() SyncCheckResult {
	result := ss.isSynchronized(true)
	return result
}

func (ss *StagedSync) GetParsedSyncStatusDoubleChecked() (IsSynchronized bool, OtherHeight uint64, HeightDiff uint64) {
	result := ss.isSynchronized(true)
	return ParseResult(result)
}

// isSynchronized queries the remote DNS node for the latest height to check what is the current sync status
func (ss *StagedSync) isSynchronized(doubleCheck bool) SyncCheckResult {
	if ss.syncConfig == nil {
		return SyncCheckResult{} // If syncConfig is not instantiated, return not in sync
	}
	lastHeight := ss.Blockchain().CurrentBlock().NumberU64()
	otherHeight1, errMaxHeight1 := ss.getMaxPeerHeight()
	if errMaxHeight1 != nil {
		return SyncCheckResult{
			IsSynchronized: false,
			OtherHeight:    0,
			HeightDiff:     0,
		}
	}
	wasOutOfSync := lastHeight+inSyncThreshold < otherHeight1

	if !doubleCheck {
		heightDiff := otherHeight1 - lastHeight
		if otherHeight1 < lastHeight {
			heightDiff = 0 //
		}
		utils.Logger().Info().
			Uint64("OtherHeight", otherHeight1).
			Uint64("lastHeight", lastHeight).
			Msg("[STAGED_SYNC] Checking sync status")
		return SyncCheckResult{
			IsSynchronized: !wasOutOfSync,
			OtherHeight:    otherHeight1,
			HeightDiff:     heightDiff,
		}
	}
	// double check the sync status after 1 second to confirm (avoid false alarm)
	time.Sleep(1 * time.Second)

	otherHeight2, errMaxHeight2 := ss.getMaxPeerHeight()
	if errMaxHeight2 != nil {
		otherHeight2 = otherHeight1
	}
	currentHeight := ss.Blockchain().CurrentBlock().NumberU64()

	isOutOfSync := currentHeight+inSyncThreshold < otherHeight2
	utils.Logger().Info().
		Uint64("OtherHeight1", otherHeight1).
		Uint64("OtherHeight2", otherHeight2).
		Uint64("lastHeight", lastHeight).
		Uint64("currentHeight", currentHeight).
		Msg("[STAGED_SYNC] Checking sync status")
	// Only confirm out of sync when the node has lower height and didn't move in heights for 2 consecutive checks
	heightDiff := otherHeight2 - lastHeight
	if otherHeight2 < lastHeight {
		heightDiff = 0 // overflow
	}
	return SyncCheckResult{
		IsSynchronized: !(wasOutOfSync && isOutOfSync && lastHeight == currentHeight),
		OtherHeight:    otherHeight2,
		HeightDiff:     heightDiff,
	}
}
