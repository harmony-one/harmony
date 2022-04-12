package node

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/legacysync"
	legdownloader "github.com/harmony-one/harmony/api/service/legacysync/downloader"
	downloader_pb "github.com/harmony-one/harmony/api/service/legacysync/downloader/proto"
	"github.com/harmony-one/harmony/api/service/synchronize"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy/downloader"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
)

// Constants related to doing syncing.
const (
	SyncFrequency = 60

	// getBlocksRequestHardCap is the hard capped message size at server side for getBlocks request.
	// The number is 4MB (default gRPC message size) minus 2k reserved for message overhead.
	getBlocksRequestHardCap = 4*1024*1024 - 2*1024

	// largeNumberDiff is the number of big block diff to set un-sync
	// TODO: refactor this.
	largeNumberDiff = 1000
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// BeaconSyncHook is the hook function called after inserted beacon in downloader
// TODO: This is a small misc piece of consensus logic. Better put it to consensus module.
func (node *Node) BeaconSyncHook() {
	if node.Consensus.IsLeader() || rand.Intn(100) == 0 {
		// TODO: Instead of leader, it would better be validator do this broadcast since leader do
		//       not have much idle resources.
		node.BroadcastCrossLink()
	}
}

// GenerateRandomString generates a random string with given length
func GenerateRandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// getNeighborPeers is a helper function to return list of peers
// based on different neightbor map
func getNeighborPeers(neighbor *sync.Map) []p2p.Peer {
	tmp := []p2p.Peer{}
	neighbor.Range(func(k, v interface{}) bool {
		p := v.(p2p.Peer)
		t := p.Port
		p.Port = legacysync.GetSyncingPort(t)
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
		node.stateSync = node.createStateSync(node.Blockchain())
	}
	return node.stateSync.IsSameBlockchainHeight(node.Blockchain())
}

func (node *Node) createStateSync(bc *core.BlockChain) *legacysync.StateSync {
	// Temp hack: The actual port used in dns sync is node.downloaderServer.Port.
	// But registration is done through an old way of port arithmetics (syncPort + 3000).
	// Thus for compatibility, we are doing the arithmetics here, and not to change the
	// protocol itself. This is just the temporary hack and will not be a concern after
	// state sync.
	var mySyncPort int
	if node.downloaderServer != nil {
		mySyncPort = node.downloaderServer.Port
	} else {
		// If local sync server is not started, the port field in protocol is actually not
		// functional, simply set it to default value.
		mySyncPort = nodeconfig.DefaultDNSPort
	}
	mutatedPort := strconv.Itoa(mySyncPort + legacysync.SyncingPortDifference)
	role := node.NodeConfig.Role()
	return legacysync.CreateStateSync(bc, node.SelfPeer.IP, mutatedPort,
		node.GetSyncID(), node.NodeConfig.Role() == nodeconfig.ExplorerNode, role)
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

// doBeaconSyncing update received beaconchain blocks and downloads missing beacon chain blocks
func (node *Node) doBeaconSyncing() {
	if node.NodeConfig.IsOffline {
		return
	}

	if !node.NodeConfig.Downloader {
		// If Downloader is not working, we need also deal with blocks from beaconBlockChannel
		go func(node *Node) {
			// TODO ek – infinite loop; add shutdown/cleanup logic
			for beaconBlock := range node.BeaconBlockChannel {
				if node.beaconSync != nil {
					if beaconBlock.NumberU64() >= node.Beaconchain().CurrentBlock().NumberU64()+1 {
						err := node.beaconSync.UpdateBlockAndStatus(
							beaconBlock, node.Beaconchain(), true,
						)
						if err != nil {
							node.beaconSync.AddLastMileBlock(beaconBlock)
						} else if node.Consensus.IsLeader() || rand.Intn(100) == 0 {
							// Only leader or 1% of validators broadcast crosslink to avoid spamming p2p
							if beaconBlock.NumberU64() == node.Beaconchain().CurrentBlock().NumberU64() {
								node.BroadcastCrossLink()
							}
						}
					}
				}
			}
		}(node)
	}

	// TODO ek – infinite loop; add shutdown/cleanup logic
	for {
		if node.beaconSync == nil {
			utils.Logger().Info().Msg("initializing beacon sync")
			node.beaconSync = node.createStateSync(node.Beaconchain())
		}
		if node.beaconSync.GetActivePeerNumber() == 0 {
			utils.Logger().Info().Msg("no peers; bootstrapping beacon sync config")
			peers, err := node.SyncingPeerProvider.SyncingPeers(shard.BeaconChainShardID)
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

// DoSyncing keep the node in sync with other peers, willJoinConsensus means the node will try to join consensus after catch up
func (node *Node) DoSyncing(bc *core.BlockChain, worker *worker.Worker, willJoinConsensus bool) {
	if node.NodeConfig.IsOffline {
		return
	}

	ticker := time.NewTicker(time.Duration(SyncFrequency) * time.Second)
	defer ticker.Stop()
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
	if node.stateSync.GetActivePeerNumber() < legacysync.NumPeersLowBound {
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
		utils.Logger().Debug().Int("len", node.stateSync.GetActivePeerNumber()).Msg("[SYNC] Get Active Peers")
	}
	// TODO: treat fake maximum height
	if result := node.stateSync.GetSyncStatusDoubleChecked(); !result.IsInSync {
		node.IsInSync.UnSet()
		if willJoinConsensus {
			node.Consensus.BlocksNotSynchronized()
		}
		node.stateSync.SyncLoop(bc, worker, false, node.Consensus)
		if willJoinConsensus {
			node.IsInSync.Set()
			node.Consensus.BlocksSynchronized()
		}
	}
	node.IsInSync.Set()
}

// SupportGRPCSyncServer do gRPC sync server
func (node *Node) SupportGRPCSyncServer(port int) {
	node.InitSyncingServer(port)
	node.StartSyncingServer(port)
}

// StartGRPCSyncClient start the legacy gRPC sync process
func (node *Node) StartGRPCSyncClient() {
	if node.Blockchain().ShardID() != shard.BeaconChainShardID {
		utils.Logger().Info().
			Uint32("shardID", node.Blockchain().ShardID()).
			Msg("SupportBeaconSyncing")
		go node.doBeaconSyncing()
	}
	node.supportSyncing()
}

// supportSyncing keeps sleeping until it's doing consensus or it's a leader.
func (node *Node) supportSyncing() {
	joinConsensus := false
	// Check if the current node is explorer node.
	switch node.NodeConfig.Role() {
	case nodeconfig.Validator:
		joinConsensus = true
	}

	// Send new block to unsync node if the current node is not explorer node.
	// TODO: leo this pushing logic has to be removed
	if joinConsensus {
		go node.SendNewBlockToUnsync()
	}

	if node.stateSync == nil {
		node.stateSync = node.createStateSync(node.Blockchain())
		utils.Logger().Debug().Msg("[SYNC] initialized state sync")
	}

	go node.DoSyncing(node.Blockchain(), node.Worker, joinConsensus)
}

// InitSyncingServer starts downloader server.
func (node *Node) InitSyncingServer(port int) {
	if node.downloaderServer == nil {
		node.downloaderServer = legdownloader.NewServer(node, port)
	}
}

// StartSyncingServer starts syncing server.
func (node *Node) StartSyncingServer(port int) {
	utils.Logger().Info().Msg("[SYNC] support_syncing: StartSyncingServer")
	if node.downloaderServer.GrpcServer == nil {
		node.downloaderServer.Start()
	}
}

// SendNewBlockToUnsync send latest verified block to unsync, registered nodes
func (node *Node) SendNewBlockToUnsync() {
	for {
		block := <-node.Consensus.VerifiedNewBlock
		blockBytes, err := rlp.EncodeToBytes(block)
		if err != nil {
			utils.Logger().Warn().Msg("[SYNC] unable to encode block to hashes")
			continue
		}
		blockWithSigBytes, err := node.getEncodedBlockWithSigFromBlock(block)
		if err != nil {
			utils.Logger().Warn().Err(err).Msg("[SYNC] rlp encode BlockWithSig")
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
			sendBytes := blockBytes
			if config.withSig {
				sendBytes = blockWithSigBytes
			}
			response, err := config.client.PushNewBlock(node.GetSyncID(), sendBytes, false)
			// close the connection if cannot push new block to unsync node
			if err != nil {
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
			}
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
	if node.NodeConfig.IsOffline {
		return response, nil
	}

	switch request.Type {
	case downloader_pb.DownloaderRequest_BLOCKHASH:
		dnsServerRequestCounterVec.With(dnsReqMetricLabel("block_hash")).Inc()
		if request.BlockHash == nil {
			return response, fmt.Errorf("[SYNC] GetBlockHashes Request BlockHash is NIL")
		}
		if request.Size == 0 || request.Size > legacysync.SyncLoopBatchSize {
			return response, fmt.Errorf("[SYNC] GetBlockHashes Request contains invalid Size %v", request.Size)
		}
		size := uint64(request.Size)
		var startHashHeader common.Hash
		copy(startHashHeader[:], request.BlockHash[:])
		startHeader := node.Blockchain().GetHeaderByHash(startHashHeader)
		if startHeader == nil {
			return response, fmt.Errorf("[SYNC] GetBlockHashes Request cannot find startHash %s", startHashHeader.Hex())
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
		dnsServerRequestCounterVec.With(dnsReqMetricLabel("block_header")).Inc()
		var hash common.Hash
		for _, bytes := range request.Hashes {
			hash.SetBytes(bytes)
			encodedBlockHeader, err := node.getEncodedBlockHeaderByHash(hash)

			if err == nil {
				response.Payload = append(response.Payload, encodedBlockHeader)
			}
		}

	case downloader_pb.DownloaderRequest_BLOCK:
		dnsServerRequestCounterVec.With(dnsReqMetricLabel("block")).Inc()
		var hash common.Hash

		payloadSize := 0
		withSig := request.GetBlocksWithSig
		for _, bytes := range request.Hashes {
			hash.SetBytes(bytes)
			var (
				encoded []byte
				err     error
			)
			if withSig {
				encoded, err = node.getEncodedBlockWithSigByHash(hash)
			} else {
				encoded, err = node.getEncodedBlockByHash(hash)
			}
			if err != nil {
				continue
			}
			payloadSize += len(encoded)
			if payloadSize > getBlocksRequestHardCap {
				utils.Logger().Warn().Err(err).
					Int("req size", len(request.Hashes)).
					Int("cur size", len(response.Payload)).
					Msg("[SYNC] Max blocks response size reached, ignoring the rest.")
				break
			}
			response.Payload = append(response.Payload, encoded)
		}

	case downloader_pb.DownloaderRequest_BLOCKHEIGHT:
		dnsServerRequestCounterVec.With(dnsReqMetricLabel("block_height")).Inc()
		response.BlockHeight = node.Blockchain().CurrentBlock().NumberU64()

	// this is the out of sync node acts as grpc server side
	case downloader_pb.DownloaderRequest_NEWBLOCK:
		dnsServerRequestCounterVec.With(dnsReqMetricLabel("new block")).Inc()
		if node.IsInSync.IsSet() {
			response.Type = downloader_pb.DownloaderResponse_INSYNC
			return response, nil
		}
		block, err := legacysync.RlpDecodeBlockOrBlockWithSig(request.BlockHash)
		if err != nil {
			utils.Logger().Warn().Err(err).Msg("[SYNC] unable to decode received new block")
			return response, err
		}
		node.stateSync.AddNewBlock(request.PeerHash, block)

	case downloader_pb.DownloaderRequest_REGISTER:
		peerID := string(request.PeerHash[:])
		ip := request.Ip
		port := request.Port
		withSig := request.RegisterWithSig
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
			syncPort := legacysync.GetSyncingPort(port)
			client := legdownloader.ClientSetup(ip, syncPort)
			if client == nil {
				utils.Logger().Warn().
					Str("ip", ip).
					Str("port", port).
					Msg("[SYNC] unable to setup client for peerID")
				return response, nil
			}
			config := &syncConfig{timestamp: time.Now().UnixNano(), client: client, withSig: withSig}
			node.peerRegistrationRecord[peerID] = config
			utils.Logger().Debug().
				Str("ip", ip).
				Str("port", port).
				Msg("[SYNC] register peerID success")
			response.Type = downloader_pb.DownloaderResponse_SUCCESS
		}

	case downloader_pb.DownloaderRequest_REGISTERTIMEOUT:
		if !node.IsInSync.IsSet() {
			count := node.stateSync.RegisterNodeInfo()
			utils.Logger().Debug().
				Int("number", count).
				Msg("[SYNC] extra node registered")
		}

	case downloader_pb.DownloaderRequest_BLOCKBYHEIGHT:
		if len(request.Heights) == 0 {
			return response, errors.New("empty heights list provided")
		}

		if len(request.Heights) > int(legacysync.SyncLoopBatchSize) {
			return response, errors.New("exceed size limit")
		}

		out := make([][]byte, 0, len(request.Heights))
		for _, v := range request.Heights {
			block := node.Blockchain().GetBlockByNumber(v)
			if block == nil {
				return response, errors.Errorf("no block with height %d found", v)
			}
			blockBytes, err := node.getEncodedBlockWithSigByHeight(v)
			if err != nil {
				return response, errors.Errorf("failed to get block")
			}
			out = append(out, blockBytes)
		}
		response.Payload = out
	}

	return response, nil
}

func init() {
	prom.PromRegistry().MustRegister(
		dnsServerRequestCounterVec,
	)
}

var (
	dnsServerRequestCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "dns_server",
			Name:      "request_count",
			Help:      "request count for each dns request",
		},
		[]string{"method"},
	)
)

func dnsReqMetricLabel(method string) prometheus.Labels {
	return prometheus.Labels{
		"method": method,
	}
}

const (
	headerCacheSize       = 10000
	blockCacheSize        = 10000
	blockWithSigCacheSize = 10000
)

var (
	// Cached fields for block header and block requests
	headerReqCache, _       = lru.New(headerCacheSize)
	blockReqCache, _        = lru.New(blockCacheSize)
	blockWithSigReqCache, _ = lru.New(blockWithSigCacheSize)

	errHeaderNotExist = errors.New("header not exist")
	errBlockNotExist  = errors.New("block not exist")
)

func (node *Node) getEncodedBlockHeaderByHash(hash common.Hash) ([]byte, error) {
	if b, ok := headerReqCache.Get(hash); ok {
		return b.([]byte), nil
	}
	h := node.Blockchain().GetHeaderByHash(hash)
	if h == nil {
		return nil, errHeaderNotExist
	}
	b, err := rlp.EncodeToBytes(h)
	if err != nil {
		return nil, err
	}
	headerReqCache.Add(hash, b)
	return b, nil
}

func (node *Node) getEncodedBlockByHash(hash common.Hash) ([]byte, error) {
	if b, ok := blockReqCache.Get(hash); ok {
		return b.([]byte), nil
	}
	blk := node.Blockchain().GetBlockByHash(hash)
	if blk == nil {
		return nil, errBlockNotExist
	}
	b, err := rlp.EncodeToBytes(blk)
	if err != nil {
		return nil, err
	}
	blockReqCache.Add(hash, b)
	return b, nil
}

func (node *Node) getEncodedBlockWithSigByHash(hash common.Hash) ([]byte, error) {
	if b, ok := blockWithSigReqCache.Get(hash); ok {
		return b.([]byte), nil
	}
	blk := node.Blockchain().GetBlockByHash(hash)
	if blk == nil {
		return nil, errBlockNotExist
	}
	sab, err := node.getCommitSigAndBitmapFromChildOrDB(blk)
	if err != nil {
		return nil, err
	}
	bwh := legacysync.BlockWithSig{
		Block:              blk,
		CommitSigAndBitmap: sab,
	}
	b, err := rlp.EncodeToBytes(bwh)
	if err != nil {
		return nil, err
	}
	blockWithSigReqCache.Add(hash, b)
	return b, nil
}

func (node *Node) getEncodedBlockWithSigByHeight(height uint64) ([]byte, error) {
	blk := node.Blockchain().GetBlockByNumber(height)
	if blk == nil {
		return nil, errBlockNotExist
	}
	sab, err := node.getCommitSigAndBitmapFromChildOrDB(blk)
	if err != nil {
		return nil, err
	}
	bwh := legacysync.BlockWithSig{
		Block:              blk,
		CommitSigAndBitmap: sab,
	}
	b, err := rlp.EncodeToBytes(bwh)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (node *Node) getEncodedBlockWithSigFromBlock(block *types.Block) ([]byte, error) {
	bwh := legacysync.BlockWithSig{
		Block:              block,
		CommitSigAndBitmap: block.GetCurrentCommitSig(),
	}
	return rlp.EncodeToBytes(bwh)
}

func (node *Node) getCommitSigAndBitmapFromChildOrDB(block *types.Block) ([]byte, error) {
	child := node.Blockchain().GetBlockByNumber(block.NumberU64() + 1)
	if child != nil {
		return node.getCommitSigFromChild(block, child)
	}
	return node.getCommitSigFromDB(block)
}

func (node *Node) getCommitSigFromChild(parent, child *types.Block) ([]byte, error) {
	if child.ParentHash() != parent.Hash() {
		return nil, fmt.Errorf("child's parent hash unexpected: %v / %v",
			child.ParentHash().String(), parent.Hash().String())
	}
	sig := child.Header().LastCommitSignature()
	bitmap := child.Header().LastCommitBitmap()
	return append(sig[:], bitmap...), nil
}

func (node *Node) getCommitSigFromDB(block *types.Block) ([]byte, error) {
	return node.Blockchain().ReadCommitSig(block.NumberU64())
}

// SyncStatus return the syncing status, including whether node is syncing
// and the target block number, and the difference between current block
// and target block.
func (node *Node) SyncStatus(shardID uint32) (bool, uint64, uint64) {
	if node.NodeConfig.Role() == nodeconfig.ExplorerNode {
		exp, err := node.getExplorerService()
		if err != nil {
			// unreachable code
			return false, 0, largeNumberDiff
		}
		if !exp.IsAvailable() {
			return false, 0, largeNumberDiff
		}
	}

	ds := node.getDownloaders()
	if ds == nil || !ds.IsActive() {
		// downloaders inactive. Ask DNS sync instead
		return node.legacySyncStatus(shardID)
	}
	return ds.SyncStatus(shardID)
}

func (node *Node) legacySyncStatus(shardID uint32) (bool, uint64, uint64) {
	switch shardID {
	case node.NodeConfig.ShardID:
		if node.stateSync == nil {
			return false, 0, 0
		}
		result := node.stateSync.GetSyncStatus()
		return result.IsInSync, result.OtherHeight, result.HeightDiff

	case shard.BeaconChainShardID:
		if node.beaconSync == nil {
			return false, 0, 0
		}
		result := node.beaconSync.GetSyncStatus()
		return result.IsInSync, result.OtherHeight, result.HeightDiff

	default:
		// Shard node is not working on
		return false, 0, 0
	}
}

// IsOutOfSync return whether the node is out of sync of the given hsardID
func (node *Node) IsOutOfSync(shardID uint32) bool {
	ds := node.getDownloaders()
	if ds == nil || !ds.IsActive() {
		// downloaders inactive. Ask DNS sync instead
		return node.legacyIsOutOfSync(shardID)
	}
	isSyncing, _, _ := ds.SyncStatus(shardID)
	return !isSyncing
}

func (node *Node) legacyIsOutOfSync(shardID uint32) bool {
	switch shardID {
	case node.NodeConfig.ShardID:
		if node.stateSync == nil {
			return true
		}
		result := node.stateSync.GetSyncStatus()
		return !result.IsInSync

	case shard.BeaconChainShardID:
		if node.beaconSync == nil {
			return true
		}
		result := node.beaconSync.GetSyncStatus()
		return !result.IsInSync

	default:
		return true
	}
}

// SyncPeers return connected sync peers for each shard
func (node *Node) SyncPeers() map[string]int {
	ds := node.getDownloaders()
	if ds == nil {
		return nil
	}
	nums := ds.NumPeers()
	res := make(map[string]int)
	for sid, num := range nums {
		s := fmt.Sprintf("shard-%v", sid)
		res[s] = num
	}
	return res
}

func (node *Node) getDownloaders() *downloader.Downloaders {
	syncService := node.serviceManager.GetService(service.Synchronize)
	if syncService == nil {
		return nil
	}
	dsService, ok := syncService.(*synchronize.Service)
	if !ok {
		return nil
	}
	return dsService.Downloaders
}
