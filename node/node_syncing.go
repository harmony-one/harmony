
import (
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/internal/utils"
)

// DoSyncing wait for check status and starts syncing if out of sync
func (node *Node) DoSyncing() {
	for {
		select {
		// in current implementation logic, timeout means in sync
		case <-time.After(5 * time.Second):
			//myHeight := node.blockchain.CurrentBlock().NumberU64()
			//utils.GetLogInstance().Debug("[SYNC]", "currentHeight", myHeight)
			node.stateMutex.Lock()
			node.State = NodeReadyForConsensus
			node.stateMutex.Unlock()
			continue
		case consensusBlockInfo := <-node.Consensus.ConsensusBlock:
			if !node.IsOutOfSync(consensusBlockInfo) {
				startHash := node.blockchain.CurrentBlock().Hash()
				node.stateSync.StartStateSync(startHash[:], node.blockchain, node.Worker)
				if node.State == NodeNotInSync {
					utils.GetLogInstance().Info("[SYNC] Node is now IN SYNC!")
				}
				node.stateMutex.Lock()
				node.State = NodeReadyForConsensus
				node.stateMutex.Unlock()
				node.stateSync.CloseConnections()
				node.stateSync = nil
				continue
			} else {
				utils.GetLogInstance().Debug("[SYNC] node is out of sync")
				node.stateMutex.Lock()
				node.State = NodeNotInSync
				node.stateMutex.Unlock()
			}

			if node.stateSync == nil {
				node.stateSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port)
				node.stateSync.CreateSyncConfig(node.GetSyncingPeers())
				node.stateSync.MakeConnectionToPeers()
			}
			startHash := node.blockchain.CurrentBlock().Hash()
			node.stateSync.StartStateSync(startHash[:], node.blockchain, node.Worker)
		}
	}
}

// SupportSyncing keeps sleeping until it's doing consensus or it's a leader.
func (node *Node) SupportSyncing() {
	node.InitSyncingServer()
	node.StartSyncingServer()

	go node.DoSyncing()
	go node.SendNewBlockToUnsync()
}

// InitSyncingServer starts downloader server.
func (node *Node) InitSyncingServer() {
	node.downloaderServer = downloader.NewServer(node)
}

// StartSyncingServer starts syncing server.
func (node *Node) StartSyncingServer() {
	utils.GetLogInstance().Info("support_sycning: StartSyncingServer")
	node.downloaderServer.Start(node.SelfPeer.IP, GetSyncingPort(node.SelfPeer.Port))
}

// SendNewBlockToUnsync send latest verified block to unsync, registered nodes
func (node *Node) SendNewBlockToUnsync() {
	for {
		block := <-node.Consensus.VerifiedNewBlock
		blockHash, err := rlp.EncodeToBytes(block)
		if err != nil {
			utils.GetLogInstance().Warn("[SYNC] unable to encode block to hashes")
			continue
		}

		// really need to have a unique id independent of ip/port
		selfPeerID := utils.GetUniqueIDFromIPPort(node.SelfPeer.IP, node.SelfPeer.Port)
		utils.GetLogInstance().Debug("[SYNC] peerRegistration Record", "peerID", selfPeerID, "number", len(node.peerRegistrationRecord))

		for peerID, config := range node.peerRegistrationRecord {
			elapseTime := time.Now().UnixNano() - config.timestamp
			if elapseTime > broadcastTimeout {
				utils.GetLogInstance().Warn("[SYNC] SendNewBlockToUnsync to peer timeout", "peerID", peerID)
				// send last time and delete
				config.client.PushNewBlock(selfPeerID, blockHash, true)
				node.stateMutex.Lock()
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
				node.stateMutex.Unlock()
				continue
			}
			response := config.client.PushNewBlock(selfPeerID, blockHash, false)
			if response != nil && response.Type == downloader_pb.DownloaderResponse_INSYNC {
				node.stateMutex.Lock()
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
				node.stateMutex.Unlock()
			}
		}
	}
}
