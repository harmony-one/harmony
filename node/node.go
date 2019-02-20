package node

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/harmony-one/harmony/drand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/client"
	clientService "github.com/harmony-one/harmony/api/client/service"
	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/api/service"
	service_manager "github.com/harmony-one/harmony/api/service"
	blockproposal "github.com/harmony-one/harmony/api/service/blockproposal"
	"github.com/harmony-one/harmony/api/service/clientsupport"
	consensus_service "github.com/harmony-one/harmony/api/service/consensus"
	"github.com/harmony-one/harmony/api/service/discovery"
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/api/service/networkinfo"
	randomness_service "github.com/harmony-one/harmony/api/service/randomness"

	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	bft "github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
)

// State is a state of a node.
type State byte

// All constants except the NodeLeader below are for validators only.
const (
	NodeInit              State = iota // Node just started, before contacting BeaconChain
	NodeWaitToJoin                     // Node contacted BeaconChain, wait to join Shard
	NodeNotInSync                      // Node out of sync, might be just joined Shard or offline for a period of time
	NodeOffline                        // Node is offline
	NodeReadyForConsensus              // Node is ready for doing consensus
	NodeDoingConsensus                 // Node is already doing consensus
	NodeLeader                         // Node is the leader of some shard.
)

// Role defines a role of a node.
type Role byte

// All constants for different node roles.
const (
	Unknown Role = iota
	ShardLeader
	ShardValidator
	BeaconLeader
	BeaconValidator
	NewNode
	ClientNode
)

func (state State) String() string {
	switch state {
	case NodeInit:
		return "NodeInit"
	case NodeWaitToJoin:
		return "NodeWaitToJoin"
	case NodeNotInSync:
		return "NodeNotInSync"
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
	lastMileThreshold = 4
	inSyncThreshold   = 1
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

//constants related to staking
//The first four bytes of the call data for a function call specifies the function to be called.
//It is the first (left, high-order in big-endian) four bytes of the Keccak-256 (SHA-3)
//Refer: https://solidity.readthedocs.io/en/develop/abi-spec.html

const (
	depositFuncSignature  = "0xd0e30db0"
	withdrawFuncSignature = "0x2e1a7d4d"
	funcSingatureBytes    = 4
)

// Node represents a protocol-participating node in the network
type Node struct {
	Consensus              *bft.Consensus       // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BlockChannel           chan *types.Block    // The channel to send newly proposed blocks
	ConfirmedBlockChannel  chan *types.Block    // The channel to send confirmed blocks
	pendingTransactions    types.Transactions   // All the transactions received but not yet processed for Consensus
	transactionInConsensus []*types.Transaction // The transactions selected into the new block and under Consensus process
	pendingTxMutex         sync.Mutex
	DRand                  *drand.DRand // The instance for distributed randomness protocol

	blockchain *core.BlockChain   // The blockchain for the shard where this node belongs
	db         *ethdb.LDBDatabase // LevelDB to store blockchain.

	ClientPeer *p2p.Peer      // The peer for the harmony tx generator client, used for leaders to return proof-of-accept
	Client     *client.Client // The presence of a client object means this node will also act as a client
	SelfPeer   p2p.Peer       // TODO(minhdoan): it could be duplicated with Self below whose is Alok work.
	BCPeers    []p2p.Peer     // list of Beacon Chain Peers.  This is needed by all nodes.

	// TODO: Neighbors should store only neighbor nodes in the same shard
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
	peerRegistrationRecord map[uint32]*syncConfig // record registration time (unixtime) of peers begin in syncing

	// The p2p host used to send/receive p2p messages
	host p2p.Host

	// Channel to stop sending ping message
	StopPing chan struct{}

	// Signal channel for lost validators
	OfflinePeers chan p2p.Peer

	// Node Role.
	Role Role

	// Service manager.
	serviceManager *service_manager.Manager

	//Staked Accounts and Contract
	CurrentStakes          map[common.Address]int64 //This will save the latest information about staked nodes.
	StakingContractAddress common.Address
	WithdrawStakeFunc      []byte

	//Node Account
	AccountKey *ecdsa.PrivateKey
	Address    common.Address

	// For test only
	TestBankKeys      []*ecdsa.PrivateKey
	ContractKeys      []*ecdsa.PrivateKey
	ContractAddresses []common.Address

	// Group Message Receiver
	groupReceiver p2p.GroupReceiver

	// Client Message Receiver to handle light client messages
	// Beacon leader needs to use this receiver to talk to new node
	clientReceiver p2p.GroupReceiver

	// Duplicated Ping Message Received
	duplicatedPing map[string]bool

	// Channel to notify consensus service to really start consensus
	startConsensus chan struct{}

	// My GroupID
	MyShardGroupID p2p.GroupID

	// My ShardClient GroupID
	MyClientGroupID p2p.GroupID
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
	utils.GetLogInstance().Debug("Got more transactions", "num", len(newTxs), "totalPending", len(node.pendingTransactions))
}

// Take out a subset of valid transactions from the pending transaction list
// Note the pending transaction list will then contain the rest of the txs
func (node *Node) getTransactionsForNewBlock(maxNumTxs int) types.Transactions {
	node.pendingTxMutex.Lock()
	selected, unselected, invalid := node.Worker.SelectTransactionsForNewBlock(node.pendingTransactions, maxNumTxs)
	_ = invalid // invalid txs are discard

	utils.GetLogInstance().Debug("Invalid transactions discarded", "number", len(invalid))
	node.pendingTransactions = unselected
	utils.GetLogInstance().Debug("Remaining pending transactions", "number", len(node.pendingTransactions), "selected", len(selected))
	node.pendingTxMutex.Unlock()
	return selected
}

// StartServer starts a server and process the requests by a handler.
func (node *Node) StartServer() {
	if utils.UseLibP2P {
		select {}
	} else {
		node.host.BindHandlerAndServe(node.StreamHandler)
	}
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

// New creates a new node.
func New(host p2p.Host, consensus *bft.Consensus, db ethdb.Database) *Node {
	node := Node{}

	if host != nil {
		node.host = host
		node.SelfPeer = host.GetSelfPeer()
	}

	if host != nil && consensus != nil {
		// Consensus and associated channel to communicate blocks
		node.Consensus = consensus

		// Init db.
		database := db
		if database == nil {
			database = ethdb.NewMemDatabase()
		}

		chain, err := node.GenesisBlockSetup(database)
		if err != nil {
			utils.GetLogInstance().Error("Error when doing genesis setup")
			os.Exit(1)
		}
		node.blockchain = chain
		node.BlockChannel = make(chan *types.Block)
		node.ConfirmedBlockChannel = make(chan *types.Block)

		node.TxPool = core.NewTxPool(core.DefaultTxPoolConfig, params.TestChainConfig, chain)
		node.Worker = worker.New(params.TestChainConfig, chain, node.Consensus, pki.GetAddressFromPublicKey(node.SelfPeer.PubKey), node.Consensus.ShardID)
		if node.Role == BeaconLeader || node.Role == BeaconValidator {
			node.CurrentStakes = make(map[common.Address]int64)
		}
		bytes, _ := rlp.EncodeToBytes(chain.GetBlockByNumber(0))
		utils.GetLogInstance().Debug("TESTTEST", "block", hex.EncodeToString(bytes))
		utils.GetLogInstance().Debug("Received", "blockHash", chain.GetBlockByNumber(0).Hash().Hex())
		node.Consensus.ConsensusBlock = make(chan *bft.BFTBlockInfo)
		node.Consensus.VerifiedNewBlock = make(chan *types.Block)
	}

	if consensus != nil && consensus.IsLeader {
		node.State = NodeLeader
		go node.ReceiveClientGroupMessage()
	} else {
		node.State = NodeInit
	}

	// Setup initial state of syncing.
	node.StopPing = make(chan struct{})
	node.peerRegistrationRecord = make(map[uint32]*syncConfig)

	node.OfflinePeers = make(chan p2p.Peer)
	go node.RemovePeersHandler()

	// start the goroutine to receive group message
	go node.ReceiveGroupMessage()

	node.duplicatedPing = make(map[string]bool)

	if utils.UseLibP2P {
		node.startConsensus = make(chan struct{})
	} else {
		node.startConsensus = nil
	}

	return &node
}

func (node *Node) getDeployedStakingContract() common.Address {
	return node.StakingContractAddress
}

//In order to get the deployed contract address of a contract, we need to find the nonce of the address that created it.
//(Refer: https://solidity.readthedocs.io/en/v0.5.3/introduction-to-smart-contracts.html#index-8)
// Then we can (re)create the deployed address. Trivially, this is 0 for us.
// The deployed contract address can also be obtained via the receipt of the contract creating transaction.
func (node *Node) generateDeployedStakingContractAddress(mycontracttx *types.Transaction, contractAddress common.Address) common.Address {
	//Ideally we send the transaction to

	//Correct Way 1:
	//node.SendTx(mycontracttx)
	//receipts := node.worker.GetCurrentReceipts()
	//deployedcontractaddress = recepits[len(receipts)-1].ContractAddress //get the address from the receipt

	//Correct Way 2:
	//nonce := GetNonce(contractAddress)
	//deployedAddress := crypto.CreateAddress(contractAddress, uint64(nonce))
	//deployedcontractaddress = recepits[len(receipts)-1].ContractAddress //get the address from the receipt
	nonce := 0
	return crypto.CreateAddress(contractAddress, uint64(nonce))
}

// IsOutOfSync checks whether the node is out of sync by comparing latest block with consensus block
func (node *Node) IsOutOfSync(consensusBlockInfo *bft.BFTBlockInfo) bool {
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
		// TODO: make peers into a context object shared by consensus and drand
		node.DRand.AddPeers(peers)
	}
	return count
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
			ping := proto_discovery.NewPingMessage(node.SelfPeer)
			if node.Client != nil { // assume this is the client node
				ping.Node.Role = proto_node.ClientRole
			}
			buffer := ping.ConstructPingMessage()

			// Talk to leader.
			node.SendMessage(leader, buffer)
		case <-timeout.C:
			utils.GetLogInstance().Info("JoinShard timeout")
			return
		case <-node.StopPing:
			utils.GetLogInstance().Info("Stopping JoinShard")
			return
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
		peerID := binary.BigEndian.Uint32(request.PeerHash)
		if _, ok := node.peerRegistrationRecord[peerID]; ok {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			utils.GetLogInstance().Warn("[SYNC] peerRegistration record already exists", "peerID", peerID)
			return response, nil
		} else if len(node.peerRegistrationRecord) >= maxBroadcastNodes {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			utils.GetLogInstance().Warn("[SYNC] maximum registration limit exceeds", "peerID", peerID)
			return response, nil
		} else {
			peer, ok := node.Consensus.GetPeerFromID(peerID)
			if !ok {
				utils.GetLogInstance().Warn("[SYNC] unable to get peer from peerID", "peerID", peerID)
			}
			client := downloader.ClientSetup(peer.IP, GetSyncingPort(peer.Port))
			if client == nil {
				utils.GetLogInstance().Warn("[SYNC] unable to setup client for peerID", "peerID", peerID)
				return response, nil
			}
			config := &syncConfig{timestamp: time.Now().UnixNano(), client: client}
			node.stateMutex.Lock()
			node.peerRegistrationRecord[peerID] = config
			node.stateMutex.Unlock()
			utils.GetLogInstance().Debug("[SYNC] register peerID success", "peerID", peerID)
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

//UpdateStakingList updates the stakes of every node.
func (node *Node) UpdateStakingList(block *types.Block) error {
	signerType := types.HomesteadSigner{}
	txns := block.Transactions()
	for i := range txns {
		txn := txns[i]
		toAddress := txn.To()
		if toAddress != nil && *toAddress != node.StakingContractAddress { //Not a address aimed at the staking contract.
			continue
		}
		currentSender, _ := types.Sender(signerType, txn)
		_, isPresent := node.CurrentStakes[currentSender]
		data := txn.Data()
		switch funcSignature := decodeFuncSign(data); funcSignature {
		case depositFuncSignature: //deposit, currently: 0xd0e30db0
			amount := txn.Value()
			value := amount.Int64()
			if isPresent {
				//This means the node has increased its stake.
				node.CurrentStakes[currentSender] += value
			} else {
				//This means its a new node that is staking the first time.
				node.CurrentStakes[currentSender] = value
			}
		case withdrawFuncSignature: //withdaw, currently: 0x2e1a7d4d
			value := decodeStakeCall(data)
			if isPresent {
				if node.CurrentStakes[currentSender] > value {
					node.CurrentStakes[currentSender] -= value
				} else if node.CurrentStakes[currentSender] == value {
					delete(node.CurrentStakes, currentSender)
				} else {
					continue //Overdraft protection.
				}
			} else {
				continue //no-op: a node that is not staked cannot withdraw stake.
			}
		default:
			continue //no-op if its not deposit or withdaw
		}
	}
	return nil
}

//The first four bytes of the call data for a function call specifies the function to be called.
//It is the first (left, high-order in big-endian) four bytes of the Keccak-256 (SHA-3)
//Refer: https://solidity.readthedocs.io/en/develop/abi-spec.html
func decodeStakeCall(getData []byte) int64 {
	value := new(big.Int)
	value.SetBytes(getData[funcSingatureBytes:]) //Escape the method call.
	return value.Int64()
}

//The first four bytes of the call data for a function call specifies the function to be called.
//It is the first (left, high-order in big-endian) four bytes of the Keccak-256 (SHA-3)
//Refer: https://solidity.readthedocs.io/en/develop/abi-spec.html
//gets the function signature from data.
func decodeFuncSign(data []byte) string {
	funcSign := hexutil.Encode(data[:funcSingatureBytes]) //The function signature is first 4 bytes of data in ethereum
	return funcSign
}

func (node *Node) initNodeConfiguration() (service.NodeConfig, chan p2p.Peer) {
	chanPeer := make(chan p2p.Peer)

	nodeConfig := service.NodeConfig{
		IsBeacon: false,
		IsClient: true,
		Beacon:   p2p.GroupIDBeacon,
		Group:    p2p.GroupIDUnknown,
		Actions:  make(map[p2p.GroupID]p2p.ActionType),
	}
	nodeConfig.Actions[p2p.GroupIDBeaconClient] = p2p.ActionStart

	var err error
	node.groupReceiver, err = node.host.GroupReceiver(p2p.GroupIDBeaconClient)
	if err != nil {
		utils.GetLogInstance().Error("create group receiver error", "msg", err)
	}

	return nodeConfig, chanPeer
}

func (node *Node) initBeaconNodeConfiguration() (service.NodeConfig, chan p2p.Peer) {
	chanPeer := make(chan p2p.Peer)

	nodeConfig := service.NodeConfig{
		IsBeacon: true,
		IsClient: true,
		Beacon:   p2p.GroupIDBeacon,
		Group:    p2p.GroupIDUnknown,
		Actions:  make(map[p2p.GroupID]p2p.ActionType),
	}
	nodeConfig.Actions[p2p.GroupIDBeaconClient] = p2p.ActionStart

	var err error
	node.groupReceiver, err = node.host.GroupReceiver(p2p.GroupIDBeacon)
	if err != nil {
		utils.GetLogInstance().Error("create group receiver error", "msg", err)
	}

	// All beacon chain node will subscribe to BeaconClient topic
	node.clientReceiver, err = node.host.GroupReceiver(p2p.GroupIDBeaconClient)
	if err != nil {
		utils.GetLogInstance().Error("create client receiver error", "msg", err)
	}
	node.MyClientGroupID = p2p.GroupIDBeaconClient

	return nodeConfig, chanPeer
}

func (node *Node) setupForShardLeader() {
	nodeConfig, chanPeer := node.initNodeConfiguration()

	// Register peer discovery service. No need to do staking for beacon chain node.
	node.serviceManager.RegisterService(service_manager.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer))
	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(service_manager.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer))

	// Register explorer service.
	node.serviceManager.RegisterService(service_manager.SupportExplorer, explorer.New(&node.SelfPeer))
	// Register consensus service.
	node.serviceManager.RegisterService(service_manager.Consensus, consensus_service.New(node.BlockChannel, node.Consensus, node.startConsensus))
	// Register new block service.
	node.serviceManager.RegisterService(service_manager.BlockProposal, blockproposal.New(node.Consensus.ReadySignal, node.WaitForConsensusReady))
	// Register client support service.
	node.serviceManager.RegisterService(service_manager.ClientSupport, clientsupport.New(node.blockchain.State, node.CallFaucetContract, node.getDeployedStakingContract, node.SelfPeer.IP, node.SelfPeer.Port))
	// Register randomness service
	node.serviceManager.RegisterService(service_manager.Randomness, randomness_service.New(node.DRand))
}

func (node *Node) setupForShardValidator() {
	nodeConfig, chanPeer := node.initNodeConfiguration()

	// Register peer discovery service. "0" is the beacon shard ID. No need to do staking for beacon chain node.
	node.serviceManager.RegisterService(service_manager.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer))
	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(service_manager.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer))
}

func (node *Node) setupForBeaconLeader() {
	nodeConfig, chanPeer := node.initBeaconNodeConfiguration()

	// Register peer discovery service. No need to do staking for beacon chain node.
	node.serviceManager.RegisterService(service_manager.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer))
	// Register networkinfo service.
	node.serviceManager.RegisterService(service_manager.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer))

	// Register consensus service.
	node.serviceManager.RegisterService(service_manager.Consensus, consensus_service.New(node.BlockChannel, node.Consensus, node.startConsensus))
	// Register new block service.
	node.serviceManager.RegisterService(service_manager.BlockProposal, blockproposal.New(node.Consensus.ReadySignal, node.WaitForConsensusReady))
	// Register client support service.
	node.serviceManager.RegisterService(service_manager.ClientSupport, clientsupport.New(node.blockchain.State, node.CallFaucetContract, node.getDeployedStakingContract, node.SelfPeer.IP, node.SelfPeer.Port))
	// Register randomness service
	node.serviceManager.RegisterService(service_manager.Randomness, randomness_service.New(node.DRand))
}

func (node *Node) setupForBeaconValidator() {
	nodeConfig, chanPeer := node.initBeaconNodeConfiguration()

	// Register peer discovery service. No need to do staking for beacon chain node.
	node.serviceManager.RegisterService(service_manager.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer))
	// Register networkinfo service.
	node.serviceManager.RegisterService(service_manager.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer))
	// Register randomness service
	node.serviceManager.RegisterService(service_manager.Randomness, randomness_service.New(node.DRand))
}

func (node *Node) setupForNewNode() {
	nodeConfig, chanPeer := node.initNodeConfiguration()

	// Register staking service.
	// node.serviceManager.RegisterService(service_manager.Staking, staking.New(node.AccountKey, 0, stakingPeer))
	// TODO: (leo) no need to start discovery service for new node until we received the sharding info
	// Register peer discovery service.
	node.serviceManager.RegisterService(service_manager.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer))
	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(service_manager.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer))

	// TODO: how to restart networkinfo and discovery service after receiving shard id info from beacon chain?
}

func (node *Node) setupForClientNode() {
	nodeConfig, chanPeer := node.initNodeConfiguration()

	// Register peer discovery service.
	node.serviceManager.RegisterService(service_manager.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer))
	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(service_manager.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer))
}

// ServiceManagerSetup setups service store.
func (node *Node) ServiceManagerSetup() {
	node.serviceManager = &service_manager.Manager{}
	switch node.Role {
	case ShardLeader:
		node.setupForShardLeader()
	case ShardValidator:
		node.setupForShardValidator()
	case BeaconLeader:
		node.setupForBeaconLeader()
	case BeaconValidator:
		node.setupForBeaconValidator()
	case NewNode:
		node.setupForNewNode()
	case ClientNode:
		node.setupForClientNode()
	}
}

// RunServices runs registered services.
func (node *Node) RunServices() {
	if node.serviceManager == nil {
		utils.GetLogInstance().Info("Service manager is not set up yet.")
		return
	}
	node.serviceManager.RunServices()
}
