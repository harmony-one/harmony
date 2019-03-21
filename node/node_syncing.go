package node

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Constants related to doing syncing.
const (
	lastMileThreshold = 4
	inSyncThreshold   = 1
)

// GetSyncingPort returns the syncing port.
func GetSyncingPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-syncing.SyncingPortDifference)
	}
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
	tmp := []p2p.Peer{}
	neighbor.Range(func(k, v interface{}) bool {
		tmp = append(tmp, v.(p2p.Peer))
		return true
	})
	for _, peer := range tmp {
		port := GetSyncingPort(peer.Port)
		if peer.Port != node.SelfPeer.Port && port != "" {
			peer.Port = port
			res = append(res, peer)
		}
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
				node.beaconSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port, node.Consensus.PubKey.GetAddress())
				node.beaconSync.CreateSyncConfig(node.GetBeaconSyncingPeers())
				node.beaconSync.MakeConnectionToPeers()
			}
			startHash := node.beaconChain.CurrentBlock().Hash()
			node.beaconSync.AddLastMileBlock(beaconBlock)
			node.beaconSync.ProcessStateSync(startHash[:], node.beaconChain, node.BeaconWorker)
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

// DoSync syncs with peers until catchup, this function is not coupled with consensus
func (node *Node) DoSync() {
	<-node.peerReadyChan
	ss := syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port, node.Consensus.PubKey.GetAddress())
	if ss.CreateSyncConfig(node.GetSyncingPeers()) {
		ss.MakeConnectionToPeers()
		ss.SyncLoop(node.blockchain, node.Worker)
	}
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
				node.stateSync.ProcessStateSync(startHash[:], node.blockchain, node.Worker)
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
				node.stateSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port, node.Consensus.PubKey.GetAddress())
				node.stateSync.CreateSyncConfig(node.GetSyncingPeers())
				node.stateSync.MakeConnectionToPeers()
			}
			startHash := node.blockchain.CurrentBlock().Hash()
			node.stateSync.ProcessStateSync(startHash[:], node.blockchain, node.Worker)
		}
	}
}

// SupportSyncing keeps sleeping until it's doing consensus or it's a leader.
func (node *Node) SupportSyncing() {
	node.InitSyncingServer()
	node.StartSyncingServer()

	//go node.DoSyncing()
	go node.DoSync()
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
		selfPeerAddress := node.Consensus.SelfAddress
		utils.GetLogInstance().Debug("[SYNC] peerRegistration Record", "selfPeerAddress", selfPeerAddress, "number", len(node.peerRegistrationRecord))

		for peerID, config := range node.peerRegistrationRecord {
			elapseTime := time.Now().UnixNano() - config.timestamp
			if elapseTime > broadcastTimeout {
				utils.GetLogInstance().Warn("[SYNC] SendNewBlockToUnsync to peer timeout", "peerID", peerID)
				// send last time and delete
				config.client.PushNewBlock(node.Consensus.PubKey.GetAddress(), blockHash, true)
				node.stateMutex.Lock()
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
				node.stateMutex.Unlock()
				continue
			}
			response := config.client.PushNewBlock(node.Consensus.PubKey.GetAddress(), blockHash, false)
			if response != nil && response.Type == downloader_pb.DownloaderResponse_INSYNC {
				node.stateMutex.Lock()
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
				node.stateMutex.Unlock()
			}
		}
	}
}

// CalculateResponse implements DownloadInterface on Node object.
func (node *Node) CalculateResponse(request *downloader_pb.DownloaderRequest) (*downloader_pb.DownloaderResponse, error) {
	response := &downloader_pb.DownloaderResponse{}
	switch request.Type {
	case downloader_pb.DownloaderRequest_HEADER:
		utils.GetLogInstance().Debug("[SYNC] CalculateResponse DownloaderRequest_HEADER", "request.BlockHash", request.BlockHash)
		var startHeaderHash []byte
		if request.BlockHash == nil {
			tmp := node.blockchain.Genesis().Hash()
			startHeaderHash = tmp[:]
		} else {
			startHeaderHash = request.BlockHash
		}
		for block := node.blockchain.CurrentBlock(); block != nil; block = node.blockchain.GetBlockByHash(block.Header().ParentHash) {
			blockHash := block.Hash()
			if bytes.Compare(blockHash[:], startHeaderHash) == 0 {
				break
			}
			response.Payload = append(response.Payload, blockHash[:])
		}

	case downloader_pb.DownloaderRequest_BLOCK:
		for _, bytes := range request.Hashes {
			var hash common.Hash
			hash.SetBytes(bytes)
			block := node.blockchain.GetBlockByHash(hash)
			encodedBlock, err := rlp.EncodeToBytes(block)
			if err == nil {
				response.Payload = append(response.Payload, encodedBlock)
			}
		}

	case downloader_pb.DownloaderRequest_BLOCKHEIGHT:
		response.BlockHeight = node.blockchain.CurrentBlock().NumberU64()

	// this is the out of sync node acts as grpc server side
	case downloader_pb.DownloaderRequest_NEWBLOCK:
		if node.State != NodeNotInSync {
			utils.GetLogInstance().Debug("[SYNC] new block received, but state is", "state", node.State.String())
			response.Type = downloader_pb.DownloaderResponse_INSYNC
			return response, nil
		}
		var blockObj types.Block
		err := rlp.DecodeBytes(request.BlockHash, &blockObj)
		if err != nil {
			utils.GetLogInstance().Warn("[SYNC] unable to decode received new block")
			return response, err
		}
		node.stateSync.AddNewBlock(request.PeerHash, &blockObj)

	case downloader_pb.DownloaderRequest_REGISTER:
		peerAddress := common.BytesToAddress(request.PeerHash[:]).Hex()
		if _, ok := node.peerRegistrationRecord[peerAddress]; ok {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			utils.GetLogInstance().Warn("[SYNC] peerRegistration record already exists", "peerAddress", peerAddress)
			return response, nil
		} else if len(node.peerRegistrationRecord) >= maxBroadcastNodes {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			utils.GetLogInstance().Warn("[SYNC] maximum registration limit exceeds", "peerAddress", peerAddress)
			return response, nil
		} else {
			peer := node.Consensus.GetPeerByAddress(peerAddress)
			response.Type = downloader_pb.DownloaderResponse_FAIL
			if peer == nil {
				utils.GetLogInstance().Warn("[SYNC] unable to get peer from peerID", "peerAddress", peerAddress)
				return response, nil
			}
			client := downloader.ClientSetup(peer.IP, GetSyncingPort(peer.Port))
			if client == nil {
				utils.GetLogInstance().Warn("[SYNC] unable to setup client for peerID", "peerAddress", peerAddress)
				return response, nil
			}
			config := &syncConfig{timestamp: time.Now().UnixNano(), client: client}
			node.stateMutex.Lock()
			node.peerRegistrationRecord[peerAddress] = config
			node.stateMutex.Unlock()
			utils.GetLogInstance().Debug("[SYNC] register peerID success", "peerAddress", peerAddress)
			response.Type = downloader_pb.DownloaderResponse_SUCCESS
		}

	case downloader_pb.DownloaderRequest_REGISTERTIMEOUT:
		if node.State == NodeNotInSync {
			count := node.stateSync.RegisterNodeInfo()
			utils.GetLogInstance().Debug("[SYNC] extra node registered", "number", count)
		}
	}
	return response, nil
}
