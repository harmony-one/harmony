package node

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	peer "github.com/libp2p/go-libp2p-peer"
)

// GetSyncingPort returns the syncing port.
func GetSyncingPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-syncing.SyncingPortDifference)
	}
	os.Exit(1)
	return ""
}

// (TODO) temporary, remove it later
func getPeerFromIPandPort(ip, port string) p2p.Peer {
	priKey, _, _ := utils.GenKeyP2P(ip, port)
	peerID, _ := peer.IDFromPrivateKey(priKey)
	return p2p.Peer{IP: ip, Port: port, PeerID: peerID}
}

// getNeighborPeers is a helper function to return list of peers
// based on different neightbor map
func (node *Node) getNeighborPeers(neighbor *sync.Map) []p2p.Peer {
	res := []p2p.Peer{}
	neighbor.Range(func(k, v interface{}) bool {
		res = append(res, v.(p2p.Peer))
		return true
	})
	removeID := -1
	for i := range res {
		if res[i].Port == node.SelfPeer.Port {
			removeID = i
		}
		res[i].Port = GetSyncingPort(res[i].Port)
	}
	if removeID != -1 {
		res = append(res[:removeID], res[removeID+1:]...)
	}
	return res
}

// GetBeaconSyncingPeers returns a list of peers for beaconchain syncing
func (node *Node) GetBeaconSyncingPeers() []p2p.Peer {

	return node.getNeighborPeers(&node.BeaconNeighbors)
}

// GetSyncingPeers returns list of peers for regular shard syncing.
func (node *Node) GetSyncingPeers() []p2p.Peer {
	return node.getNeighborPeers(&node.Neighbors)
}

// DoBeaconSyncing update received beaconchain blocks and downloads missing beacon chain blocks
func (node *Node) DoBeaconSyncing() {
	for {
		select {
		case beaconBlock := <-node.BeaconBlockChannel:
			if node.beaconSync == nil {
				node.beaconSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port)
				node.beaconSync.CreateSyncConfig(node.GetBeaconSyncingPeers())
				node.beaconSync.MakeConnectionToPeers()
			}
			startHash := node.beaconChain.CurrentBlock().Hash()
			node.beaconSync.AddLastMileBlock(beaconBlock)
			node.beaconSync.StartStateSync(startHash[:], node.beaconChain, node.BeaconWorker, true)
			utils.GetLogInstance().Debug("[SYNC] STARTING BEACON SYNC")
		}
	}
}

// IsOutOfSync checks whether the node is out of sync by comparing latest block with consensus block
func (node *Node) IsOutOfSync(consensusBlockInfo *consensus.BFTBlockInfo) bool {
	consensusBlock := consensusBlockInfo.Block
	consensusID := consensusBlockInfo.ConsensusID

	myHeight := node.blockchain.CurrentBlock().NumberU64()
	newHeight := consensusBlock.NumberU64()
	utils.GetLogInstance().Debug("[SYNC]", "myHeight", myHeight, "newHeight", newHeight)
	if newHeight-myHeight <= inSyncThreshold {
		node.stateSync.AddLastMileBlock(consensusBlock)
		node.Consensus.UpdateConsensusID(consensusID + 1)
		return false
	}
	// cache latest blocks for last mile catch up
	if newHeight-myHeight <= lastMileThreshold && node.stateSync != nil {
		node.stateSync.AddLastMileBlock(consensusBlock)
	}
	return true
}

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
				node.stateSync.StartStateSync(startHash[:], node.blockchain, node.Worker, false)
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
			node.stateSync.StartStateSync(startHash[:], node.blockchain, node.Worker, false)
		}
	}
}

// SupportSyncing keeps sleeping until it's doing consensus or it's a leader.
func (node *Node) SupportSyncing() {
	node.InitSyncingServer()
	node.StartSyncingServer()

	go node.DoSyncing()
	go node.DoBeaconSyncing()
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
