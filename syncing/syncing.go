package syncing

import (
	"bufio"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/p2p"
	proto_node "github.com/harmony-one/harmony/proto/node"
)

// SyncPeerConfig is peer config to sync.
type SyncPeerConfig struct {
	peer        p2p.Peer
	conn        net.Conn
	w           *bufio.Writer
	err         error
	trusted     bool
	blockHashes [][32]byte
}

// SyncBlockTask is the task struct to sync a specific block.
type SyncBlockTask struct {
	index     int
	blockHash [32]byte
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

// ProcessStateSyncFromSinglePeer used to do state sync from a single peer.
func (ss *StateSync) ProcessStateSyncFromSinglePeer(peer *p2p.Peer, bc *blockchain.Blockchain) (chan struct{}, error) {
	// Later.
	return nil, nil
}

func (ss *StateSync) createSyncConfig(peers []p2p.Peer) {
	ss.peerNumber = len(peers)
	ss.syncConfig = &SyncConfig{
		peers: make([]SyncPeerConfig, ss.peerNumber),
	}
	for id := range ss.syncConfig.peers {
		ss.syncConfig.peers[id].peer = peers[id]
		ss.syncConfig.peers[id].trusted = false
	}
}

func (ss *StateSync) makeConnectionToPeers() {
	var wg sync.WaitGroup
	wg.Add(ss.peerNumber)

	for _, synPeerConfig := range ss.syncConfig.peers {
		go func(peerConfig *SyncPeerConfig) {
			defer wg.Done()
			peerConfig.conn, peerConfig.err = p2p.DialWithSocketClient(peerConfig.peer.Ip, peerConfig.peer.Port)
		}(&synPeerConfig)
	}
	wg.Wait()
	ss.activePeerNumber = 0
	for _, configPeer := range ss.syncConfig.peers {
		if configPeer.err == nil {
			ss.activePeerNumber++
			configPeer.w = bufio.NewWriter(configPeer.conn)
			configPeer.trusted = true
		}
	}
}

// areConsensusHashesEqual chesk if all consensus hashes are equal.
func (ss *StateSync) areConsensusHashesEqual() bool {
	var fixedPeer *SyncPeerConfig
	for _, configPeer := range ss.syncConfig.peers {
		if configPeer.trusted {
			if fixedPeer == nil {
				fixedPeer = &configPeer
			}
			if !reflect.DeepEqual(configPeer.blockHashes, fixedPeer) {
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

		for _, configPeer := range ss.syncConfig.peers {
			if configPeer.err != nil {
				continue
			}
			go func(peerConfig *SyncPeerConfig) {
				defer wg.Done()
				msg := proto_node.ConstructBlockchainSyncMessage(proto_node.GetLastBlockHashes, [32]byte{})
				peerConfig.w.Write(msg)
				peerConfig.w.Flush()
				var content []byte
				content, peerConfig.err = p2p.ReadMessageContent(peerConfig.conn)
				if peerConfig.err != nil {
					peerConfig.trusted = false
					return
				}
				var blockchainSyncMessage *proto_node.BlockchainSyncMessage
				blockchainSyncMessage, peerConfig.err = proto_node.DeserializeBlockchainSyncMessage(content)
				if peerConfig.err != nil {
					peerConfig.trusted = false
					return
				}
				peerConfig.blockHashes = blockchainSyncMessage.BlockHashes
			}(&configPeer)
		}
		if ss.areConsensusHashesEqual() {
			break
		}
	}
}

// getConsensusHashes gets all hashes needed to download.
func (ss *StateSync) generateStateSyncTaskQueue() {
	ss.stateSyncTaskQueue = queue.New(0)
	for _, configPeer := range ss.syncConfig.peers {
		if configPeer.trusted {
			for id, blockHash := range configPeer.blockHashes {
				ss.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash})
			}
			ss.blockHeight = len(configPeer.blockHashes)
		}
	}
}

// downloadBlocks downloads blocks from state sync task queue.
func (ss *StateSync) downloadBlocks(bc *blockchain.Blockchain) {
	// Initialize blockchain
	bc.Blocks = make([]*blockchain.Block, ss.blockHeight)
	var wg sync.WaitGroup
	wg.Add(int(ss.stateSyncTaskQueue.Len()))
	for _, configPeer := range ss.syncConfig.peers {
		if configPeer.err != nil {
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
				msg := proto_node.ConstructBlockchainSyncMessage(proto_node.GetBlock, syncTask.blockHash)
				peerConfig.w.Write(msg)
				peerConfig.w.Flush()
				var content []byte
				content, peerConfig.err = p2p.ReadMessageContent(peerConfig.conn)
				if peerConfig.err != nil {
					peerConfig.trusted = false
					return
				}
				block, err := blockchain.DeserializeBlock(content)
				if err == nil {
					bc.Blocks[syncTask.index] = block
				}
			}
		}(&configPeer, ss.stateSyncTaskQueue, bc)
	}
	wg.Wait()
}

// StartStateSync starts state sync.
func (ss *StateSync) StartStateSync(peers []p2p.Peer, bc *blockchain.Blockchain) {
	// Creates sync config.
	ss.createSyncConfig(peers)

	// Makes connections to peers.
	ss.makeConnectionToPeers()

	// Gets consensus hashes.
	ss.getConsensusHashes()

	// Generates state-sync task queue.
	ss.generateStateSyncTaskQueue()

	ss.downloadBlocks(bc)
}

func getConsensus(syncConfig *SyncConfig) bool {
	return true
}
