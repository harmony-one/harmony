package node

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
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
	MinConnectedPeers = 10 // minimum number of peers connected to in node syncing
)

// getNeighborPeers is a helper function to return list of peers
// based on different neightbor map
func getNeighborPeers(neighbor *sync.Map) []p2p.Peer {
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
	go node.DoSyncing(node.Blockchain(), node.Worker, false) //Don't join consensus
}

// IsSameHeight tells whether node is at same bc height as a peer
func (node *Node) IsSameHeight() (uint64, bool) {
	if node.stateSync == nil {
		node.stateSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port, node.GetSyncID())
	}
	return node.stateSync.IsSameBlockchainHeight(node.Blockchain())
}

// SyncingPeerProvider is an interface for getting the peers in the given shard.
type SyncingPeerProvider interface {
	SyncingPeers(shardID uint32) (peers []p2p.Peer, err error)
}

// LegacySyncingPeerProvider uses neighbor lists stored in a Node to serve
// syncing peer list query.
type LegacySyncingPeerProvider struct {
	node    *Node
	shardID func() uint32
}

// NewLegacySyncingPeerProvider creates and returns a new node-based syncing
// peer provider.
func NewLegacySyncingPeerProvider(node *Node) *LegacySyncingPeerProvider {
	var shardID func() uint32
	if node.shardChains != nil {
		shardID = node.Blockchain().ShardID
	}
	return &LegacySyncingPeerProvider{node: node, shardID: shardID}
}

// SyncingPeers returns peers stored in neighbor maps in the node structure.
func (p *LegacySyncingPeerProvider) SyncingPeers(shardID uint32) (peers []p2p.Peer, err error) {
	switch shardID {
	case p.shardID():
		peers = getNeighborPeers(&p.node.Neighbors)
	case 0:
		peers = getNeighborPeers(&p.node.BeaconNeighbors)
	default:
		return nil, errors.Errorf("unsupported shard ID %v", shardID)
	}
	return peers, nil
}

// DNSSyncingPeerProvider uses the given DNS zone to resolve syncing peers.
type DNSSyncingPeerProvider struct {
	zone, port string
	lookupHost func(name string) (addrs []string, err error)
}

// NewDNSSyncingPeerProvider returns a provider that uses given DNS name and
// port number to resolve syncing peers.
func NewDNSSyncingPeerProvider(zone, port string) *DNSSyncingPeerProvider {
	return &DNSSyncingPeerProvider{
		zone:       zone,
		port:       port,
		lookupHost: net.LookupHost,
	}
}

// SyncingPeers resolves DNS name into peers and returns them.
func (p *DNSSyncingPeerProvider) SyncingPeers(shardID uint32) (peers []p2p.Peer, err error) {
	dns := fmt.Sprintf("s%d.%s", shardID, p.zone)
	addrs, err := p.lookupHost(dns)
	if err != nil {
		return nil, errors.Wrapf(err,
			"[SYNC] cannot find peers using DNS name %#v", dns)
	}
	for _, addr := range addrs {
		peers = append(peers, p2p.Peer{IP: addr, Port: p.port})
	}
	return peers, nil
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
				peers, err := node.SyncingPeerProvider.SyncingPeers(0)
				if err != nil {
					utils.Logger().Warn().
						Err(err).
						Msg("cannot retrieve beacon syncing peers")
					continue
				}
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
func (node *Node) DoSyncing(bc *core.BlockChain, worker *worker.Worker, willJoinConsensus bool) {
	ticker := time.NewTicker(SyncFrequency * time.Second)

SyncingLoop:
	for {
		select {
		case <-ticker.C:
			if node.stateSync == nil {
				node.stateSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port, node.GetSyncID())

				utils.Logger().Debug().Msg("[SYNC] initialized state sync")
			}
			if node.stateSync.GetActivePeerNumber() < MinConnectedPeers {
				shardID := bc.ShardID()
				peers, err := node.SyncingPeerProvider.SyncingPeers(shardID)
				if err != nil {
					utils.Logger().Warn().
						Err(err).
						Uint32("shard_id", shardID).
						Msg("cannot retrieve syncing peers")
					continue SyncingLoop
				}
				if err := node.stateSync.CreateSyncConfig(peers, false); err != nil {
					utils.Logger().Warn().
						Err(err).
						Interface("peers", peers).
						Msg("[SYNC] create peers error")
					continue SyncingLoop
				}
				utils.Logger().Debug().Int("len", node.stateSync.GetActivePeerNumber()).Msg("[SYNC] Get Active Peers")
			}
			if node.stateSync.IsOutOfSync(bc) {
				node.stateMutex.Lock()
				node.State = NodeNotInSync
				node.stateMutex.Unlock()
				if willJoinConsensus && node.Consensus.Mode() != consensus.Listening {
					node.Consensus.BlocksNotSynchronized()
				}
				node.stateSync.SyncLoop(bc, worker, willJoinConsensus, false)
				if willJoinConsensus && node.Consensus.Mode() != consensus.Listening {
					node.stateMutex.Lock()
					node.State = NodeReadyForConsensus
					node.stateMutex.Unlock()
					node.Consensus.BlocksSynchronized()
				}
			}
			node.stateMutex.Lock()
			node.State = NodeReadyForConsensus
			node.stateMutex.Unlock()
			if willJoinConsensus && node.Consensus.Mode() != consensus.Listening {
				node.Consensus.WaitForSyncing()
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

	// Check if the current node is explorer node.
	isExplorerNode := node.NodeConfig.Role() == nodeconfig.ExplorerNode

	// Send new block to unsync node if the current node is not explorer node.
	if !isExplorerNode {
		go node.SendNewBlockToUnsync()
	}

	go node.DoSyncing(node.Blockchain(), node.Worker, !isExplorerNode)
}

// InitSyncingServer starts downloader server.
func (node *Node) InitSyncingServer() {
	if node.downloaderServer == nil {
		node.downloaderServer = downloader.NewServer(node)
	}
}

// StartSyncingServer starts syncing server.
func (node *Node) StartSyncingServer() {
	utils.Logger().Info().Msg("[SYNC] support_syncing: StartSyncingServer")
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
			utils.Logger().Warn().Msg("[SYNC] unable to encode block to hashes")
			continue
		}

		node.stateMutex.Lock()
		for peerID, config := range node.peerRegistrationRecord {
			elapseTime := time.Now().UnixNano() - config.timestamp
			if elapseTime > broadcastTimeout {
				utils.Logger().Warn().Str("peerID", peerID).Msg("[SYNC] SendNewBlockToUnsync to peer timeout")
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
				continue
			}
			response := config.client.PushNewBlock(node.GetSyncID(), blockHash, false)
			if response != nil && response.Type == downloader_pb.DownloaderResponse_INSYNC {
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
			}
		}
		node.stateMutex.Unlock()
	}
}

// CalculateResponse implements DownloadInterface on Node object.
func (node *Node) CalculateResponse(request *downloader_pb.DownloaderRequest, incomingPeer string) (*downloader_pb.DownloaderResponse, error) {
	response := &downloader_pb.DownloaderResponse{}
	switch request.Type {
	case downloader_pb.DownloaderRequest_HEADER:
		if request.BlockHash == nil {
			return response, fmt.Errorf("[SYNC] GetBlockHashes Request BlockHash is NIL")
		}
		if request.Size == 0 || request.Size > syncing.BatchSize {
			return response, fmt.Errorf("[SYNC] GetBlockHashes Request contains invalid Size %v", request.Size)
		}
		size := uint64(request.Size)
		var startHashHeader common.Hash
		copy(startHashHeader[:], request.BlockHash[:])
		startBlock := node.Blockchain().GetBlockByHash(startHashHeader)
		if startBlock == nil {
			return response, fmt.Errorf("[SYNC] GetBlockHashes Request cannot find startHash %s", startHashHeader.Hex())
		}
		startHeight := startBlock.NumberU64()
		endHeight := node.Blockchain().CurrentBlock().NumberU64()
		if startHeight >= endHeight {
			utils.Logger().
				Debug().
				Uint64("myHeight", endHeight).
				Uint64("requestHeight", startHeight).
				Str("incomingIP", request.Ip).
				Str("incomingPort", request.Port).
				Str("incomingPeer", incomingPeer).
				Msg("[SYNC] GetBlockHashes Request: I am not higher than requested node")
			return response, nil
		}

		for blockNum := startHeight; blockNum <= startHeight+size; blockNum++ {
			block := node.Blockchain().GetBlockByNumber(blockNum)
			if block == nil {
				break
			}
			blockHash := block.Hash()
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
			utils.Logger().Debug().
				Str("state", node.State.String()).
				Msg("[SYNC] new block received, but state is")
			response.Type = downloader_pb.DownloaderResponse_INSYNC
			return response, nil
		}
		var blockObj types.Block
		err := rlp.DecodeBytes(request.BlockHash, &blockObj)
		if err != nil {
			utils.Logger().Warn().Msg("[SYNC] unable to decode received new block")
			return response, err
		}
		node.stateSync.AddNewBlock(request.PeerHash, &blockObj)

	case downloader_pb.DownloaderRequest_REGISTER:
		peerID := string(request.PeerHash[:])
		ip := request.Ip
		port := request.Port
		node.stateMutex.Lock()
		defer node.stateMutex.Unlock()
		if _, ok := node.peerRegistrationRecord[peerID]; ok {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			utils.Logger().Warn().
				Interface("ip", ip).
				Interface("port", port).
				Msg("[SYNC] peerRegistration record already exists")
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
				utils.Logger().Warn().
					Str("ip", ip).
					Str("port", port).
					Msg("[SYNC] unable to setup client for peerID")
				return response, nil
			}
			config := &syncConfig{timestamp: time.Now().UnixNano(), client: client}
			node.peerRegistrationRecord[peerID] = config
			utils.Logger().Debug().
				Str("ip", ip).
				Str("port", port).
				Msg("[SYNC] register peerID success")
			response.Type = downloader_pb.DownloaderResponse_SUCCESS
		}

	case downloader_pb.DownloaderRequest_REGISTERTIMEOUT:
		if node.State == NodeNotInSync {
			count := node.stateSync.RegisterNodeInfo()
			utils.Logger().Debug().
				Int("number", count).
				Msg("[SYNC] extra node registered")
		}
	}
	return response, nil
}
