package node

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/api/client"
	clientService "github.com/harmony-one/harmony/api/client/service"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/contracts"
	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/drand"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
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

const (
	// ClientServicePortDiff is the positive port diff for client service
	ClientServicePortDiff       = 5555
	maxBroadcastNodes           = 10                  // broadcast at most maxBroadcastNodes peers that need in sync
	broadcastTimeout      int64 = 3 * 60 * 1000000000 // 3 mins
	//SyncIDLength is the length of bytes for syncID
	SyncIDLength = 20
)

// use to push new block to outofsync node
type syncConfig struct {
	timestamp int64
	client    *downloader.Client
}

// Node represents a protocol-participating node in the network
type Node struct {
	Consensus              *consensus.Consensus // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BlockChannel           chan *types.Block    // The channel to send newly proposed blocks
	ConfirmedBlockChannel  chan *types.Block    // The channel to send confirmed blocks
	BeaconBlockChannel     chan *types.Block    // The channel to send beacon blocks for non-beaconchain nodes
	pendingTransactions    types.Transactions   // All the transactions received but not yet processed for Consensus
	transactionInConsensus []*types.Transaction // The transactions selected into the new block and under Consensus process
	pendingTxMutex         sync.Mutex
	DRand                  *drand.DRand // The instance for distributed randomness protocol

	blockchain  *core.BlockChain   // The blockchain for the shard where this node belongs
	beaconChain *core.BlockChain   // The blockchain for beacon chain.
	db          *ethdb.LDBDatabase // LevelDB to store blockchain.

	ClientPeer *p2p.Peer      // The peer for the harmony tx generator client, used for leaders to return proof-of-accept
	Client     *client.Client // The presence of a client object means this node will also act as a client
	SelfPeer   p2p.Peer       // TODO(minhdoan): it could be duplicated with Self below whose is Alok work.
	BCPeers    []p2p.Peer     // list of Beacon Chain Peers.  This is needed by all nodes.

	// TODO: Neighbors should store only neighbor nodes in the same shard
	Neighbors  sync.Map   // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer
	State      State      // State of the Node
	stateMutex sync.Mutex // mutex for change node state

	// BeaconNeighbors store only neighbor nodes in the beacon chain shard
	BeaconNeighbors sync.Map // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer

	TxPool       *core.TxPool
	Worker       *worker.Worker
	BeaconWorker *worker.Worker // worker for beacon chain

	// Client server (for wallet requests)
	clientServer *clientService.Server

	// Syncing component.
	syncID                 [SyncIDLength]byte // a unique ID for the node during the state syncing process with peers
	downloaderServer       *downloader.Server
	stateSync              *syncing.StateSync
	beaconSync             *syncing.StateSync
	peerRegistrationRecord map[string]*syncConfig // record registration time (unixtime) of peers begin in syncing

	// The p2p host used to send/receive p2p messages
	host p2p.Host

	// Service manager.
	serviceManager *service.Manager

	//Staked Accounts and Contract
	CurrentStakes          map[common.Address]*structs.StakeInfo //This will save the latest information about staked nodes.
	StakingContractAddress common.Address
	WithdrawStakeFunc      []byte

	// Demo account.
	DemoContractAddress      common.Address
	LotteryManagerPrivateKey *ecdsa.PrivateKey

	// Puzzle account.
	PuzzleContractAddress   common.Address
	PuzzleManagerPrivateKey *ecdsa.PrivateKey

	// Staking Account
	// TODO: leochen, can we use multiple account for staking?
	StakingAccount accounts.Account

	// For test only
	TestBankKeys                 []*ecdsa.PrivateKey
	ContractDeployerKey          *ecdsa.PrivateKey
	ContractDeployerCurrentNonce uint64 // The nonce of the deployer contract at current block
	ContractAddresses            []common.Address

	// For puzzle contracts
	AddressNonce sync.Map

	// Shard group Message Receiver
	shardGroupReceiver p2p.GroupReceiver

	// Global group Message Receiver, communicate with beacon chain, or cross-shard TX
	globalGroupReceiver p2p.GroupReceiver

	// Client Message Receiver to handle light client messages
	// Beacon leader needs to use this receiver to talk to new node
	clientReceiver p2p.GroupReceiver

	// Duplicated Ping Message Received
	duplicatedPing sync.Map

	// Channel to notify consensus service to really start consensus
	startConsensus chan struct{}

	// node configuration, including group ID, shard ID, etc
	NodeConfig *nodeconfig.ConfigType

	// map of service type to its message channel.
	serviceMessageChan map[service.Type]chan *msg_pb.Message

	// Used to call smart contract locally
	ContractCaller *contracts.ContractCaller

	accountManager *accounts.Manager
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

	node.pendingTransactions = unselected
	utils.GetLogInstance().Debug("Selecting Transactions", "remainPending", len(node.pendingTransactions), "selected", len(selected), "invalidDiscarded", len(invalid))
	node.pendingTxMutex.Unlock()
	return selected
}

// MaybeKeepSendingPongMessage keeps sending pong message if the current node is a leader.
func (node *Node) MaybeKeepSendingPongMessage() {
	if nodeconfig.GetDefaultConfig().IsLeader() {
		go node.SendPongMessage()
	}
}

// StartServer starts a server and process the requests by a handler.
func (node *Node) StartServer() {
	select {}
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

// GetSyncID returns the syncID of this node
func (node *Node) GetSyncID() [SyncIDLength]byte {
	return node.syncID
}

// New creates a new node.
func New(host p2p.Host, consensusObj *consensus.Consensus, db ethdb.Database, isArchival bool) *Node {
	var chain *core.BlockChain
	var err error
	var isFirstTime bool // if cannot get blockchain from database, then isFirstTime = true

	node := Node{}
	copy(node.syncID[:], GenerateRandomString(SyncIDLength))
	if host != nil {
		node.host = host
		node.SelfPeer = host.GetSelfPeer()
	}

	if host != nil && consensusObj != nil {
		// Consensus and associated channel to communicate blocks
		node.Consensus = consensusObj

		// TODO(ricl): placeholder. Set the account manager to node.accountManager
		// // Ensure that the AccountManager method works before the node has started.
		// // We rely on this in cmd/geth.
		// am, ephemeralKeystore, err := makeAccountManager(conf)
		// if err != nil {
		// 	return nil, err
		// }
		// node.accountManager = am

		// Init db
		database := db
		if database == nil {
			database = ethdb.NewMemDatabase()
			chain, err = node.GenesisBlockSetup(database, consensusObj.ShardID, false)
			isFirstTime = true
		} else {
			chain, err = node.InitBlockChainFromDB(db, node.Consensus, isArchival)
			isFirstTime = false
			if err != nil || chain == nil || chain.CurrentBlock().NumberU64() <= 0 {
				chain, err = node.GenesisBlockSetup(database, consensusObj.ShardID, isArchival)
				isFirstTime = true
			}
		}
		if err != nil {
			utils.GetLogInstance().Error("Error when setup blockchain", "err", err)
			os.Exit(1)
		}

		node.blockchain = chain

		node.BlockChannel = make(chan *types.Block)
		node.ConfirmedBlockChannel = make(chan *types.Block)
		node.BeaconBlockChannel = make(chan *types.Block)
		node.TxPool = core.NewTxPool(core.DefaultTxPoolConfig, params.TestChainConfig, chain)
		node.Worker = worker.New(params.TestChainConfig, chain, node.Consensus, pki.GetAddressFromPublicKey(node.SelfPeer.ConsensusPubKey), node.Consensus.ShardID)

		node.Consensus.VerifiedNewBlock = make(chan *types.Block)
		// the sequence number is the next block number to be added in consensus protocol, which is always one more than current chain header block
		node.Consensus.SetSeqNum(chain.CurrentBlock().NumberU64() + 1)

		// Add Faucet contract to all shards, so that on testnet, we can demo wallet in explorer
		// TODO (leo): we need to have support of cross-shard tx later so that the token can be transferred from beacon chain shard to other tx shards.
		if isFirstTime {
			// Setup one time smart contracts
			node.AddFaucetContractToPendingTransactions()
		} else {
			node.AddContractKeyAndAddress(scFaucet)
		}

		if node.Consensus.ShardID == 0 {
			// Contracts only exist in beacon chain
			if isFirstTime {
				// Setup one time smart contracts
				node.CurrentStakes = make(map[common.Address]*structs.StakeInfo)
				node.AddStakingContractToPendingTransactions() //This will save the latest information about staked nodes in current staked
			} else {
				node.AddContractKeyAndAddress(scStaking)
			}
		}
		if isFirstTime {
			// TODO(minhdoan): Think of a better approach to deploy smart contract.
			// This is temporary for demo purpose.
			node.AddLotteryContract()
			node.AddPuzzleContract()
		} else {
			node.AddContractKeyAndAddress(scLottery)
			node.AddContractKeyAndAddress(scPuzzle)
		}
	}

	node.ContractCaller = contracts.NewContractCaller(&db, node.blockchain, params.TestChainConfig)

	if consensusObj != nil && nodeconfig.GetDefaultConfig().IsLeader() {
		node.State = NodeLeader
	} else {
		node.State = NodeInit
	}

	// start the goroutine to receive client message
	// client messages are sent by clients, like txgen, wallet
	go node.ReceiveClientGroupMessage()

	// start the goroutine to receive group message
	go node.ReceiveGroupMessage()

	// start the goroutine to receive global message, used for cross-shard TX
	// FIXME (leo): we use beacon client topic as the global topic for now
	go node.ReceiveGlobalMessage()

	// Setup initial state of syncing.
	node.peerRegistrationRecord = make(map[string]*syncConfig)

	node.startConsensus = make(chan struct{})

	// Get the node config that's created in the harmony.go program.
	if consensusObj != nil {
		node.NodeConfig = nodeconfig.GetShardConfig(consensusObj.ShardID)
	} else {
		node.NodeConfig = nodeconfig.GetDefaultConfig()
	}

	return &node
}

// InitShardState initialize genesis shard state and update committee pub keys for consensus and drand
func (node *Node) InitShardState(isGenesis bool) {
	shardState := types.ShardState{}
	if isGenesis {
		// Store the genesis shard state into db.
		if node.Consensus != nil {
			if node.Consensus.ShardID == 0 {
				shardState = node.blockchain.StoreNewShardState(node.blockchain.CurrentBlock(), nil)
			} else {
				shardState = node.beaconChain.StoreNewShardState(node.beaconChain.CurrentBlock(), nil)
			}
		}
	} else {
		epochShardState, err := node.retrieveEpochShardState()
		if err != nil {
			utils.GetLogInstance().Error("[Shard State] Failed to decode epoch shard state", "error", err)
		}
		utils.GetLogInstance().Info("Successfully loaded epoch shard state")
		shardState = epochShardState.ShardState
	}

	// Update validator public keys
	for _, shard := range shardState {
		if shard.ShardID == node.Consensus.ShardID {
			pubKeys := []*bls.PublicKey{}
			for _, node := range shard.NodeList {
				blsPubKey := &bls.PublicKey{}
				blsPubKey.Deserialize(node.BlsPublicKey[:])
				pubKeys = append(pubKeys, blsPubKey)
			}
			node.Consensus.UpdatePublicKeys(pubKeys)
			node.DRand.UpdatePublicKeys(pubKeys)
			break
		}
	}
}

// AddPeers adds neighbors nodes
func (node *Node) AddPeers(peers []*p2p.Peer) int {
	count := 0
	for _, p := range peers {
		key := fmt.Sprintf("%s:%s:%s", p.IP, p.Port, p.PeerID)
		_, ok := node.Neighbors.LoadOrStore(key, *p)
		if !ok {
			// !ok means new peer is stored
			count++
			node.host.AddPeer(p)
			continue
		}
	}

	// Only leader needs to add the peer info into consensus
	// Validators will receive the updated peer info from Leader via pong message
	// TODO: remove this after fully migrating to beacon chain-based committee membership
	//if count > 0 && node.NodeConfig.IsLeader() {
	//	node.Consensus.AddPeers(peers)
	//	// TODO: make peers into a context object shared by consensus and drand
	//	node.DRand.AddPeers(peers)
	//}
	return count
}

// AddBeaconPeer adds beacon chain neighbors nodes
// Return false means new neighbor peer was added
// Return true means redundant neighbor peer wasn't added
func (node *Node) AddBeaconPeer(p *p2p.Peer) bool {
	key := fmt.Sprintf("%s:%s:%s", p.IP, p.Port, p.PeerID)
	_, ok := node.BeaconNeighbors.LoadOrStore(key, *p)
	return ok
}

// isBeacon = true if the node is beacon node
// isClient = true if the node light client(txgen,wallet)
func (node *Node) initNodeConfiguration() (service.NodeConfig, chan p2p.Peer) {
	chanPeer := make(chan p2p.Peer)

	nodeConfig := service.NodeConfig{
		IsBeacon:     node.NodeConfig.IsBeacon(),
		IsClient:     node.NodeConfig.IsClient(),
		Beacon:       p2p.GroupIDBeacon,
		ShardGroupID: node.NodeConfig.GetShardGroupID(),
		Actions:      make(map[p2p.GroupID]p2p.ActionType),
	}

	if nodeConfig.IsClient {
		nodeConfig.Actions[p2p.GroupIDBeaconClient] = p2p.ActionStart
	} else {
		nodeConfig.Actions[node.NodeConfig.GetShardGroupID()] = p2p.ActionStart
	}

	var err error
	node.shardGroupReceiver, err = node.host.GroupReceiver(node.NodeConfig.GetShardGroupID())
	if err != nil {
		utils.GetLogInstance().Error("Failed to create shard receiver", "msg", err)
	}

	node.globalGroupReceiver, err = node.host.GroupReceiver(p2p.GroupIDBeaconClient)
	if err != nil {
		utils.GetLogInstance().Error("Failed to create global receiver", "msg", err)
	}

	node.clientReceiver, err = node.host.GroupReceiver(node.NodeConfig.GetClientGroupID())
	if err != nil {
		utils.GetLogInstance().Error("Failed to create client receiver", "msg", err)
	}

	return nodeConfig, chanPeer
}

// AddBeaconChainDatabase adds database support for beaconchain blocks on normal sharding nodes (not BeaconChain node)
func (node *Node) AddBeaconChainDatabase(db ethdb.Database) {
	database := db
	if database == nil {
		database = ethdb.NewMemDatabase()
	}
	// TODO (chao) currently we use the same genesis block as normal shard
	chain, err := node.GenesisBlockSetup(database, 0, true)
	if err != nil {
		utils.GetLogInstance().Error("Error when doing genesis setup")
		os.Exit(1)
	}
	node.beaconChain = chain
	node.BeaconWorker = worker.New(params.TestChainConfig, chain, &consensus.Consensus{}, pki.GetAddressFromPublicKey(node.SelfPeer.ConsensusPubKey), node.Consensus.ShardID)
}

// InitBlockChainFromDB retrieves the latest blockchain and state available from the local database
func (node *Node) InitBlockChainFromDB(db ethdb.Database, consensus *consensus.Consensus, isArchival bool) (*core.BlockChain, error) {
	chainConfig := params.TestChainConfig
	if consensus != nil {
		chainConfig.ChainID = big.NewInt(int64(consensus.ShardID)) // Use ChainID as piggybacked ShardID
	}
	cacheConfig := core.CacheConfig{}
	if isArchival {
		cacheConfig = core.CacheConfig{Disabled: true, TrieNodeLimit: 256 * 1024 * 1024, TrieTimeLimit: 30 * time.Second}
	}
	chain, err := core.NewBlockChain(db, &cacheConfig, chainConfig, consensus, vm.Config{}, nil)
	return chain, err
}
