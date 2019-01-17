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

	"github.com/Workiva/go-datastructures/queue"
	"github.com/ethereum/go-ethereum/log"
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
	blockHashes [][]byte
	newBlocks   []*types.Block
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
	return stateSync
}

// StateSync is the struct that implements StateSyncInterface.
type StateSync struct {
	selfip             string
	selfport           string
	peerNumber         int
	activePeerNumber   int
	blockHeight        int
	syncConfig         *SyncConfig
	stateSyncTaskQueue *queue.Queue
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
	Log.Debug("[sync] adding new block from peer", "peerHash", peerHash, "blockHash", block.Hash())
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
		Log.Debug("[sync] new block added, total blocks", "total", len(ss.syncConfig.peers[i].newBlocks))
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

// GetBlockHashes gets block hashes by calling grpc request to the corresponding peer.
func (peerConfig *SyncPeerConfig) GetBlockHashes() error {
	if peerConfig.client == nil {
		return ErrSyncPeerConfigClientNotReady
	}
	response := peerConfig.client.GetBlockHashes()
	peerConfig.blockHashes = make([][]byte, len(response.Payload))
	for i := range response.Payload {
		peerConfig.blockHashes[i] = make([]byte, len(response.Payload[i]))
		copy(peerConfig.blockHashes[i], response.Payload[i])
	}
	return nil
}

// GetBlocks gets blocks by calling grpc request to the corresponding peer.
func (peerConfig *SyncPeerConfig) GetBlocks(hashes [][]byte) ([][]byte, error) {
	if peerConfig.client == nil {
		return nil, ErrSyncPeerConfigClientNotReady
	}
	response := peerConfig.client.GetBlocks(hashes)
	return response.Payload, nil
}

// ProcessStateSyncFromPeers used to do state sync.
func (ss *StateSync) ProcessStateSyncFromPeers(peers []p2p.Peer, bc *core.BlockChain) (chan struct{}, error) {
	// TODO: Validate peers.
	done := make(chan struct{})
	go func() {
		ss.StartStateSync(peers, bc)
		done <- struct{}{}
	}()
	return done, nil
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
func (ss *StateSync) GetConsensusHashes() bool {
	count := 0
	for {
		var wg sync.WaitGroup
		wg.Add(ss.activePeerNumber)

		for id := range ss.syncConfig.peers {
			if ss.syncConfig.peers[id].client == nil {
				continue
			}
			go func(peerConfig *SyncPeerConfig) {
				defer wg.Done()
				response := peerConfig.client.GetBlockHashes()
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

// getConsensusHashes gets all hashes needed to download.
func (ss *StateSync) generateStateSyncTaskQueue(bc *core.BlockChain) {
	ss.stateSyncTaskQueue = queue.New(0)
	for _, configPeer := range ss.syncConfig.peers {
		if configPeer.client != nil {

			ss.blockHeight = len(configPeer.blockHashes)
			// TODO (minh) rework the syncing for account model.
			//bc.Blocks = append(bc.Blocks, make([]*blockchain.Block, ss.blockHeight-len(bc.Blocks))...)
			//for id, blockHash := range configPeer.blockHashes {
			//	if bc.Blocks[id] == nil || !reflect.DeepEqual(bc.Blocks[id].Hash[:], blockHash) {
			//		ss.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash})
			//		// TODO(minhdoan): Check error
			//	}
			//}
			break
		}
	}
	Log.Info("syncing: Finished generateStateSyncTaskQueue.")
}

// downloadBlocks downloads blocks from state sync task queue.
func (ss *StateSync) downloadBlocks(bc *core.BlockChain) {
	// Initialize blockchain
	var wg sync.WaitGroup
	wg.Add(ss.activePeerNumber)
	for i := range ss.syncConfig.peers {
		if ss.syncConfig.peers[i].client == nil {
			continue
		}
		go func(peerConfig *SyncPeerConfig, stateSyncTaskQueue *queue.Queue, bc *core.BlockChain) {
			defer wg.Done()
			for !stateSyncTaskQueue.Empty() {
				task, err := stateSyncTaskQueue.Poll(1, time.Millisecond)
				if err == queue.ErrTimeout {
					break
				}
				syncTask := task[0].(SyncBlockTask)
				for {
					//id := syncTask.index
					_, err := peerConfig.GetBlocks([][]byte{syncTask.blockHash})
					if err == nil {
						// As of now, only send and ask for one block.
						// TODO (minh) rework the syncing for account model.
						//bc.Blocks[id], err = blockchain.DeserializeBlock(payload[0])
						//_, err = blockchain.DeserializeBlock(payload[0])
						if err == nil {
							break
						}
					}
				}
			}
		}(ss.syncConfig.peers[i], ss.stateSyncTaskQueue, bc)
	}
	wg.Wait()
	Log.Info("[sync] Finished downloadBlocks.")
}

// StartStateSync starts state sync.
func (ss *StateSync) StartStateSync(peers []p2p.Peer, bc *core.BlockChain) bool {
	// Creates sync config.
	ss.CreateSyncConfig(peers)
	// Makes connections to peers.
	ss.MakeConnectionToPeers()
	numRegistered := ss.RegisterNodeInfo()
	Log.Info("[sync] waiting for broadcast from peers", "numRegistered", numRegistered)

	// Gets consensus hashes.
	//	if !ss.GetConsensusHashes() {
	//		return false
	//	}
	//ss.generateStateSyncTaskQueue(bc)
	// Download blocks.
	//if ss.stateSyncTaskQueue.Len() > 0 {
	//		ss.downloadBlocks(bc)
	//	}

	// TODO: change to true afterfinish implementation
	return false
}

func (peerConfig *SyncPeerConfig) registerToBroadcast(peerHash []byte) error {
	response := peerConfig.client.Register(peerHash)
	if response == nil || response.Type == pb.DownloaderResponse_FAIL {
		Log.Debug("[sync] register to broadcast fail")
		return ErrRegistrationFail
	} else if response.Type == pb.DownloaderResponse_SUCCESS {
		Log.Debug("[sync] register to broadcast success")
		return nil
	}
	Log.Debug("[sync] register to broadcast fail")
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
		Log.Debug("[sync] registering to peer", "ip", peerConfig.ip, "port", peerConfig.port, "peerHash", peerHash)
		err := peerConfig.registerToBroadcast(peerHash)
		if err != nil {
			Log.Debug("[sync] register FAILED to peer", "ip", peerConfig.ip, "port", peerConfig.port, "peerHash", peerHash)
			continue
		}
		count++
	}
	return count
}

func getPeerInfo(ip string, port string) []byte {
	peerStr := ip + ":" + port
	return []byte(peerStr)
}
