package syncing

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
)

// Constants for syncing.
const (
	ConsensusRatio                        = float64(0.66)
	SleepTimeAfterNonConsensusBlockHashes = time.Second * 30
	TimesToFail                           = 5 // Downloadblocks service retry limit
	RegistrationNumber                    = 3
	SyncingPortDifference                 = 3000
	inSyncThreshold                       = 0 // when peerBlockHeight - myBlockHeight <= inSyncThreshold, it's ready to join consensus
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
}

// AddLastMileBlock add the lastest a few block into queue for syncing
func (ss *StateSync) AddLastMileBlock(block *types.Block) {
	ss.syncMux.Lock()
	defer ss.syncMux.Unlock()
	ss.lastMileBlocks = append(ss.lastMileBlocks, block)
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
		if bytes.Compare(pc.peerHash, peerHash) == 0 {
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
	utils.GetLogInstance().Debug("[SYNC] new block received", "total", len(pc.newBlocks), "blockHeight", block.NumberU64())
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
	utils.GetLogInstance().Debug("CreateSyncConfig: len of peers", "len", len(peers), "isBeacon", isBeacon)
	if len(peers) == 0 {
		return ctxerror.New("[SYNC] no peers to connect to")
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
	utils.GetLogInstance().Info("[SYNC] Finished making connection to peers.", "len", len(ss.syncConfig.peers), "isBeacon", isBeacon)

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

// GetBlockHashesConsensusAndCleanUp chesk if all consensus hashes are equal.
func (sc *SyncConfig) GetBlockHashesConsensusAndCleanUp() bool {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	// Sort all peers by the blockHashes.
	sort.Slice(sc.peers, func(i, j int) bool {
		return CompareSyncPeerConfigByblockHashes(sc.peers[i], sc.peers[j]) == -1
	})
	maxFirstID, maxCount := sc.getHowManyMaxConsensus()
	utils.GetLogInstance().Info("[SYNC] block consensus hashes", "maxFirstID", maxFirstID, "maxCount", maxCount)
	if float64(maxCount) >= ConsensusRatio*float64(len(sc.peers)) {
		sc.cleanUpPeers(maxFirstID)
		return true
	}
	return false
}

// GetConsensusHashes gets all hashes needed to download.
func (ss *StateSync) GetConsensusHashes(startHash []byte) bool {
	count := 0
	for {
		var wg sync.WaitGroup
		ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				response := peerConfig.client.GetBlockHashes(startHash)
				if response == nil {
					return
				}
				peerConfig.blockHashes = response.Payload
			}()
			return
		})
		wg.Wait()
		if ss.syncConfig.GetBlockHashesConsensusAndCleanUp() {
			break
		}
		if count > TimesToFail {
			utils.GetLogInstance().Info("GetConsensusHashes: reached retry limit")
			return false
		}
		count++
		time.Sleep(SleepTimeAfterNonConsensusBlockHashes)
	}
	utils.GetLogInstance().Info("syncing: Finished getting consensus block hashes.")
	return true
}

func (ss *StateSync) generateStateSyncTaskQueue(bc *core.BlockChain) {
	ss.stateSyncTaskQueue = queue.New(0)
	ss.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		for id, blockHash := range configPeer.blockHashes {
			ss.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash})
		}
		brk = true
		return
	})
	utils.GetLogInstance().Info("syncing: Finished generateStateSyncTaskQueue", "length", ss.stateSyncTaskQueue.Len())
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
					utils.GetLogInstance().Debug("[SYNC] ss.stateSyncTaskQueue poll timeout", "error", err)
					break
				}
				syncTask := task[0].(SyncBlockTask)
				//id := syncTask.index
				payload, err := peerConfig.GetBlocks([][]byte{syncTask.blockHash})
				if err != nil || len(payload) == 0 {
					count++
					utils.GetLogInstance().Debug("[SYNC] GetBlocks failed", "failNumber", count)
					if count > TimesToFail {
						break
					}
					ss.stateSyncTaskQueue.Put(syncTask)
					continue
				}

				var blockObj types.Block
				// currently only send one block a time
				err = rlp.DecodeBytes(payload[0], &blockObj)
				if err != nil {
					count++
					utils.GetLogInstance().Debug("[SYNC] downloadBlocks: failed to DecodeBytes from received new block")
					if count > TimesToFail {
						break
					}
					ss.stateSyncTaskQueue.Put(syncTask)
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
	utils.GetLogInstance().Info("[SYNC] Finished downloadBlocks.")
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
			if bytes.Compare(ph[:], parentHash[:]) == 0 {
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
	utils.GetLogInstance().Debug("[SYNC] Find block with matching parenthash", "parentHash", parentHash, "hash", candidateBlocks[maxFirstID].Hash(), "maxCount", maxCount)
	return candidateBlocks[maxFirstID]
}

func (ss *StateSync) getBlockFromOldBlocksByParentHash(parentHash common.Hash) *types.Block {
	for _, block := range ss.commonBlocks {
		ph := block.ParentHash()
		if bytes.Compare(ph[:], parentHash[:]) == 0 {
			return block
		}
	}
	return nil
}

func (ss *StateSync) getBlockFromLastMileBlocksByParentHash(parentHash common.Hash) *types.Block {
	for _, block := range ss.lastMileBlocks {
		ph := block.ParentHash()
		if bytes.Compare(ph[:], parentHash[:]) == 0 {
			return block
		}
	}
	return nil
}

func (ss *StateSync) updateBlockAndStatus(block *types.Block, bc *core.BlockChain, worker *worker.Worker) bool {
	utils.GetLogInstance().Info("[SYNC] Current Block", "blockHex", bc.CurrentBlock().Hash().Hex())
	_, err := bc.InsertChain([]*types.Block{block})
	if err != nil {
		utils.GetLogInstance().Debug("Error adding new block to blockchain", "Error", err)
		return false
	}
	ss.syncMux.Lock()
	worker.UpdateCurrent()
	ss.syncMux.Unlock()
	utils.GetLogInstance().Info("[SYNC] new block added to blockchain", "blockHeight", bc.CurrentBlock().NumberU64(), "blockHex", bc.CurrentBlock().Hash().Hex())
	return true
}

// generateNewState will construct most recent state from downloaded blocks
func (ss *StateSync) generateNewState(bc *core.BlockChain, worker *worker.Worker) {
	// update blocks created before node start sync
	parentHash := bc.CurrentBlock().Hash()
	for {
		block := ss.getBlockFromOldBlocksByParentHash(parentHash)
		if block == nil {
			break
		}
		ok := ss.updateBlockAndStatus(block, bc, worker)
		if !ok {
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
		ok := ss.updateBlockAndStatus(block, bc, worker)
		if !ok {
			break
		}
		parentHash = block.Hash()
	}
	// TODO ek – Do we need to hold syncMux now that syncConfig has its onw
	//  mutex?
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
		ok := ss.updateBlockAndStatus(block, bc, worker)
		if !ok {
			break
		}
		parentHash = block.Hash()
	}
}

// ProcessStateSync processes state sync from the blocks received but not yet processed so far
func (ss *StateSync) ProcessStateSync(startHash []byte, bc *core.BlockChain, worker *worker.Worker, isBeacon bool) {
	if !isBeacon {
		ss.RegisterNodeInfo()
	}
	// Gets consensus hashes.
	if !ss.GetConsensusHashes(startHash) {
		utils.GetLogInstance().Debug("[SYNC] ProcessStateSync unable to reach consensus on ss.GetConsensusHashes")
		return
	}
	ss.generateStateSyncTaskQueue(bc)
	// Download blocks.
	if ss.stateSyncTaskQueue.Len() > 0 {
		ss.downloadBlocks(bc)
	}
	ss.generateNewState(bc, worker)
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
	utils.GetLogInstance().Debug("[SYNC] node registration to peers",
		"registrationNumber", registrationNumber,
		"activePeerNumber", len(ss.syncConfig.peers))

	count := 0
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		if count >= registrationNumber {
			brk = true
			return
		}
		if peerConfig.ip == ss.selfip && peerConfig.port == GetSyncingPort(ss.selfport) {
			utils.GetLogInstance().Debug("[SYNC] skip self", "peerport", peerConfig.port, "selfport", ss.selfport, "selfsyncport", GetSyncingPort(ss.selfport))
			return
		}
		err := peerConfig.registerToBroadcast(ss.selfPeerHash[:], ss.selfip, ss.selfport)
		if err != nil {
			utils.GetLogInstance().Debug("[SYNC] register failed to peer", "ip", peerConfig.ip, "port", peerConfig.port, "selfPeerHash", ss.selfPeerHash)
			return
		}
		utils.GetLogInstance().Debug("[SYNC] register success", "ip", peerConfig.ip, "port", peerConfig.port)
		count++
		return
	})
	return count
}

// getMaxPeerHeight gets the maximum blockchain heights from peers
func (ss *StateSync) getMaxPeerHeight() uint64 {
	maxHeight := uint64(0)
	var wg sync.WaitGroup
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := peerConfig.client.GetBlockChainHeight()
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

// IsOutOfSync checks whether the node is out of sync from other peers
func (ss *StateSync) IsOutOfSync(bc *core.BlockChain) bool {
	otherHeight := ss.getMaxPeerHeight()
	currentHeight := bc.CurrentBlock().NumberU64()
	utils.GetLogInstance().Debug("[SYNC] IsOutOfSync", "otherHeight", otherHeight, "myHeight", currentHeight)
	return currentHeight+inSyncThreshold < otherHeight
}

// SyncLoop will keep syncing with peers until catches up
func (ss *StateSync) SyncLoop(bc *core.BlockChain, worker *worker.Worker, willJoinConsensus bool, isBeacon bool) {
	for {
		if !ss.IsOutOfSync(bc) {
			utils.GetLogInstance().Info("[SYNC] Node is now IN SYNC!")
			return
		}
		startHash := bc.CurrentBlock().Hash()
		ss.ProcessStateSync(startHash[:], bc, worker, isBeacon)
	}
}

// GetSyncingPort returns the syncing port.
func GetSyncingPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-SyncingPortDifference)
	}
	return ""
}
