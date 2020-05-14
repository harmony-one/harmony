package node

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
	"github.com/pkg/errors"
)

// Constants related to doing syncing.
const (
	lastMileThreshold = 4
	inSyncThreshold   = 1 // unit in number of block
	SyncFrequency     = 60
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

// LocalSyncingPeerProvider uses localnet deployment convention to synthesize
// syncing peers.
type LocalSyncingPeerProvider struct {
	basePort, selfPort   uint16
	numShards, shardSize uint32
}

// NewLocalSyncingPeerProvider returns a provider that synthesizes syncing
// peers given the network configuration
func NewLocalSyncingPeerProvider(
	basePort, selfPort uint16, numShards, shardSize uint32,
) *LocalSyncingPeerProvider {
	return &LocalSyncingPeerProvider{
		basePort:  basePort,
		selfPort:  selfPort,
		numShards: numShards,
		shardSize: shardSize,
	}
}

// SyncingPeers returns local syncing peers using the sharding configuration.
func (p *LocalSyncingPeerProvider) SyncingPeers(shardID uint32) (peers []p2p.Peer, err error) {
	if shardID >= p.numShards {
		return nil, errors.Errorf(
			"shard ID %d out of range 0..%d", shardID, p.numShards-1)
	}
	firstPort := uint32(p.basePort) + shardID
	endPort := uint32(p.basePort) + p.numShards*p.shardSize
	for port := firstPort; port < endPort; port += p.numShards {
		if port == uint32(p.selfPort) {
			continue // do not sync from self
		}
		peers = append(peers, p2p.Peer{IP: "127.0.0.1", Port: fmt.Sprint(port)})
	}
	return peers, nil
}

// DoBeaconSyncing update received beaconchain blocks and downloads missing beacon chain blocks
func (node *Node) DoBeaconSyncing() {
	go func(node *Node) {
		// TODO ek – infinite loop; add shutdown/cleanup logic
		for beaconBlock := range node.BeaconBlockChannel {
			if node.beaconSync != nil {
				err := node.beaconSync.UpdateBlockAndStatus(
					beaconBlock, node.Beaconchain(), node.BeaconWorker, true,
				)
				if err != nil {
					node.beaconSync.AddLastMileBlock(beaconBlock)
				}
			}
		}
	}(node)

	// TODO ek – infinite loop; add shutdown/cleanup logic
	for {
		if node.beaconSync == nil {
			utils.Logger().Info().Msg("initializing beacon sync")
			node.beaconSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port, node.GetSyncID())
		}
		if node.beaconSync.GetActivePeerNumber() == 0 {
			utils.Logger().Info().Msg("no peers; bootstrapping beacon sync config")
			// 0 means shardID=0 here
			peers, err := node.SyncingPeerProvider.SyncingPeers(0)
			if err != nil {
				utils.Logger().Warn().
					Err(err).
					Msg("cannot retrieve beacon syncing peers")
				continue
			}
			if err := node.beaconSync.CreateSyncConfig(peers, true); err != nil {
				utils.Logger().Warn().Err(err).Msg("cannot create beacon sync config")
				continue
			}
		}
		node.beaconSync.SyncLoop(node.Beaconchain(), node.BeaconWorker, true, nil)
		time.Sleep(time.Duration(SyncFrequency) * time.Second)
	}
}

// DoSyncing keep the node in sync with other peers,
// willJoinConsensus means the node will try to join consensus after catch up
func (node *Node) DoSyncing(bc *core.BlockChain, worker *worker.Worker, willJoinConsensus bool) {
	ticker := time.NewTicker(time.Duration(SyncFrequency) * time.Second)
	// TODO ek – infinite loop; add shutdown/cleanup logic
	for {
		select {
		case <-ticker.C:
			node.doSync(bc, worker, willJoinConsensus)
		case <-node.Consensus.BlockNumLowChan:
			node.doSync(bc, worker, willJoinConsensus)
		}
	}
}

// doSync keep the node in sync with other peers, willJoinConsensus means the node will try to join consensus after catch up
func (node *Node) doSync(bc *core.BlockChain, worker *worker.Worker, willJoinConsensus bool) {
	if node.stateSync == nil {
		node.stateSync = syncing.CreateStateSync(
			node.SelfPeer.IP, node.SelfPeer.Port, node.GetSyncID(),
		)
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
			return
		}
		if err := node.stateSync.CreateSyncConfig(peers, false); err != nil {
			utils.Logger().Warn().
				Err(err).
				Interface("peers", peers).
				Msg("[SYNC] create peers error")
			return
		}
		utils.Logger().Debug().
			Int("len", node.stateSync.GetActivePeerNumber()).
			Msg("[SYNC] Get Active Peers")
	}
	// TODO: treat fake maximum height
	if node.stateSync.IsOutOfSync(bc) {
		node.stateMutex.Lock()
		node.State = NodeNotInSync
		node.stateMutex.Unlock()
		if willJoinConsensus {
			node.Consensus.BlocksNotSynchronized()
		}
		node.stateSync.SyncLoop(bc, worker, false, node.Consensus)
		if willJoinConsensus {
			node.stateMutex.Lock()
			node.State = NodeReadyForConsensus
			node.stateMutex.Unlock()
			node.Consensus.BlocksSynchronized()
		}
	}
	node.stateMutex.Lock()
	node.State = NodeReadyForConsensus
	node.stateMutex.Unlock()
}

// SupportBeaconSyncing sync with beacon chain
// for archival node in beacon chan or non-beacon node
func (node *Node) SupportBeaconSyncing() {
	go node.DoBeaconSyncing()
}

// SupportSyncing keeps sleeping until it's doing consensus or it's a leader.
func (node *Node) SupportSyncing() {
	if node.downloaderServer == nil {
		node.downloaderServer = downloader.NewServer(node)
	}
	utils.Logger().Info().Msg("[SYNC] support_syncing: StartSyncingServer")
	if node.downloaderServer.GrpcServer == nil {
		node.downloaderServer.Start(
			node.SelfPeer.IP, syncing.GetSyncingPort(node.SelfPeer.Port),
		)
	}

	go node.DoSyncing(
		node.Blockchain(),
		node.Worker,
		node.NodeConfig.Role() == nodeconfig.Validator,
	)
}

// CalculateResponse implements DownloadInterface on Node object.
func (node *Node) CalculateResponse(
	request *downloader_pb.DownloaderRequest,
	incomingPeer string,
) (*downloader_pb.DownloaderResponse, error) {
	response := &downloader_pb.DownloaderResponse{}
	switch request.Type {
	case downloader_pb.DownloaderRequest_BLOCKHASH:
		if request.BlockHash == nil {
			return response, fmt.Errorf("[SYNC] GetBlockHashes Request BlockHash is NIL")
		}
		if request.Size == 0 || request.Size > syncing.SyncLoopBatchSize {
			return response, errors.Errorf(
				"[SYNC] GetBlockHashes Request contains invalid Size %v", request.Size,
			)
		}
		size := uint64(request.Size)
		var startHashHeader common.Hash
		copy(startHashHeader[:], request.BlockHash[:])
		startHeader := node.Blockchain().GetHeaderByHash(startHashHeader)
		if startHeader == nil {
			return response, errors.Errorf(
				"sync issue, GetBlockHashes Request cannot find startHash %s", startHashHeader.Hex(),
			)
		}
		startHeight := startHeader.Number().Uint64()
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
			header := node.Blockchain().GetHeaderByNumber(blockNum)
			if header == nil {
				break
			}
			blockHash := header.Hash()
			response.Payload = append(response.Payload, blockHash[:])
		}

	case downloader_pb.DownloaderRequest_BLOCKHEADER:
		var hash common.Hash
		for _, bytes := range request.Hashes {
			hash.SetBytes(bytes)
			blockHeader := node.Blockchain().GetHeaderByHash(hash)
			if blockHeader == nil {
				continue
			}
			encodedBlockHeader, err := rlp.EncodeToBytes(blockHeader)

			if err == nil {
				response.Payload = append(response.Payload, encodedBlockHeader)
			}
		}

	case downloader_pb.DownloaderRequest_BLOCK:
		var hash common.Hash
		for _, bytes := range request.Hashes {
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
		node.stateMutex.Lock()
		defer node.stateMutex.Unlock()
		if node.State != NodeNotInSync {
			utils.Logger().Debug().
				Msgf("[SYNC] new block received, but state is %v", node.State)
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
			utils.Logger().Debug().
				Str("ip", ip).
				Str("port", port).
				Msg("[SYNC] maximum registration limit exceeds")
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
