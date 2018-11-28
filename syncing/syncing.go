package syncing

import (
	"bytes"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/syncing/downloader"
)

// Constants for syncing.
const (
	ConsensusRatio = float64(0.66)
)

// SyncPeerConfig is peer config to sync.
type SyncPeerConfig struct {
	ip          string
	port        string
	client      *downloader.Client
	blockHashes [][]byte
}

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
func GetStateSync() *StateSync {
	return &StateSync{}
}

// StateSync is the struct that implements StateSyncInterface.
type StateSync struct {
	peerNumber         int
	activePeerNumber   int
	blockHeight        int
	syncConfig         *SyncConfig
	stateSyncTaskQueue *queue.Queue
}

func compareSyncPeerConfigByBlockHashes(a *SyncPeerConfig, b *SyncPeerConfig) int {
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
func (ss *StateSync) ProcessStateSyncFromPeers(peers []p2p.Peer, bc *blockchain.Blockchain) (chan struct{}, error) {
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
	ss.peerNumber = len(peers)
	ss.syncConfig = &SyncConfig{
		peers: make([]*SyncPeerConfig, ss.peerNumber),
	}
	for id := range ss.syncConfig.peers {
		ss.syncConfig.peers[id] = &SyncPeerConfig{
			ip:   peers[id].Ip,
			port: peers[id].Port,
		}
	}
}

// makeConnectionToPeers makes grpc connection to all peers.
func (ss *StateSync) makeConnectionToPeers() {
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

// getHowMaxConsensus returns max number of consensus nodes and the first ID of consensus group.
// Assumption: all peers are sorted by compareSyncPeerConfigByBlockHashes first.
func (syncConfig *SyncConfig) getHowMaxConsensus() (int, int) {
	// As all peers are sorted by their blockHashes, all equal blockHashes should come together and consecutively.
	curCount := 0
	curFirstID := -1
	maxCount := 0
	maxFirstID := -1
	for i := range syncConfig.peers {
		if curFirstID == -1 || compareSyncPeerConfigByBlockHashes(syncConfig.peers[curFirstID], syncConfig.peers[i]) != 0 {
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

// CleanUpPeers cleans up all peers whose blockHashes are not equal to consensus block hashes.
func (syncConfig *SyncConfig) CleanUpPeers(maxFirstID int) {
	for i := range syncConfig.peers {
		if compareSyncPeerConfigByBlockHashes(syncConfig.peers[maxFirstID], syncConfig.peers[i]) != 0 {
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

// getBlockHashesConsensusAndCleanUp chesk if all consensus hashes are equal.
func (ss *StateSync) getBlockHashesConsensusAndCleanUp() bool {
	// Sort all peers by the blockHashes.
	sort.Slice(ss.syncConfig.peers, func(i, j int) bool {
		return compareSyncPeerConfigByBlockHashes(ss.syncConfig.peers[i], ss.syncConfig.peers[j]) == -1
	})
	maxCount, maxFirstID := ss.syncConfig.getHowMaxConsensus()
	if float64(maxCount) >= ConsensusRatio*float64(ss.activePeerNumber) {
		ss.syncConfig.CleanUpPeers(maxFirstID)
		ss.CleanUpNilPeers()
		return true
	}
	return false
}

// getConsensusHashes gets all hashes needed to download.
func (ss *StateSync) getConsensusHashes() {
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
		if ss.getBlockHashesConsensusAndCleanUp() {
			break
		}
	}
}

// getConsensusHashes gets all hashes needed to download.
func (ss *StateSync) generateStateSyncTaskQueue(bc *blockchain.Blockchain) {
	ss.stateSyncTaskQueue = queue.New(0)
	for _, configPeer := range ss.syncConfig.peers {
		if configPeer.client != nil {

			ss.blockHeight = len(configPeer.blockHashes)
			bc.Blocks = append(bc.Blocks, make([]*blockchain.Block, ss.blockHeight-len(bc.Blocks))...)
			for id, blockHash := range configPeer.blockHashes {
				if bc.Blocks[id] == nil || !reflect.DeepEqual(bc.Blocks[id].Hash[:], blockHash) {
					ss.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash})
				}
			}
			break
		}
	}
}

// downloadBlocks downloads blocks from state sync task queue.
func (ss *StateSync) downloadBlocks(bc *blockchain.Blockchain) {
	// Initialize blockchain
	var wg sync.WaitGroup
	wg.Add(ss.activePeerNumber)
	for i := range ss.syncConfig.peers {
		if ss.syncConfig.peers[i].client == nil {
			continue
		}
		go func(peerConfig *SyncPeerConfig, stateSyncTaskQueue *queue.Queue, bc *blockchain.Blockchain) {
			defer wg.Done()
			for !stateSyncTaskQueue.Empty() {
				task, err := stateSyncTaskQueue.Poll(1, time.Millisecond)
				if err == queue.ErrTimeout {
					break
				}
				syncTask := task[0].(SyncBlockTask)
				for {
					id := syncTask.index
					payload, err := peerConfig.GetBlocks([][]byte{syncTask.blockHash})
					if err == nil {
						// As of now, only send and ask for one block.
						bc.Blocks[id], err = blockchain.DeserializeBlock(payload[0])
						if err == nil {
							break
						}
					}
				}
			}
		}(ss.syncConfig.peers[i], ss.stateSyncTaskQueue, bc)
	}
	wg.Wait()
}

// StartStateSync starts state sync.
func (ss *StateSync) StartStateSync(peers []p2p.Peer, bc *blockchain.Blockchain) {
	// Creates sync config.
	ss.CreateSyncConfig(peers)
	// Makes connections to peers.
	ss.makeConnectionToPeers()
	for {
		// Gets consensus hashes.
		ss.getConsensusHashes()

		// Generates state-sync task queue.
		ss.generateStateSyncTaskQueue(bc)

		// Download blocks.
		if ss.stateSyncTaskQueue.Len() > 0 {
			ss.downloadBlocks(bc)
		} else {
			break
		}
	}
}
