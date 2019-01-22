package syncing

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/services/syncing/downloader"
	pb "github.com/harmony-one/harmony/api/services/syncing/downloader/proto"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/p2p"
)

// Constants for syncing.
const (
	ConsensusRatio                        = float64(0.66)
	SleepTimeAfterNonConsensusBlockHashes = time.Second * 30
	TimesToFail                           = 5
	RegistrationNumber                    = 3
	SyncingPortDifference                 = 3000
)

// SyncPeerConfig is peer config to sync.
type SyncPeerConfig struct {
	ip          string
	port        string
	client      *downloader.Client
	blockHashes [][]byte       // block hashes before node doing sync
	newBlocks   []*types.Block // blocks after node doing sync
	mux         sync.Mutex
}

// GetClient returns client pointer of downloader.Client
func (peerConfig *SyncPeerConfig) GetClient() *downloader.Client {
	return peerConfig.client
}

// Log is the temporary log for syncing.
var Log = log.New()

// SyncBlockTask is the task struct to sync a specific block.
type SyncBlockTask struct {
	index     int
	blockHash []byte
}

// SyncConfig contains an array of SyncPeerConfig.
type SyncConfig struct {
	peers []*SyncPeerConfig
}

// GetStateSync returns the implementation of StateSyncInterface interface.
func CreateStateSync(ip string, port string) *StateSync {
	stateSync := &StateSync{}
	stateSync.selfip = ip
	stateSync.selfport = port
	stateSync.commonBlocks = make(map[int]*types.Block)
	return stateSync
}

// StateSync is the struct that implements StateSyncInterface.
type StateSync struct {
	selfip             string
	selfport           string
	peerNumber         int
	activePeerNumber   int
	commonBlocks       map[int]*types.Block
	syncConfig         *SyncConfig
	stateSyncTaskQueue *queue.Queue
	syncMux            sync.Mutex
}

// GetServicePort returns the service port from syncing port
// TODO: really need use a unique ID instead of ip/port
func GetServicePort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port+SyncingPortDifference)
	}
	os.Exit(1)
	return ""
}

// AddNewBlock will add newly received block into state syncing queue
func (ss *StateSync) AddNewBlock(peerHash []byte, block *types.Block) {
	for i, pc := range ss.syncConfig.peers {
		pid := utils.GetUniqueIDFromIPPort(pc.ip, GetServicePort(pc.port))
		ph := make([]byte, 4)
		binary.BigEndian.PutUint32(ph, pid)
		if bytes.Compare(ph, peerHash) != 0 {
			continue
		}
		pc.mux.Lock()
		pc.newBlocks = append(pc.newBlocks, block)
		pc.mux.Unlock()
		Log.Debug("[sync] new block [pre]added", "total", len(ss.syncConfig.peers[i].newBlocks), "blockHeight", block.NumberU64())
	}
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
	if peerConfig.client == nil {
		return nil, ErrSyncPeerConfigClientNotReady
	}
	response := peerConfig.client.GetBlocks(hashes)
	if response == nil {
		return nil, ErrGetBlock
	}
	return response.Payload, nil
}

// CreateSyncConfig creates SyncConfig for StateSync object.
func (ss *StateSync) CreateSyncConfig(peers []p2p.Peer) {
	Log.Debug("CreateSyncConfig: len of peers", "len", len(peers))
	Log.Debug("CreateSyncConfig: len of peers", "peers", peers)
	ss.peerNumber = len(peers)
	ss.syncConfig = &SyncConfig{
		peers: make([]*SyncPeerConfig, ss.peerNumber),
	}
	for id := range ss.syncConfig.peers {
		ss.syncConfig.peers[id] = &SyncPeerConfig{
			ip:   peers[id].IP,
			port: peers[id].Port,
		}
		Log.Debug("[sync] CreateSyncConfig: peer port to connect", "port", peers[id].Port)
	}
	Log.Info("[sync] syncing: Finished creating SyncConfig.")
}

// MakeConnectionToPeers makes grpc connection to all peers.
func (ss *StateSync) MakeConnectionToPeers() {
	var wg sync.WaitGroup
	wg.Add(ss.peerNumber)

	for id := range ss.syncConfig.peers {
		go func(peerConfig *SyncPeerConfig) {
			defer wg.Done()
			peerConfig.client = downloader.ClientSetup(peerConfig.ip, peerConfig.port)
		}(ss.syncConfig.peers[id])
	}
	wg.Wait()
	ss.CleanUpNilPeers()
	Log.Info("syncing: Finished making connection to peers.")
}

// CleanUpNilPeers cleans up peer with nil client and recalculate activePeerNumber.
func (ss *StateSync) CleanUpNilPeers() {
	ss.activePeerNumber = 0
	for _, configPeer := range ss.syncConfig.peers {
		if configPeer.client != nil {
			ss.activePeerNumber++
		}
	}
}

// GetHowManyMaxConsensus returns max number of consensus nodes and the first ID of consensus group.
// Assumption: all peers are sorted by CompareSyncPeerConfigByBlockHashes first.
func (syncConfig *SyncConfig) GetHowManyMaxConsensus() (int, int) {
	// As all peers are sorted by their blockHashes, all equal blockHashes should come together and consecutively.
	curCount := 0
	curFirstID := -1
	maxCount := 0
	maxFirstID := -1
	for i := range syncConfig.peers {
		if curFirstID == -1 || CompareSyncPeerConfigByblockHashes(syncConfig.peers[curFirstID], syncConfig.peers[i]) != 0 {
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
func (syncConfig *SyncConfig) InitForTesting(client *downloader.Client, blockHashes [][]byte) {
	for i := range syncConfig.peers {
		syncConfig.peers[i].blockHashes = blockHashes
		syncConfig.peers[i].client = client
	}
}

// CleanUpPeers cleans up all peers whose blockHashes are not equal to consensus block hashes.
func (syncConfig *SyncConfig) CleanUpPeers(maxFirstID int) {
	fixedPeer := syncConfig.peers[maxFirstID]
	for i := 0; i < len(syncConfig.peers); i++ {
		if CompareSyncPeerConfigByblockHashes(fixedPeer, syncConfig.peers[i]) != 0 {
			// TODO: move it into a util delete func.
			// See tip https://github.com/golang/go/wiki/SliceTricks
			// Close the client and remove the peer out of the
			syncConfig.peers[i].client.Close()
			copy(syncConfig.peers[i:], syncConfig.peers[i+1:])
			syncConfig.peers[len(syncConfig.peers)-1] = nil
			syncConfig.peers = syncConfig.peers[:len(syncConfig.peers)-1]
		}
	}
}

// GetBlockHashesConsensusAndCleanUp chesk if all consensus hashes are equal.
func (ss *StateSync) GetBlockHashesConsensusAndCleanUp() bool {
	// Sort all peers by the blockHashes.
	sort.Slice(ss.syncConfig.peers, func(i, j int) bool {
		return CompareSyncPeerConfigByblockHashes(ss.syncConfig.peers[i], ss.syncConfig.peers[j]) == -1
	})
	maxFirstID, maxCount := ss.syncConfig.GetHowManyMaxConsensus()
	if float64(maxCount) >= ConsensusRatio*float64(ss.activePeerNumber) {
		ss.syncConfig.CleanUpPeers(maxFirstID)
		ss.CleanUpNilPeers()
		return true
	}
	return false
}

// GetConsensusHashes gets all hashes needed to download.
func (ss *StateSync) GetConsensusHashes(startHash []byte) bool {
	count := 0
	for {
		var wg sync.WaitGroup
		for id := range ss.syncConfig.peers {
			if ss.syncConfig.peers[id].client == nil {
				continue
			}
			wg.Add(1)
			go func(peerConfig *SyncPeerConfig) {
				defer wg.Done()
				response := peerConfig.client.GetBlockHashes(startHash)
				if response == nil {
					return
				}
				peerConfig.blockHashes = response.Payload
			}(ss.syncConfig.peers[id])
		}
		wg.Wait()
		if ss.GetBlockHashesConsensusAndCleanUp() {
			break
		}
		if count > TimesToFail {
			Log.Info("GetConsensusHashes: reached # of times to failed")
			return false
		}
		count++
		time.Sleep(SleepTimeAfterNonConsensusBlockHashes)
	}
	Log.Info("syncing: Finished getting consensus block hashes.")
	return true
}

func (ss *StateSync) generateStateSyncTaskQueue(bc *core.BlockChain) {
	ss.stateSyncTaskQueue = queue.New(0)
	for _, configPeer := range ss.syncConfig.peers {
		if configPeer.client != nil {
			for id, blockHash := range configPeer.blockHashes {
				ss.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash})
			}
			break
		}
	}
	Log.Info("syncing: Finished generateStateSyncTaskQueue", "length", ss.stateSyncTaskQueue.Len())
}

// downloadBlocks downloads blocks from state sync task queue.
func (ss *StateSync) downloadBlocks(bc *core.BlockChain) {
	// Initialize blockchain
	var wg sync.WaitGroup
	wg.Add(ss.activePeerNumber)
	count := 0
	for i := range ss.syncConfig.peers {
		if ss.syncConfig.peers[i].client == nil {
			continue
		}
		go func(peerConfig *SyncPeerConfig, stateSyncTaskQueue *queue.Queue, bc *core.BlockChain) {
			defer wg.Done()
			for !stateSyncTaskQueue.Empty() {
				task, err := ss.stateSyncTaskQueue.Poll(1, time.Millisecond)
				if err == queue.ErrTimeout {
					Log.Debug("[sync] ss.stateSyncTaskQueue poll timeout", "error", err)
					break
				}
				syncTask := task[0].(SyncBlockTask)
				//id := syncTask.index
				payload, err := peerConfig.GetBlocks([][]byte{syncTask.blockHash})
				if err != nil {
					count++
					Log.Debug("[sync] GetBlocks failed", "failNumber", count)
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
					Log.Debug("[sync] downloadBlocks: failed to DecodeBytes from received new block")
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
		}(ss.syncConfig.peers[i], ss.stateSyncTaskQueue, bc)
	}
	wg.Wait()
	Log.Info("[sync] Finished downloadBlocks.")
}

func CompareBlockByHash(a *types.Block, b *types.Block) int {
	ha := a.Hash()
	hb := b.Hash()
	return bytes.Compare(ha[:], hb[:])
}

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
	for id := range ss.syncConfig.peers {
		peerConfig := ss.syncConfig.peers[id]
		for _, block := range peerConfig.newBlocks {
			ph := block.ParentHash()
			if bytes.Compare(ph[:], parentHash[:]) == 0 {
				candidateBlocks = append(candidateBlocks, block)
				break
			}
		}
	}
	ss.syncMux.Unlock()
	if len(candidateBlocks) == 0 {
		return nil
	}
	// Sort by blockHashes.
	sort.Slice(candidateBlocks, func(i, j int) bool {
		return CompareBlockByHash(candidateBlocks[i], candidateBlocks[j]) == -1
	})
	maxFirstID, maxCount := GetHowManyMaxConsensus(candidateBlocks)
	Log.Debug("[sync] Find block with matching parenthash", "parentHash", parentHash, "hash", candidateBlocks[maxFirstID].Hash(), "maxCount", maxCount)
	return candidateBlocks[maxFirstID]
}

func (ss *StateSync) getBlockFromParentHash(parentHash common.Hash) *types.Block {
	for _, block := range ss.commonBlocks {
		ph := block.ParentHash()
		if bytes.Compare(ph[:], parentHash[:]) == 0 {
			return block
		}
	}
	return nil
}

func (ss *StateSync) updateBlockAndStatus(block *types.Block, bc *core.BlockChain, worker *worker.Worker) bool {
	Log.Info("[sync] Current Block", "blockHex", bc.CurrentBlock().Hash().Hex())
	_, err := bc.InsertChain([]*types.Block{block})
	if err != nil {
		Log.Debug("Error adding new block to blockchain", "Error", err)
		return false
	}
	Log.Info("[sync] new block [POST]added to blockchain", "blockHeight", bc.CurrentBlock().NumberU64(), "blockHex", bc.CurrentBlock().Hash().Hex(), "parentHex", bc.CurrentBlock().ParentHash().Hex())
	ss.syncMux.Lock()
	worker.UpdateCurrent()
	ss.syncMux.Unlock()
	return true
}

// generateNewState will construct most recent state from downloaded blocks
func (ss *StateSync) generateNewState(bc *core.BlockChain, worker *worker.Worker) {
	// update blocks created before node start sync
	parentHash := bc.CurrentBlock().Hash()
	for {
		block := ss.getBlockFromParentHash(parentHash)
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
	ss.syncMux.Lock()
	for id := range ss.syncConfig.peers {
		ss.syncConfig.peers[id].newBlocks = []*types.Block{}
	}
	ss.syncMux.Unlock()
}

// StartStateSync starts state sync.
func (ss *StateSync) StartStateSync(startHash []byte, peers []p2p.Peer, bc *core.BlockChain, worker *worker.Worker) {
	// Creates sync config.
	ss.CreateSyncConfig(peers)
	// Makes connections to peers.
	ss.MakeConnectionToPeers()
	numRegistered := ss.RegisterNodeInfo()
	Log.Info("[sync] waiting for broadcast from peers", "numRegistered", numRegistered)

	// Gets consensus hashes.
	if !ss.GetConsensusHashes(startHash) {
		Log.Debug("[sync] StartStateSync unable to reach consensus on ss.GetConsensusHashes")
		return
	}
	Log.Debug("[sync] StartStateSync reach consensus on ss.GetConsensusHashes")
	ss.generateStateSyncTaskQueue(bc)
	// Download blocks.
	if ss.stateSyncTaskQueue.Len() > 0 {
		ss.downloadBlocks(bc)
	}
	ss.generateNewState(bc, worker)
}

func (peerConfig *SyncPeerConfig) registerToBroadcast(peerHash []byte) error {
	response := peerConfig.client.Register(peerHash)
	if response == nil || response.Type == pb.DownloaderResponse_FAIL {
		return ErrRegistrationFail
	} else if response.Type == pb.DownloaderResponse_SUCCESS {
		return nil
	}
	return ErrRegistrationFail
}

// RegisterNodeInfo will register node to peers to accept future new block broadcasting
// return number of successfull registration
func (ss *StateSync) RegisterNodeInfo() int {
	ss.CleanUpNilPeers()
	registrationNumber := RegistrationNumber
	Log.Debug("[sync]", "registrationNumber", registrationNumber, "activePeerNumber", ss.activePeerNumber)
	peerID := utils.GetUniqueIDFromIPPort(ss.selfip, ss.selfport)
	peerHash := make([]byte, 4)
	binary.BigEndian.PutUint32(peerHash[:], peerID)

	count := 0
	for id := range ss.syncConfig.peers {
		peerConfig := ss.syncConfig.peers[id]
		if count >= registrationNumber {
			break
		}
		if peerConfig.client == nil {
			continue
		}
		err := peerConfig.registerToBroadcast(peerHash)
		if err != nil {
			Log.Debug("[sync] register failed to peer", "ip", peerConfig.ip, "port", peerConfig.port, "peerHash", peerHash)
			continue
		}
		Log.Debug("[sync] register success", "ip", peerConfig.ip, "port", peerConfig.port)
		count++
	}
	return count
}
