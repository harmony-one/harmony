package node

import (
	"bytes"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
)

// Constants related to doing syncing.
const (
	lastMileThreshold = 4
	inSyncThreshold   = 1  // unit in number of block
	SyncFrequency     = 10 // unit in second
)

// getNeighborPeers is a helper function to return list of peers
// based on different neightbor map
func (node *Node) getNeighborPeers(neighbor *sync.Map) []p2p.Peer {
	tmp := []p2p.Peer{}
	neighbor.Range(func(k, v interface{}) bool {
		p := v.(p2p.Peer)
		t := p.Port
		p.Port = syncing.GetSyncingPort(t)
		tmp = append(tmp, p)
		return true
	})
	return tmp
}

// DoSyncWithoutConsensus gets sync-ed to blockchain without joining consensus
func (node *Node) DoSyncWithoutConsensus() {
	go node.DoSyncing(node.Blockchain(), node.Worker, node.GetSyncingPeers, false) //Don't join consensus
}

// IsSameHeight tells whether node is at same bc height as a peer
func (node *Node) IsSameHeight() (uint64, bool) {
	if node.stateSync == nil {
		node.stateSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port, node.GetSyncID())
	}
	return node.stateSync.IsSameBlockchainHeight(node.Blockchain())
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
				node.beaconSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port, node.GetSyncID())
			}
			if node.beaconSync.GetActivePeerNumber() == 0 {
				peers := node.GetBeaconSyncingPeers()
				if err := node.beaconSync.CreateSyncConfig(peers, true); err != nil {
					ctxerror.Log15(utils.GetLogInstance().Debug, err)
					continue
				}
			}
			node.beaconSync.AddLastMileBlock(beaconBlock)
			node.beaconSync.SyncLoop(node.Beaconchain(), node.BeaconWorker, false, true)
		}
	}
}

// DoSyncing keep the node in sync with other peers, willJoinConsensus means the node will try to join consensus after catch up
func (node *Node) DoSyncing(bc *core.BlockChain, worker *worker.Worker, getPeers func() []p2p.Peer, willJoinConsensus bool) {
	ticker := time.NewTicker(SyncFrequency * time.Second)

	logger := utils.GetLogInstance()
	getLogger := func() log.Logger { return utils.WithCallerSkip(logger, 1) }
SyncingLoop:
	for {
		select {
		case <-ticker.C:
			if node.stateSync == nil {
				node.stateSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port, node.GetSyncID())
				logger = logger.New("syncID", node.GetSyncID())
				getLogger().Debug("initialized state sync")
			}
			if node.stateSync.GetActivePeerNumber() == 0 {
				peers := getPeers()
				if err := node.stateSync.CreateSyncConfig(peers, false); err != nil {
					ctxerror.Log15(utils.GetLogInstance().Debug, err)
					continue SyncingLoop
				}
			}
			if node.stateSync.IsOutOfSync(bc) {
				utils.GetLogInstance().Debug("[SYNC] out of sync, doing syncing", "willJoinConsensus", willJoinConsensus)
				node.stateMutex.Lock()
				node.State = NodeNotInSync
				node.stateMutex.Unlock()
				node.stateSync.SyncLoop(bc, worker, willJoinConsensus, false)
				getLogger().Debug("now in sync")
				if willJoinConsensus {
					getLogger().Debug("entering NodeReadyForConsensus state")
					node.stateMutex.Lock()
					node.State = NodeReadyForConsensus
					node.stateMutex.Unlock()
					node.Consensus.BlocksSynchronized()
				}
			}
			node.stateMutex.Lock()
			node.State = NodeReadyForConsensus
			node.stateMutex.Unlock()
			if willJoinConsensus {
				<-node.Consensus.ViewIDLowChan
			}
		}
	}
}

// SupportBeaconSyncing sync with beacon chain for archival node in beacon chan or non-beacon node
func (node *Node) SupportBeaconSyncing() {
	go node.DoBeaconSyncing()
}

// SupportSyncing keeps sleeping until it's doing consensus or it's a leader.
func (node *Node) SupportSyncing() {
	node.InitSyncingServer()
	node.StartSyncingServer()
	go node.SendNewBlockToUnsync()

	go node.DoSyncing(node.Blockchain(), node.Worker, node.GetSyncingPeers, true)
}

// InitSyncingServer starts downloader server.
func (node *Node) InitSyncingServer() {
	if node.downloaderServer == nil {
		node.downloaderServer = downloader.NewServer(node)
	}
}

// StartSyncingServer starts syncing server.
func (node *Node) StartSyncingServer() {
	utils.GetLogInstance().Info("support_syncing: StartSyncingServer")
	if node.downloaderServer.GrpcServer == nil {
		node.downloaderServer.Start(node.SelfPeer.IP, syncing.GetSyncingPort(node.SelfPeer.Port))
	}
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
				config.client.PushNewBlock(node.GetSyncID(), blockHash, true)
				node.stateMutex.Lock()
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
				node.stateMutex.Unlock()
				continue
			}
			response := config.client.PushNewBlock(node.GetSyncID(), blockHash, false)
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
		var startHeaderHash []byte
		if request.BlockHash == nil {
			tmp := node.Blockchain().Genesis().Hash()
			startHeaderHash = tmp[:]
		} else {
			startHeaderHash = request.BlockHash
		}
		for block := node.Blockchain().CurrentBlock(); block != nil; block = node.Blockchain().GetBlockByHash(block.Header().ParentHash) {
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
			block := node.Blockchain().GetBlockByHash(hash)
			if block == nil {
				continue
			}
			encodedBlock, err := rlp.EncodeToBytes(block)
			if err == nil {
				response.Payload = append(response.Payload, encodedBlock)
			}
		}

	case downloader_pb.DownloaderRequest_BLOCKHEIGHT:
		response.BlockHeight = node.Blockchain().CurrentBlock().NumberU64()

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
		peerID := string(request.PeerHash[:])
		ip := request.Ip
		port := request.Port
		if _, ok := node.peerRegistrationRecord[peerID]; ok {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			utils.GetLogInstance().Warn("[SYNC] peerRegistration record already exists", "ip", ip, "port", port)
			return response, nil
		} else if len(node.peerRegistrationRecord) >= maxBroadcastNodes {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			utils.GetLogInstance().Warn("[SYNC] maximum registration limit exceeds", "ip", ip, "port", port)
			return response, nil
		} else {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			syncPort := syncing.GetSyncingPort(port)
			client := downloader.ClientSetup(ip, syncPort)
			if client == nil {
				utils.GetLogInstance().Warn("[SYNC] unable to setup client for peerID", "ip", ip, "port", port)
				return response, nil
			}
			config := &syncConfig{timestamp: time.Now().UnixNano(), client: client}
			node.stateMutex.Lock()
			node.peerRegistrationRecord[peerID] = config
			node.stateMutex.Unlock()
			utils.GetLogInstance().Debug("[SYNC] register peerID success", "ip", ip, "port", port)
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
