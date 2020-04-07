package syncing

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/harmony-one/harmony/consensus/engine"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
)

// Constants for syncing.
const (
	downloadBlocksRetryLimit        = 5 // downloadBlocks service retry limit
	TimesToFail                     = 5 // downloadBlocks service retry limit
	RegistrationNumber              = 3
	SyncingPortDifference           = 3000
	inSyncThreshold                 = 0    // when peerBlockHeight - myBlockHeight <= inSyncThreshold, it's ready to join consensus
	SyncLoopBatchSize        uint32 = 1000 // maximum size for one query of block hashes
	verifyHeaderBatchSize    uint64 = 100  // block chain header verification batch size
	SyncLoopFrequency               = 1    // unit in second
	LastMileBlocksSize              = 50
)

// SyncPeerConfig is peer config to sync.
type SyncPeerConfig struct {
	ip          string
	port        string
	peerHash    []byte
	client      *downloader.Client
	blockHashes [][]byte       // block hashes before node doing sync
	newBlocks   []*types.Block // blocks after node doing sync
	mux         sync.Mutex
}

// GetClient returns client pointer of downloader.Client
func (peerConfig *SyncPeerConfig) GetClient() *downloader.Client {
	return peerConfig.client
}

// SyncBlockTask is the task struct to sync a specific block.
type SyncBlockTask struct {
	index     int
	blockHash []byte
}

// SyncConfig contains an array of SyncPeerConfig.
type SyncConfig struct {
	// mtx locks peers, and *SyncPeerConfig pointers in peers.
	// SyncPeerConfig itself is guarded by its own mutex.
	mtx sync.RWMutex

	peers []*SyncPeerConfig
}

// AddPeer adds the given sync peer.
func (sc *SyncConfig) AddPeer(peer *SyncPeerConfig) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	sc.peers = append(sc.peers, peer)
}

// ForEachPeer calls the given function with each peer.
// It breaks the iteration iff the function returns true.
func (sc *SyncConfig) ForEachPeer(f func(peer *SyncPeerConfig) (brk bool)) {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	for _, peer := range sc.peers {
		if f(peer) {
			break
		}
	}
}

// CreateStateSync returns the implementation of StateSyncInterface interface.
func CreateStateSync(ip string, port string, peerHash [20]byte) *StateSync {
	stateSync := &StateSync{}
	stateSync.selfip = ip
	stateSync.selfport = port
	stateSync.selfPeerHash = peerHash
	stateSync.commonBlocks = make(map[int]*types.Block)
	stateSync.lastMileBlocks = []*types.Block{}
	return stateSync
}

// StateSync is the struct that implements StateSyncInterface.
type StateSync struct {
	selfip             string
	selfport           string
	selfPeerHash       [20]byte // hash of ip and address combination
	commonBlocks       map[int]*types.Block
	lastMileBlocks     []*types.Block // last mile blocks to catch up with the consensus
	syncConfig         *SyncConfig
	stateSyncTaskQueue *queue.Queue
	syncMux            sync.Mutex
	lastMileMux        sync.Mutex
}

func (ss *StateSync) purgeAllBlocksFromCache() {
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

func (ss *StateSync) purgeOldBlocksFromCache() {
	ss.syncMux.Lock()
	defer ss.syncMux.Unlock()
	ss.commonBlocks = make(map[int]*types.Block)
	ss.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		configPeer.blockHashes = nil
		return
	})
}

// AddLastMileBlock add the lastest a few block into queue for syncing
// only keep the latest blocks with size capped by LastMileBlocksSize
func (ss *StateSync) AddLastMileBlock(block *types.Block) {
	ss.lastMileMux.Lock()
	defer ss.lastMileMux.Unlock()
	if ss.lastMileBlocks != nil {
		if len(ss.lastMileBlocks) >= LastMileBlocksSize {
			ss.lastMileBlocks = ss.lastMileBlocks[1:]
		}
		ss.lastMileBlocks = append(ss.lastMileBlocks, block)
	}
}

// CloseConnections close grpc  connections for state sync clients
func (sc *SyncConfig) CloseConnections() {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	for _, pc := range sc.peers {
		pc.client.Close()
	}
}

// FindPeerByHash returns the peer with the given hash, or nil if not found.
func (sc *SyncConfig) FindPeerByHash(peerHash []byte) *SyncPeerConfig {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	for _, pc := range sc.peers {
		if bytes.Equal(pc.peerHash, peerHash) {
			return pc
		}
	}
	return nil
}

// AddNewBlock will add newly received block into state syncing queue
func (ss *StateSync) AddNewBlock(peerHash []byte, block *types.Block) {
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
		Msg("[SYNC] new block received")
}

// CreateTestSyncPeerConfig used for testing.
func CreateTestSyncPeerConfig(client *downloader.Client, blockHashes [][]byte) *SyncPeerConfig {
	return &SyncPeerConfig{
		client:      client,
		blockHashes: blockHashes,
	}
}

// CompareSyncPeerConfigByblockHashes compares two SyncPeerConfig by blockHashes.
func CompareSyncPeerConfigByblockHashes(a *SyncPeerConfig, b *SyncPeerConfig) int {
	if len(a.blockHashes) != len(b.blockHashes) {
		if len(a.blockHashes) < len(b.blockHashes) {
			return -1
		}
		return 1
	}
	for id := range a.blockHashes {
		if !reflect.DeepEqual(a.blockHashes[id], b.blockHashes[id]) {
			return bytes.Compare(a.blockHashes[id], b.blockHashes[id])
		}
	}
	return 0
}

// GetBlocks gets blocks by calling grpc request to the corresponding peer.
func (peerConfig *SyncPeerConfig) GetBlocks(hashes [][]byte) ([][]byte, error) {
	response := peerConfig.client.GetBlocks(hashes)
	if response == nil {
		return nil, ErrGetBlock
	}
	return response.Payload, nil
}

// CreateSyncConfig creates SyncConfig for StateSync object.
func (ss *StateSync) CreateSyncConfig(peers []p2p.Peer, isBeacon bool) error {
	utils.Logger().Debug().
		Int("len", len(peers)).
		Bool("isBeacon", isBeacon).
		Msg("[SYNC] CreateSyncConfig: len of peers")

	if len(peers) == 0 {
		return ctxerror.New("[SYNC] no peers to connect to")
	}
	if ss.syncConfig != nil {
		ss.syncConfig.CloseConnections()
	}
	ss.syncConfig = &SyncConfig{}
	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(peer p2p.Peer) {
			defer wg.Done()
			client := downloader.ClientSetup(peer.IP, peer.Port)
			if client == nil {
				return
			}
			peerConfig := &SyncPeerConfig{
				ip:     peer.IP,
				port:   peer.Port,
				client: client,
			}
			ss.syncConfig.AddPeer(peerConfig)
		}(peer)
	}
	wg.Wait()
	utils.Logger().Info().
		Int("len", len(ss.syncConfig.peers)).
		Bool("isBeacon", isBeacon).
		Msg("[SYNC] Finished making connection to peers")

	return nil
}

// GetActivePeerNumber returns the number of active peers
func (ss *StateSync) GetActivePeerNumber() int {
	if ss.syncConfig == nil {
		return 0
	}
	// len() is atomic; no need to hold mutex.
	return len(ss.syncConfig.peers)
}

// getHowManyMaxConsensus returns max number of consensus nodes and the first ID of consensus group.
// Assumption: all peers are sorted by CompareSyncPeerConfigByBlockHashes first.
// Caller shall ensure mtx is locked for reading.
func (sc *SyncConfig) getHowManyMaxConsensus() (int, int) {
	// As all peers are sorted by their blockHashes, all equal blockHashes should come together and consecutively.
	curCount := 0
	curFirstID := -1
	maxCount := 0
	maxFirstID := -1
	for i := range sc.peers {
		if curFirstID == -1 || CompareSyncPeerConfigByblockHashes(sc.peers[curFirstID], sc.peers[i]) != 0 {
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

// InitForTesting used for testing.
func (sc *SyncConfig) InitForTesting(client *downloader.Client, blockHashes [][]byte) {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	for i := range sc.peers {
		sc.peers[i].blockHashes = blockHashes
		sc.peers[i].client = client
	}
}

// cleanUpPeers cleans up all peers whose blockHashes are not equal to
// consensus block hashes.  Caller shall ensure mtx is locked for RW.
func (sc *SyncConfig) cleanUpPeers(maxFirstID int) {
	fixedPeer := sc.peers[maxFirstID]
	for i := 0; i < len(sc.peers); i++ {
		if CompareSyncPeerConfigByblockHashes(fixedPeer, sc.peers[i]) != 0 {
			// TODO: move it into a util delete func.
			// See tip https://github.com/golang/go/wiki/SliceTricks
			// Close the client and remove the peer out of the
			sc.peers[i].client.Close()
			copy(sc.peers[i:], sc.peers[i+1:])
			sc.peers[len(sc.peers)-1] = nil
			sc.peers = sc.peers[:len(sc.peers)-1]
		}
	}
}

// GetBlockHashesConsensusAndCleanUp selects the most common peer config based on their block hashes to download/sync.
// Note that choosing the most common peer config does not guarantee that the blocks to be downloaded are the correct ones.
// The subsequent node syncing steps of verifying the block header chain will give such confirmation later.
// If later block header verification fails with the sync peer config chosen here, the entire sync loop gets retried with a new peer set.
func (sc *SyncConfig) GetBlockHashesConsensusAndCleanUp() {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	// Sort all peers by the blockHashes.
	sort.Slice(sc.peers, func(i, j int) bool {
		return CompareSyncPeerConfigByblockHashes(sc.peers[i], sc.peers[j]) == -1
	})
	maxFirstID, maxCount := sc.getHowManyMaxConsensus()
	utils.Logger().Info().
		Int("maxFirstID", maxFirstID).
		Int("maxCount", maxCount).
		Msg("[SYNC] block consensus hashes")
	sc.cleanUpPeers(maxFirstID)
}

// getConsensusHashes gets all hashes needed to download.
func (ss *StateSync) getConsensusHashes(startHash []byte, size uint32) {
	var wg sync.WaitGroup
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			response := peerConfig.client.GetBlockHashes(startHash, size, ss.selfip, ss.selfport)
			if response == nil {
				utils.Logger().Warn().
					Str("peerIP", peerConfig.ip).
					Str("peerPort", peerConfig.port).
					Msg("[SYNC] getConsensusHashes Nil Response")
				return
			}
			if len(response.Payload) > int(size+1) {
				utils.Logger().Warn().
					Uint32("requestSize", size).
					Int("respondSize", len(response.Payload)).
					Msg("[SYNC] getConsensusHashes: receive more blockHahses than request!")
				peerConfig.blockHashes = response.Payload[:size+1]
			} else {
				peerConfig.blockHashes = response.Payload
			}
		}()
		return
	})
	wg.Wait()
	ss.syncConfig.GetBlockHashesConsensusAndCleanUp()
	utils.Logger().Info().Msg("[SYNC] Finished getting consensus block hashes")
}

func (ss *StateSync) generateStateSyncTaskQueue(bc *core.BlockChain) {
	ss.stateSyncTaskQueue = queue.New(0)
	ss.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		for id, blockHash := range configPeer.blockHashes {
			if err := ss.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash}); err != nil {
				utils.Logger().Warn().
					Err(err).
					Int("taskIndex", id).
					Str("taskBlock", hex.EncodeToString(blockHash)).
					Msg("[SYNC] generateStateSyncTaskQueue: cannot add task")
			}
		}
		brk = true
		return
	})
	utils.Logger().Info().Int64("length", ss.stateSyncTaskQueue.Len()).Msg("[SYNC] generateStateSyncTaskQueue: finished")
}

// downloadBlocks downloads blocks from state sync task queue.
func (ss *StateSync) downloadBlocks(bc *core.BlockChain) {
	// Initialize blockchain
	var wg sync.WaitGroup
	count := 0
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func(stateSyncTaskQueue *queue.Queue, bc *core.BlockChain) {
			defer wg.Done()
			for !stateSyncTaskQueue.Empty() {
				task, err := ss.stateSyncTaskQueue.Poll(1, time.Millisecond)
				if err == queue.ErrTimeout || len(task) == 0 {
					utils.Logger().Error().Err(err).Msg("[SYNC] downloadBlocks: ss.stateSyncTaskQueue poll timeout")
					break
				}
				syncTask := task[0].(SyncBlockTask)
				//id := syncTask.index
				payload, err := peerConfig.GetBlocks([][]byte{syncTask.blockHash})
				if err != nil || len(payload) == 0 {
					count++
					utils.Logger().Error().Err(err).Int("failNumber", count).Msg("[SYNC] downloadBlocks: GetBlocks failed")
					if count > downloadBlocksRetryLimit {
						break
					}
					if err := ss.stateSyncTaskQueue.Put(syncTask); err != nil {
						utils.Logger().Warn().
							Err(err).
							Int("taskIndex", syncTask.index).
							Str("taskBlock", hex.EncodeToString(syncTask.blockHash)).
							Msg("downloadBlocks: cannot add task")
					}
					continue
				}

				var blockObj types.Block
				// currently only send one block a time
				err = rlp.DecodeBytes(payload[0], &blockObj)

				if err != nil {
					count++
					utils.Logger().Error().Err(err).Msg("[SYNC] downloadBlocks: failed to DecodeBytes from received new block")
					if count > downloadBlocksRetryLimit {
						break
					}
					if err := ss.stateSyncTaskQueue.Put(syncTask); err != nil {
						utils.Logger().Warn().
							Err(err).
							Int("taskIndex", syncTask.index).
							Str("taskBlock", hex.EncodeToString(syncTask.blockHash)).
							Msg("cannot add task")
					}
					continue
				}
				ss.syncMux.Lock()
				ss.commonBlocks[syncTask.index] = &blockObj
				ss.syncMux.Unlock()
			}
		}(ss.stateSyncTaskQueue, bc)
		return
	})
	wg.Wait()
	utils.Logger().Info().Msg("[SYNC] downloadBlocks: finished")
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

func (ss *StateSync) getMaxConsensusBlockFromParentHash(parentHash common.Hash) *types.Block {
	candidateBlocks := []*types.Block{}
	ss.syncMux.Lock()
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		for _, block := range peerConfig.newBlocks {
			ph := block.ParentHash()
			if bytes.Equal(ph[:], parentHash[:]) {
				candidateBlocks = append(candidateBlocks, block)
				break
			}
		}
		return
	})
	ss.syncMux.Unlock()
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
		Msg("[SYNC] Find block with matching parenthash")
	return candidateBlocks[maxFirstID]
}

func (ss *StateSync) getBlockFromOldBlocksByParentHash(parentHash common.Hash) *types.Block {
	for _, block := range ss.commonBlocks {
		ph := block.ParentHash()
		if bytes.Equal(ph[:], parentHash[:]) {
			return block
		}
	}
	return nil
}

func (ss *StateSync) getBlockFromLastMileBlocksByParentHash(parentHash common.Hash) *types.Block {
	for _, block := range ss.lastMileBlocks {
		ph := block.ParentHash()
		if bytes.Equal(ph[:], parentHash[:]) {
			return block
		}
	}
	return nil
}

// UpdateBlockAndStatus ...
func (ss *StateSync) UpdateBlockAndStatus(block *types.Block, bc *core.BlockChain, worker *worker.Worker, verifyAllSig bool) error {
	utils.Logger().Info().Str("blockHex", bc.CurrentBlock().Hash().Hex()).Uint64("blockNum", block.NumberU64()).Msg("[SYNC] UpdateBlockAndStatus: Current Block")

	if block.NumberU64() != bc.CurrentBlock().NumberU64()+1 {
		utils.Logger().Info().Uint64("curBlockNum", bc.CurrentBlock().NumberU64()).Uint64("receivedBlockNum", block.NumberU64()).Msg("[SYNC] Inappropriate block number, ignore!")
		return nil
	}

	// Verify block signatures
	if block.NumberU64() > 1 {
		// Verify signature every 100 blocks
		verifySig := block.NumberU64()%verifyHeaderBatchSize == 0
		if verifyAllSig {
			verifySig = true
		}
		err := bc.Engine().VerifyHeader(bc, block.Header(), verifySig)
		if err == engine.ErrUnknownAncestor {
			return err
		} else if err != nil {
			utils.Logger().Error().Err(err).Msgf("[SYNC] UpdateBlockAndStatus: failed verifying signatures for new block %d", block.NumberU64())

			if !verifyAllSig {
				utils.Logger().Debug().Interface("block", bc.CurrentBlock()).Msg("[SYNC] UpdateBlockAndStatus: Rolling back last 99 blocks!")
				for i := uint64(0); i < verifyHeaderBatchSize-1; i++ {
					bc.Rollback([]common.Hash{bc.CurrentBlock().Hash()})
				}
			}
			return err
		}
	}

	_, err := bc.InsertChain([]*types.Block{block}, false /* verifyHeaders */)
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf(
				"[SYNC] UpdateBlockAndStatus: Error adding new block to blockchain %d %d",
				block.NumberU64(),
				block.ShardID(),
			)

		utils.Logger().Debug().
			Interface("block", bc.CurrentBlock()).
			Msg("[SYNC] UpdateBlockAndStatus: Rolling back current block!")
		bc.Rollback([]common.Hash{bc.CurrentBlock().Hash()})
		return err
	}
	utils.Logger().Info().
		Uint64("blockHeight", block.NumberU64()).
		Uint64("blockEpoch", block.Epoch().Uint64()).
		Str("blockHex", block.Hash().Hex()).
		Msg("[SYNC] UpdateBlockAndStatus: new block added to blockchain")
	for i, tx := range block.StakingTransactions() {
		utils.Logger().Info().
			Msgf(
				"StakingTxn %d: %s, %v", i, tx.StakingType().String(), tx.StakingMessage(),
			)
	}
	return nil
}

// generateNewState will construct most recent state from downloaded blocks
func (ss *StateSync) generateNewState(bc *core.BlockChain, worker *worker.Worker) error {
	// update blocks created before node start sync
	parentHash := bc.CurrentBlock().Hash()

	var err error
	for {
		block := ss.getBlockFromOldBlocksByParentHash(parentHash)
		if block == nil {
			break
		}
		err = ss.UpdateBlockAndStatus(block, bc, worker, false)
		if err != nil {
			break
		}
		parentHash = block.Hash()
	}
	ss.syncMux.Lock()
	ss.commonBlocks = make(map[int]*types.Block)
	ss.syncMux.Unlock()

	// update blocks after node start sync
	parentHash = bc.CurrentBlock().Hash()
	for {
		block := ss.getMaxConsensusBlockFromParentHash(parentHash)
		if block == nil {
			break
		}
		err = ss.UpdateBlockAndStatus(block, bc, worker, false)
		if err != nil {
			break
		}
		parentHash = block.Hash()
	}
	// TODO ek – Do we need to hold syncMux now that syncConfig has its own mutex?
	ss.syncMux.Lock()
	ss.syncConfig.ForEachPeer(func(peer *SyncPeerConfig) (brk bool) {
		peer.newBlocks = []*types.Block{}
		return
	})
	ss.syncMux.Unlock()

	// update last mile blocks if any
	parentHash = bc.CurrentBlock().Hash()
	for {
		block := ss.getBlockFromLastMileBlocksByParentHash(parentHash)
		if block == nil {
			break
		}
		err = ss.UpdateBlockAndStatus(block, bc, worker, false)
		if err != nil {
			break
		}
		parentHash = block.Hash()
	}

	return err
}

// ProcessStateSync processes state sync from the blocks received but not yet processed so far
func (ss *StateSync) ProcessStateSync(startHash []byte, size uint32, bc *core.BlockChain, worker *worker.Worker) error {
	// Gets consensus hashes.
	ss.getConsensusHashes(startHash, size)
	ss.generateStateSyncTaskQueue(bc)
	// Download blocks.
	if ss.stateSyncTaskQueue.Len() > 0 {
		ss.downloadBlocks(bc)
	}
	return ss.generateNewState(bc, worker)
}

func (peerConfig *SyncPeerConfig) registerToBroadcast(peerHash []byte, ip, port string) error {
	response := peerConfig.client.Register(peerHash, ip, port)
	if response == nil || response.Type == pb.DownloaderResponse_FAIL {
		return ErrRegistrationFail
	} else if response.Type == pb.DownloaderResponse_SUCCESS {
		return nil
	}
	return ErrRegistrationFail
}

// RegisterNodeInfo will register node to peers to accept future new block broadcasting
// return number of successful registration
func (ss *StateSync) RegisterNodeInfo() int {
	registrationNumber := RegistrationNumber
	utils.Logger().Debug().
		Int("registrationNumber", registrationNumber).
		Int("activePeerNumber", len(ss.syncConfig.peers)).
		Msg("[SYNC] node registration to peers")

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
				Msg("[SYNC] skip self")
			return
		}
		err := peerConfig.registerToBroadcast(ss.selfPeerHash[:], ss.selfip, ss.selfport)
		if err != nil {
			logger.Debug().
				Hex("selfPeerHash", ss.selfPeerHash[:]).
				Msg("[SYNC] register failed to peer")
			return
		}

		logger.Debug().Msg("[SYNC] register success")
		count++
		return
	})
	return count
}

// getMaxPeerHeight gets the maximum blockchain heights from peers
func (ss *StateSync) getMaxPeerHeight(isBeacon bool) uint64 {
	maxHeight := uint64(0)
	var wg sync.WaitGroup
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//debug
			// utils.Logger().Debug().Bool("isBeacon", isBeacon).Str("peerIP", peerConfig.ip).Str("peerPort", peerConfig.port).Msg("[Sync]getMaxPeerHeight")
			response, err := peerConfig.client.GetBlockChainHeight()
			if err != nil {
				utils.Logger().Warn().Err(err).Str("peerIP", peerConfig.ip).Str("peerPort", peerConfig.port).Msg("[Sync]GetBlockChainHeight failed")
				return
			}
			ss.syncMux.Lock()
			if response != nil && maxHeight < response.BlockHeight {
				maxHeight = response.BlockHeight
			}
			ss.syncMux.Unlock()
		}()
		return
	})
	wg.Wait()
	return maxHeight
}

// IsSameBlockchainHeight checks whether the node is out of sync from other peers
func (ss *StateSync) IsSameBlockchainHeight(bc *core.BlockChain) (uint64, bool) {
	otherHeight := ss.getMaxPeerHeight(false)
	currentHeight := bc.CurrentBlock().NumberU64()
	return otherHeight, currentHeight == otherHeight
}

// IsOutOfSync checks whether the node is out of sync from other peers
func (ss *StateSync) IsOutOfSync(bc *core.BlockChain) bool {
	otherHeight := ss.getMaxPeerHeight(false)
	currentHeight := bc.CurrentBlock().NumberU64()
	utils.Logger().Debug().
		Uint64("OtherHeight", otherHeight).
		Uint64("MyHeight", currentHeight).
		Bool("IsOutOfSync", currentHeight+inSyncThreshold < otherHeight).
		Msg("[SYNC] Checking sync status")
	return currentHeight+inSyncThreshold < otherHeight
}

// SyncLoop will keep syncing with peers until catches up
func (ss *StateSync) SyncLoop(bc *core.BlockChain, worker *worker.Worker, isBeacon bool, consensus *consensus.Consensus) {
	if !isBeacon {
		ss.RegisterNodeInfo()
	}
	// remove SyncLoopFrequency
	ticker := time.NewTicker(SyncLoopFrequency * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		otherHeight := ss.getMaxPeerHeight(isBeacon)
		currentHeight := bc.CurrentBlock().NumberU64()
		if currentHeight >= otherHeight {
			utils.Logger().Info().
				Msgf("[SYNC] Node is now IN SYNC! (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
					isBeacon, bc.ShardID(), otherHeight, currentHeight)
			return
		}
		utils.Logger().Debug().
			Msgf("[SYNC] Node is Not in Sync (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
				isBeacon, bc.ShardID(), otherHeight, currentHeight)

		startHash := bc.CurrentBlock().Hash()
		size := uint32(otherHeight - currentHeight)
		if size > SyncLoopBatchSize {
			size = SyncLoopBatchSize
		}
		err := ss.ProcessStateSync(startHash[:], size, bc, worker)
		if err != nil {
			utils.Logger().Error().Err(err).
				Msgf("[SYNC] ProcessStateSync failed (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
					isBeacon, bc.ShardID(), otherHeight, currentHeight)
		}
		ss.purgeOldBlocksFromCache()
		if consensus != nil {
			consensus.SetMode(consensus.UpdateConsensusInformation())
		}
	}
	ss.purgeAllBlocksFromCache()
}

// GetSyncingPort returns the syncing port.
func GetSyncingPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-SyncingPortDifference)
	}
	return ""
}
