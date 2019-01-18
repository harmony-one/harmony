package node

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/client"
	clientService "github.com/harmony-one/harmony/api/client/service"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/api/services/explorer"
	"github.com/harmony-one/harmony/api/services/syncing"
	"github.com/harmony-one/harmony/api/services/syncing/downloader"
	downloader_pb "github.com/harmony-one/harmony/api/services/syncing/downloader/proto"
	bft "github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// State is a state of a node.
type State byte

// All constants except the NodeLeader below are for validators only.
const (
	NodeInit              State = iota // Node just started, before contacting BeaconChain
	NodeWaitToJoin                     // Node contacted BeaconChain, wait to join Shard
	NodeNotSync                        // Node out of sync, might be just joined Shard or offline for a period of time
	NodeOffline                        // Node is offline
	NodeReadyForConsensus              // Node is ready for doing consensus
	NodeDoingConsensus                 // Node is already doing consensus
	NodeLeader                         // Node is the leader of some shard.
)

func (state State) String() string {
	switch state {
	case NodeInit:
		return "NodeInit"
	case NodeWaitToJoin:
		return "NodeWaitToJoin"
	case NodeNotSync:
		return "NodeNotSync"
	case NodeOffline:
		return "NodeOffline"
	case NodeReadyForConsensus:
		return "NodeReadyForConsensus"
	case NodeDoingConsensus:
		return "NodeDoingConsensus"
	case NodeLeader:
		return "NodeLeader"
	}
	return "Unknown"
}

// Constants related to doing syncing.
const (
	NotDoingSyncing uint32 = iota
	DoingSyncing
)

const (
	waitBeforeJoinShard = time.Second * 3
	timeOutToJoinShard  = time.Minute * 10
	// ClientServicePortDiff is the positive port diff for client service
	ClientServicePortDiff       = 5555
	maxBroadcastNodes           = 10                  // broadcast at most maxBroadcastNodes peers that need in sync
	broadcastTimeout      int64 = 3 * 60 * 1000000000 // 3 mins
)

// use to push new block to outofsync node
type syncConfig struct {
	timestamp int64
	client    *downloader.Client
}

// NetworkNode ...
type NetworkNode struct {
	SelfPeer p2p.Peer
	IDCPeer  p2p.Peer
}

// Node represents a protocol-participating node in the network
type Node struct {
	Consensus              *bft.Consensus       // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BlockChannel           chan *types.Block    // The channel to receive new blocks from Node
	pendingTransactions    types.Transactions   // All the transactions received but not yet processed for Consensus
	transactionInConsensus []*types.Transaction // The transactions selected into the new block and under Consensus process
	pendingTxMutex         sync.Mutex

	blockchain *core.BlockChain   // The blockchain for the shard where this node belongs
	db         *ethdb.LDBDatabase // LevelDB to store blockchain.

	log log.Logger // Log utility

	ClientPeer *p2p.Peer      // The peer for the harmony tx generator client, used for leaders to return proof-of-accept
	Client     *client.Client // The presence of a client object means this node will also act as a client
	SelfPeer   p2p.Peer       // TODO(minhdoan): it could be duplicated with Self below whose is Alok work.
	IDCPeer    p2p.Peer

	Neighbors  sync.Map   // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer
	State      State      // State of the Node
	stateMutex sync.Mutex // mutex for change node state

	TxPool *core.TxPool
	Worker *worker.Worker

	// Client server (for wallet requests)
	clientServer *clientService.Server

	// Syncing component.
	downloaderServer       *downloader.Server
	stateSync              *syncing.StateSync
	syncingState           uint32
	peerRegistrationRecord map[uint32]*syncConfig // record registration time (unixtime) of peers begin in syncing

	// The p2p host used to send/receive p2p messages
	host host.Host

	// Channel to stop sending ping message
	StopPing chan struct{}

	// Signal channel for lost validators
	OfflinePeers chan p2p.Peer

	// For test only
	TestBankKeys      []*ecdsa.PrivateKey
	ContractKeys      []*ecdsa.PrivateKey
	ContractAddresses []common.Address
}

// Blockchain returns the blockchain from node
func (node *Node) Blockchain() *core.BlockChain {
	return node.blockchain
}

// Add new transactions to the pending transaction list
func (node *Node) addPendingTransactions(newTxs types.Transactions) {
	node.pendingTxMutex.Lock()
	node.pendingTransactions = append(node.pendingTransactions, newTxs...)
	node.pendingTxMutex.Unlock()
	node.log.Debug("Got more transactions", "num", len(newTxs), "totalPending", len(node.pendingTransactions))
}

// Take out a subset of valid transactions from the pending transaction list
// Note the pending transaction list will then contain the rest of the txs
func (node *Node) getTransactionsForNewBlock(maxNumTxs int) types.Transactions {
	node.pendingTxMutex.Lock()
	selected, unselected, invalid := node.Worker.SelectTransactionsForNewBlock(node.pendingTransactions, maxNumTxs)
	_ = invalid // invalid txs are discard

	node.log.Debug("Invalid transactions discarded", "number", len(invalid))
	node.pendingTransactions = unselected
	node.log.Debug("Remaining pending transactions", "number", len(node.pendingTransactions))
	node.pendingTxMutex.Unlock()
	return selected
}

// StartServer starts a server and process the requests by a handler.
func (node *Node) StartServer() {
	node.host.BindHandlerAndServe(node.StreamHandler)
}

// Count the total number of transactions in the blockchain
// Currently used for stats reporting purpose
func (node *Node) countNumTransactionsInBlockchain() int {
	count := 0
	for block := node.blockchain.CurrentBlock(); block != nil; block = node.blockchain.GetBlockByHash(block.Header().ParentHash) {
		count += len(block.Transactions())
	}
	return count
}

// SerializeNode serializes the node
// https://stackoverflow.com/questions/12854125/how-do-i-dump-the-struct-into-the-byte-array-without-reflection/12854659#12854659
func (node *Node) SerializeNode(nnode *NetworkNode) []byte {
	//Needs to escape the serialization of unexported fields
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(nnode)
	if err != nil {
		fmt.Println("Could not serialize node")
		fmt.Println("ERROR", err)
		//node.log.Error("Could not serialize node")
	}

	return result.Bytes()
}

// DeserializeNode deserializes the node
func DeserializeNode(d []byte) *NetworkNode {
	var wn NetworkNode
	r := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&wn)
	if err != nil {
		log.Error("Could not de-serialize node 1")
	}
	return &wn
}

// New creates a new node.
func New(host host.Host, consensus *bft.Consensus, db *ethdb.LDBDatabase) *Node {
	node := Node{}

	if host != nil {
		node.host = host
		node.SelfPeer = host.GetSelfPeer()
	}

	// Logger
	node.log = log.New()

	if host != nil && consensus != nil {
		// Consensus and associated channel to communicate blocks
		node.Consensus = consensus

		// Initialize level db.
		node.db = db

		// Initialize genesis block and blockchain
		genesisAlloc := node.CreateGenesisAllocWithTestingAddresses(100)
		contractKey, _ := ecdsa.GenerateKey(crypto.S256(), strings.NewReader("Test contract key string stream that is fixed so that generated test key are deterministic every time"))
		contractAddress := crypto.PubkeyToAddress(contractKey.PublicKey)
		contractFunds := big.NewInt(9000000)
		contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
		genesisAlloc[contractAddress] = core.GenesisAccount{Balance: contractFunds}
		node.ContractKeys = append(node.ContractKeys, contractKey)

		database := ethdb.NewMemDatabase()
		chainConfig := params.TestChainConfig
		chainConfig.ChainID = big.NewInt(int64(node.Consensus.ShardID)) // Use ChainID as piggybacked ShardID
		gspec := core.Genesis{
			Config:  chainConfig,
			Alloc:   genesisAlloc,
			ShardID: uint32(node.Consensus.ShardID),
		}

		_ = gspec.MustCommit(database)
		chain, _ := core.NewBlockChain(database, nil, gspec.Config, node.Consensus, vm.Config{}, nil)
		node.blockchain = chain
		node.BlockChannel = make(chan *types.Block)
		node.TxPool = core.NewTxPool(core.DefaultTxPoolConfig, params.TestChainConfig, chain)
		node.Worker = worker.New(params.TestChainConfig, chain, node.Consensus, pki.GetAddressFromPublicKey(node.SelfPeer.PubKey), node.Consensus.ShardID)

		node.AddSmartContractsToPendingTransactions()
	}

	if consensus != nil && consensus.IsLeader {
		node.State = NodeLeader
	} else {
		node.State = NodeInit
	}

	// Setup initial state of syncing.
	node.syncingState = NotDoingSyncing
	node.StopPing = make(chan struct{})
	node.Consensus.ConsensusBlock = make(chan *types.Block)
	node.Consensus.VerifiedNewBlock = make(chan *types.Block)
	node.peerRegistrationRecord = make(map[uint32]*syncConfig)

	node.OfflinePeers = make(chan p2p.Peer)
	go node.RemovePeersHandler()

	return &node
}

// IsOutOfSync checks whether the node is out of sync by comparing latest block with consensus block
func (node *Node) IsOutOfSync(consensusBlock *types.Block) bool {
	currentBlock := node.blockchain.CurrentBlock()
	latestHash := currentBlock.Hash()
	parentHash := consensusBlock.ParentHash()
	consensusHash := consensusBlock.Hash()
	// in general this case should not happen
	if bytes.Compare(latestHash[:], consensusHash[:]) == 0 {
		return false
	}
	if bytes.Compare(latestHash[:], parentHash[:]) == 0 {
		return false
	}
	return true
}

// DoSyncing wait for check status and starts syncing if out of sync
func (node *Node) DoSyncing() {
	for {
		consensusBlock := <-node.Consensus.ConsensusBlock
		if !node.IsOutOfSync(consensusBlock) {
			if node.State == NodeNotSync {
				node.log.Info("[sync] Node is now INSYNC!")
				node.stateSync = nil
			}
			node.stateMutex.Lock()
			node.State = NodeReadyForConsensus
			node.stateMutex.Unlock()
			continue
		} else {
			node.log.Debug("[sync] node is out of sync")
			node.stateMutex.Lock()
			node.State = NodeNotSync
			node.stateMutex.Unlock()
		}

		if node.stateSync == nil {
			node.stateSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port)
		}
		startHash := node.blockchain.CurrentBlock().Hash()
		node.stateSync.StartStateSync(startHash[:], node.GetSyncingPeers(), node.blockchain, node.Worker)
	}
}

// AddPeers adds neighbors nodes
func (node *Node) AddPeers(peers []*p2p.Peer) int {
	count := 0
	for _, p := range peers {
		key := fmt.Sprintf("%v", p.PubKey)
		_, ok := node.Neighbors.Load(key)
		if !ok {
			node.Neighbors.Store(key, *p)
			count++
			continue
		}
		if node.SelfPeer.ValidatorID == -1 && p.IP == node.SelfPeer.IP && p.Port == node.SelfPeer.Port {
			node.SelfPeer.ValidatorID = p.ValidatorID
		}
	}

	if count > 0 {
		node.Consensus.AddPeers(peers)
	}
	return count
}

// GetSyncingPort returns the syncing port.
func GetSyncingPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-syncing.SyncingPortDifference)
	}
	os.Exit(1)
	return ""
}

// GetSyncingPeers returns list of peers.
func (node *Node) GetSyncingPeers() []p2p.Peer {
	res := []p2p.Peer{}
	node.Neighbors.Range(func(k, v interface{}) bool {
		res = append(res, v.(p2p.Peer))
		return true
	})

	for i := range res {
		res[i].Port = GetSyncingPort(res[i].Port)
	}
	node.log.Debug("GetSyncingPeers: ", "res", res)
	return res
}

//AddSmartContractsToPendingTransactions adds the faucet contract the genesis block.
func (node *Node) AddSmartContractsToPendingTransactions() {
	// Add a contract deployment transactionv
	priKey := node.ContractKeys[0]
	contractData := "0x6080604052678ac7230489e8000060015560028054600160a060020a031916331790556101aa806100316000396000f3fe608060405260043610610045577c0100000000000000000000000000000000000000000000000000000000600035046327c78c42811461004a5780634ddd108a1461008c575b600080fd5b34801561005657600080fd5b5061008a6004803603602081101561006d57600080fd5b503573ffffffffffffffffffffffffffffffffffffffff166100b3565b005b34801561009857600080fd5b506100a1610179565b60408051918252519081900360200190f35b60025473ffffffffffffffffffffffffffffffffffffffff1633146100d757600080fd5b600154303110156100e757600080fd5b73ffffffffffffffffffffffffffffffffffffffff811660009081526020819052604090205460ff161561011a57600080fd5b73ffffffffffffffffffffffffffffffffffffffff8116600081815260208190526040808220805460ff1916600190811790915554905181156108fc0292818181858888f19350505050158015610175573d6000803e3d6000fd5b5050565b30319056fea165627a7a723058203e799228fee2fa7c5d15e71c04267a0cc2687c5eff3b48b98f21f355e1064ab30029"
	dataEnc := common.FromHex(contractData)
	// Unsigned transaction to avoid the case of transaction address.

	contractFunds := big.NewInt(8000000)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
	mycontracttx, _ := types.SignTx(types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, priKey)
	node.ContractAddresses = append(node.ContractAddresses, crypto.CreateAddress(crypto.PubkeyToAddress(priKey.PublicKey), uint64(0)))

	node.addPendingTransactions(types.Transactions{mycontracttx})
}

//CallFaucetContract invokes the faucet contract to give the walletAddress initial money
func (node *Node) CallFaucetContract(walletAddress common.Address) common.Hash {
	state, err := node.blockchain.State()
	if err != nil {
		log.Error("Failed to get chain state", "Error", err)
	}
	nonce := state.GetNonce(crypto.PubkeyToAddress(node.ContractKeys[0].PublicKey))
	callingFunction := "0x27c78c42000000000000000000000000"
	contractData := callingFunction + hex.EncodeToString(walletAddress.Bytes())
	dataEnc := common.FromHex(contractData)
	tx, _ := types.SignTx(types.NewTransaction(nonce, node.ContractAddresses[0], node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, node.ContractKeys[0])

	node.addPendingTransactions(types.Transactions{tx})
	return tx.Hash()
}

// JoinShard helps a new node to join a shard.
func (node *Node) JoinShard(leader p2p.Peer) {
	// try to join the shard, send ping message every 1 second, with a 10 minutes time-out
	tick := time.NewTicker(1 * time.Second)
	timeout := time.NewTicker(10 * time.Minute)
	defer tick.Stop()
	defer timeout.Stop()

	for {
		select {
		case <-tick.C:
			ping := proto_node.NewPingMessage(node.SelfPeer)
			if node.Client != nil { // assume this is the client node
				ping.Node.Role = proto_node.ClientRole
			}
			buffer := ping.ConstructPingMessage()

			// Talk to leader.
			node.SendMessage(leader, buffer)
		case <-timeout.C:
			node.log.Info("JoinShard timeout")
			return
		case <-node.StopPing:
			node.log.Info("Stopping JoinShard")
			return
		}
	}
}

// SupportClient initializes and starts the client service
func (node *Node) SupportClient() {
	node.InitClientServer()
	node.StartClientServer()
}

// SupportExplorer initializes and starts the client service
func (node *Node) SupportExplorer() {
	es := explorer.Service{
		IP:   node.SelfPeer.IP,
		Port: node.SelfPeer.Port,
	}
	es.Init(true)
	es.Run()
}

// InitClientServer initializes client server.
func (node *Node) InitClientServer() {
	node.clientServer = clientService.NewServer(node.blockchain.State, node.CallFaucetContract)
}

// StartClientServer starts client server.
func (node *Node) StartClientServer() {
	port, _ := strconv.Atoi(node.SelfPeer.Port)
	node.log.Info("support_client: StartClientServer on port:", "port", port+ClientServicePortDiff)
	node.clientServer.Start(node.SelfPeer.IP, strconv.Itoa(port+ClientServicePortDiff))
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
	port := GetSyncingPort(node.SelfPeer.Port)
	node.log.Info("support_sycning: StartSyncingServer on port:", "port", port)
	node.downloaderServer.Start(node.SelfPeer.IP, GetSyncingPort(node.SelfPeer.Port))
}

// CalculateResponse implements DownloadInterface on Node object.
func (node *Node) CalculateResponse(request *downloader_pb.DownloaderRequest) (*downloader_pb.DownloaderResponse, error) {
	response := &downloader_pb.DownloaderResponse{}
	switch request.Type {
	case downloader_pb.DownloaderRequest_HEADER:
		node.log.Debug("[sync] CalculateResponse DownloaderRequest_HEADER", "request.BlockHash", request.BlockHash)
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
	case downloader_pb.DownloaderRequest_NEWBLOCK:
		if node.State != NodeNotSync {
			node.log.Debug("[sync] state not in NodeNotSync", "state", node.State.String())
			response.Type = downloader_pb.DownloaderResponse_INSYNC
			return response, nil
		}
		var blockObj types.Block
		err := rlp.DecodeBytes(request.BlockHash, &blockObj)
		if err != nil {
			node.log.Warn("[sync] unable to decode received new block")
			return response, err
		}
		node.stateSync.AddNewBlock(request.PeerHash, &blockObj)

	case downloader_pb.DownloaderRequest_REGISTER:
		peerID := binary.BigEndian.Uint32(request.PeerHash)
		if _, ok := node.peerRegistrationRecord[peerID]; ok {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			return response, nil
		} else if len(node.peerRegistrationRecord) >= maxBroadcastNodes {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			return response, nil
		} else {
			peer, ok := node.Consensus.GetPeerFromID(peerID)
			if !ok {
				node.log.Warn("[sync] unable to get peer from peerID", "peerID", peerID)
			}
			client := downloader.ClientSetup(peer.IP, GetSyncingPort(peer.Port))
			if client == nil {
				node.log.Warn("[sync] unable to setup client")
				return response, nil
			}
			node.log.Debug("[sync] client setup correctly", "client", client)
			config := &syncConfig{timestamp: time.Now().UnixNano(), client: client}
			node.stateMutex.Lock()
			node.peerRegistrationRecord[peerID] = config
			node.stateMutex.Unlock()
			node.log.Debug("[sync] register peerID success", "peerID", peerID)
			response.Type = downloader_pb.DownloaderResponse_SUCCESS
		}
	case downloader_pb.DownloaderRequest_REGISTERTIMEOUT:
		if node.State == NodeNotSync {
			count := node.stateSync.RegisterNodeInfo()
			node.log.Debug("[sync] extra node registered", "number", count)
		}
	}
	return response, nil
}

func (node *Node) SendNewBlockToUnsync() {
	for {
		block := <-node.Consensus.VerifiedNewBlock
		blockHash, err := rlp.EncodeToBytes(block)
		if err != nil {
			node.log.Warn("[sync] unable to encode block to hashes")
			continue
		}

		// really need to have a unique id independent of ip/port
		selfPeerID := utils.GetUniqueIDFromIPPort(node.SelfPeer.IP, node.SelfPeer.Port)
		peerHash := make([]byte, 4)
		binary.BigEndian.PutUint32(peerHash, selfPeerID)
		node.log.Debug("[sync] peerRegistration Record", "peerHash", peerHash, "number", len(node.peerRegistrationRecord))

		for peerID, config := range node.peerRegistrationRecord {
			elapseTime := time.Now().UnixNano() - config.timestamp
			if elapseTime > broadcastTimeout {
				node.log.Warn("[sync] SendNewBlockToUnsync to peer timeout", "peerID", peerID)
				// send last time and delete
				config.client.PushNewBlock(peerHash, blockHash, true)
				node.stateMutex.Lock()
				delete(node.peerRegistrationRecord, peerID)
				node.stateMutex.Unlock()
				continue
			}
			response := config.client.PushNewBlock(peerHash, blockHash, false)
			node.log.Debug("[sync] SendNewBlockToUnSync from", "peerHash", peerHash)
			if response.Type == downloader_pb.DownloaderResponse_INSYNC {
				node.stateMutex.Lock()
				delete(node.peerRegistrationRecord, peerID)
				node.stateMutex.Unlock()
			}
		}
	}
}

// RemovePeersHandler is a goroutine to wait on the OfflinePeers channel
// and remove the peers from validator list
func (node *Node) RemovePeersHandler() {
	for {
		select {
		case p := <-node.OfflinePeers:
			node.Consensus.OfflinePeerList = append(node.Consensus.OfflinePeerList, p)
		}
	}
}
