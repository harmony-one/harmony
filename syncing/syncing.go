package syncing

import (
	"reflect"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/syncing/downloader"
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
	peers []SyncPeerConfig
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

// GetBlockHashes ...
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

// GetBlocks ...
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

// CreateSyncConfig ...
func (ss *StateSync) CreateSyncConfig(peers []p2p.Peer) {
	ss.peerNumber = len(peers)
	ss.syncConfig = &SyncConfig{
		peers: make([]SyncPeerConfig, ss.peerNumber),
	}
	for id := range ss.syncConfig.peers {
		ss.syncConfig.peers[id] = SyncPeerConfig{
			ip:   peers[id].Ip,
			port: peers[id].Port,
		}
	}
}

func (ss *StateSync) makeConnectionToPeers() {
	var wg sync.WaitGroup
	wg.Add(ss.peerNumber)

	for id := range ss.syncConfig.peers {
		go func(peerConfig *SyncPeerConfig) {
			defer wg.Done()
			peerConfig.client = downloader.ClientSetup(peerConfig.ip, peerConfig.port)
		}(&ss.syncConfig.peers[id])
	}
	wg.Wait()
	ss.activePeerNumber = 0
	for _, configPeer := range ss.syncConfig.peers {
		if configPeer.client != nil {
			ss.activePeerNumber++
		}
	}
}

// areConsensusHashesEqual chesk if all consensus hashes are equal.
func (ss *StateSync) areConsensusHashesEqual() bool {
	var firstPeer *SyncPeerConfig
	for _, configPeer := range ss.syncConfig.peers {
		if configPeer.client != nil {
			if firstPeer == nil {
				firstPeer = &configPeer
			}
			if !reflect.DeepEqual(configPeer.blockHashes, firstPeer.blockHashes) {
				return false
			}
		}
	}
	return true
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
				peerConfig.client.GetBlockHashes()
			}(&ss.syncConfig.peers[id])
		}
		wg.Wait()
		if ss.areConsensusHashesEqual() {
			break
		}
	}
}

// getConsensusHashes gets all hashes needed to download.
func (ss *StateSync) generateStateSyncTaskQueue() {
	ss.stateSyncTaskQueue = queue.New(0)
	for _, configPeer := range ss.syncConfig.peers {
		if configPeer.client != nil {
			for id, blockHash := range configPeer.blockHashes {
				ss.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash})
			}
			ss.blockHeight = len(configPeer.blockHashes)
			break
		}
	}
}

// downloadBlocks downloads blocks from state sync task queue.
func (ss *StateSync) downloadBlocks(bc *blockchain.Blockchain) {
	// Initialize blockchain
	bc.Blocks = make([]*blockchain.Block, ss.blockHeight)
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
		}(&ss.syncConfig.peers[i], ss.stateSyncTaskQueue, bc)
	}
	wg.Wait()
}

// StartStateSync starts state sync.
func (ss *StateSync) StartStateSync(peers []p2p.Peer, bc *blockchain.Blockchain) {
	// Creates sync config.
	ss.CreateSyncConfig(peers)

	// Makes connections to peers.
	ss.makeConnectionToPeers()

	// Gets consensus hashes.
	ss.getConsensusHashes()

	// Generates state-sync task queue.
	ss.generateStateSyncTaskQueue()

	ss.downloadBlocks(bc)
}
