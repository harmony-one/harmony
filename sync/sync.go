package sync

import (
	"bufio"
	"net"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/node"
	"github.com/simple-rules/harmony-benchmark/p2p"
	proto_node "github.com/simple-rules/harmony-benchmark/proto/node"
)

type SyncPeerConfig struct {
	peer        p2p.Peer
	conn        net.Conn
	block       *blockchain.Block
	w           *bufio.Writer
	receivedMsg []byte
	err         error
	trusted     bool
	indexes     []uint16
	blockHashes [][32]byte
}

type SyncBlockTask struct {
	index     int
	blockHash [32]byte
}
type SyncConfig struct {
	peers []SyncPeerConfig
}

func StartBlockSyncing(node *node.Node) *blockchain.Blockchain {
	peers := node.Consensus.GetValidatorPeers()
	peer_number := len(peers)
	syncConfig := SyncConfig{
		peers: make([]SyncPeerConfig, peer_number),
	}
	for id := range syncConfig.peers {
		syncConfig.peers[id].peer = peers[id]
		syncConfig.peers[id].trusted = false
	}

	var wg sync.WaitGroup
	wg.Add(peer_number)

	for id := range syncConfig.peers {
		go func(peerConfig *SyncPeerConfig) {
			defer wg.Done()
			peerConfig.conn, peerConfig.err = p2p.DialWithSocketClient(peerConfig.peer.Ip, peerConfig.peer.Port)
		}(&syncConfig.peers[id])
	}
	wg.Wait()

	activePeerNumber := 0
	for _, configPeer := range syncConfig.peers {
		if configPeer.err == nil {
			activePeerNumber++
			configPeer.w = bufio.NewWriter(configPeer.conn)
			configPeer.trusted = true
		}
	}

	// Looping to get an array of block hashes from honest nodes.
LOOP_HONEST_NODE:
	for {
		var wg sync.WaitGroup
		wg.Add(activePeerNumber)

		for _, configPeer := range syncConfig.peers {
			if configPeer.err != nil {
				continue
			}
			go func(peerConfig *SyncPeerConfig) {
				defer wg.Done()
				msg := proto_node.ConstructBlockchainSyncMessage(proto_node.GET_LAST_BLOCK_HASHES, [32]byte{})
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
		wg.Wait()

		if getConsensus(&syncConfig) {
			break LOOP_HONEST_NODE
		}
	}

	taskSyncQueue := queue.New(0)
	blockSize := 0
TASK_LOOP:
	for _, configPeer := range syncConfig.peers {
		if configPeer.trusted {
			for id, blockHash := range configPeer.blockHashes {
				taskSyncQueue.Put(SyncBlockTask{index: id, blockHash: blockHash})
			}
			blockSize = len(configPeer.blockHashes)
			break TASK_LOOP
		}
	}
	// Initialize blockchain
	bc := &blockchain.Blockchain{
		Blocks: make([]*blockchain.Block, blockSize),
	}
	// loop to do syncing.
	for {
		var wg sync.WaitGroup
		wg.Add(activePeerNumber)

		for _, configPeer := range syncConfig.peers {
			if configPeer.err != nil {
				continue
			}
			go func(peerConfig *SyncPeerConfig, taskSyncQueue *queue.Queue, bc *blockchain.Blockchain) {
				defer wg.Done()
				for !taskSyncQueue.Empty() {
					task, err := taskSyncQueue.Poll(1, time.Millisecond)
					if err == queue.ErrTimeout {
						break
					}
					syncTask := task[0].(SyncBlockTask)
					msg := proto_node.ConstructBlockchainSyncMessage(proto_node.GET_BLOCK, syncTask.blockHash)
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
			}(&configPeer, taskSyncQueue, bc)
		}
		wg.Wait()
	}
}

func getConsensus(syncConfig *SyncConfig) bool {
	return true
}
