package node

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	NodeJoinedShard                    // Node joined Shard, ready for consensus
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
	case NodeJoinedShard:
		return "NodeJoinedShard"
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
	syncingPortDifference = 3000
	waitBeforeJoinShard   = time.Second * 3
	timeOutToJoinShard    = time.Minute * 10
	// ClientServicePortDiff is the positive port diff for client service
	ClientServicePortDiff = 5555
)

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

	Neighbors sync.Map // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer
	State     State    // State of the Node

	TxPool *core.TxPool
	Worker *worker.Worker

	// Client server (for wallet requests)
	clientServer *clientService.Server

	// Syncing component.
	downloaderServer *downloader.Server
	stateSync        *syncing.StateSync
	syncingState     uint32

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

	node.OfflinePeers = make(chan p2p.Peer)
	go node.RemovePeersHandler()

	return &node
}

// DoSyncing starts syncing.
func (node *Node) DoSyncing() {
	// If this node is currently doing sync, another call for syncing will be returned immediately.
	if !atomic.CompareAndSwapUint32(&node.syncingState, NotDoingSyncing, DoingSyncing) {
		return
	}
	defer atomic.StoreUint32(&node.syncingState, NotDoingSyncing)
	if node.stateSync == nil {
		node.stateSync = syncing.GetStateSync()
	}
	if node.stateSync.StartStateSync(node.GetSyncingPeers(), node.blockchain) {
		node.log.Debug("DoSyncing: successfully sync")
		if node.State == NodeJoinedShard {
			node.State = NodeReadyForConsensus
		}
	} else {
		node.log.Debug("DoSyncing: failed to sync")
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
			node.host.AddPeer(p)
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
		return fmt.Sprintf("%d", port-syncingPortDifference)
	}
	os.Exit(1)
	return ""
}

// GetSyncingPeers returns list of peers.
// Right now, the list length is only 1 for testing.
func (node *Node) GetSyncingPeers() []p2p.Peer {
	res := []p2p.Peer{}
	node.Neighbors.Range(func(k, v interface{}) bool {
		node.log.Debug("GetSyncingPeers-Range: ", "k", k, "v", v)
		if len(res) == 0 {
			res = append(res, v.(p2p.Peer))
		}
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
	if request.Type == downloader_pb.DownloaderRequest_HEADER {

		for block := node.blockchain.CurrentBlock(); block != nil; block = node.blockchain.GetBlockByHash(block.Header().ParentHash) {
			blockHash := block.Hash()
			response.Payload = append(response.Payload, blockHash[:])
		}
	} else {
		for _, bytes := range request.Hashes {
			var hash common.Hash
			hash.SetBytes(bytes)
			block := node.blockchain.GetBlockByHash(hash)
			encodedBlock, err := rlp.EncodeToBytes(block)
			if err != nil {
				response.Payload = append(response.Payload, encodedBlock)
			}
		}
	}
	return response, nil
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
