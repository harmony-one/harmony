package node

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/api/client"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/contracts"
	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/drand"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/memprofiling"
	"github.com/harmony-one/harmony/internal/shardchain"
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
	maxBroadcastNodes       = 10                  // broadcast at most maxBroadcastNodes peers that need in sync
	broadcastTimeout  int64 = 3 * 60 * 1000000000 // 3 mins
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

	// Shard databases
	shardChains shardchain.Collection

	ClientPeer *p2p.Peer      // The peer for the harmony tx generator client, used for leaders to return proof-of-accept
	Client     *client.Client // The presence of a client object means this node will also act as a client
	SelfPeer   p2p.Peer       // TODO(minhdoan): it could be duplicated with Self below whose is Alok work.
	BCPeers    []p2p.Peer     // list of Beacon Chain Peers.  This is needed by all nodes.

	// TODO: Neighbors should store only neighbor nodes in the same shard
	Neighbors  sync.Map   // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer
	numPeers   int        // Number of Peers
	State      State      // State of the Node
	stateMutex sync.Mutex // mutex for change node state

	// BeaconNeighbors store only neighbor nodes in the beacon chain shard
	BeaconNeighbors sync.Map // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer

	TxPool       *core.TxPool
	Worker       *worker.Worker
	BeaconWorker *worker.Worker // worker for beacon chain

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

	// For test only; TODO ek – remove this
	TestBankKeys []*ecdsa.PrivateKey

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

	// Next shard state
	nextShardState struct {
		// The received master shard state
		master *types.EpochShardState

		// When for a leader to propose the next shard state,
		// or for a validator to wait for a proposal before view change.
		// TODO ek – replace with retry-based logic instead of delay
		proposeTime time.Time
	}

	isFirstTime bool // the node was started with a fresh database
}

// Blockchain returns the blockchain for the node's current shard.
func (node *Node) Blockchain() *core.BlockChain {
	shardID := node.Consensus.ShardID
	bc, err := node.shardChains.ShardChain(shardID)
	if err != nil {
		err = ctxerror.New("cannot get shard chain", "shardID", shardID).
			WithCause(err)
		ctxerror.Log15(utils.GetLogger().Crit, err)
	}
	return bc
}

// Beaconchain returns the beaconchain from node.
func (node *Node) Beaconchain() *core.BlockChain {
	bc, err := node.shardChains.ShardChain(0)
	if err != nil {
		err = ctxerror.New("cannot get beaconchain").WithCause(err)
		ctxerror.Log15(utils.GetLogger().Crit, err)
	}
	return bc
}

// Add new transactions to the pending transaction list.
func (node *Node) addPendingTransactions(newTxs types.Transactions) {
	node.pendingTxMutex.Lock()
	node.pendingTransactions = append(node.pendingTransactions, newTxs...)
	node.pendingTxMutex.Unlock()
	utils.GetLogInstance().Debug("Got more transactions", "num", len(newTxs), "totalPending", len(node.pendingTransactions))
}

// AddPendingTransaction adds one new transaction to the pending transaction list.
func (node *Node) AddPendingTransaction(newTx *types.Transaction) {
	node.addPendingTransactions(types.Transactions{newTx})
	utils.GetLogInstance().Debug("Got ONE more transaction", "totalPending", len(node.pendingTransactions))
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
	for block := node.Blockchain().CurrentBlock(); block != nil; block = node.Blockchain().GetBlockByHash(block.Header().ParentHash) {
		count += len(block.Transactions())
	}
	return count
}

// GetSyncID returns the syncID of this node
func (node *Node) GetSyncID() [SyncIDLength]byte {
	return node.syncID
}

// WatchObservedObjects adds more objects to watch for memory issues.
func (node *Node) WatchObservedObjects() {
	memprofiling.GetMemProfiling().Add("currentNode.pendingTransactions", node.pendingTransactions)
	memprofiling.GetMemProfiling().Add("currentNode.transactionInConsensus", node.transactionInConsensus)
}

// New creates a new node.
func New(host p2p.Host, consensusObj *consensus.Consensus, chainDBFactory shardchain.DBFactory, isArchival bool) *Node {
	var err error

	node := Node{}
	copy(node.syncID[:], GenerateRandomString(SyncIDLength))
	if host != nil {
		node.host = host
		node.SelfPeer = host.GetSelfPeer()
	}

	// Create test keys.  Genesis will later need this.
	node.TestBankKeys, err = CreateTestBankKeys(FakeAddressNumber)
	if err != nil {
		utils.GetLogInstance().Crit("Error while creating test keys",
			"error", err)
	}

	collection := shardchain.NewCollection(
		chainDBFactory, &genesisInitializer{&node}, consensusObj)
	if isArchival {
		collection.DisableCache()
	}
	node.shardChains = collection

	if host != nil && consensusObj != nil {
		// Consensus and associated channel to communicate blocks
		node.Consensus = consensusObj

		// Load the chains.
		chain := node.Blockchain() // this also sets node.isFirstTime if the DB is fresh
		_ = node.Beaconchain()

		node.BlockChannel = make(chan *types.Block)
		node.ConfirmedBlockChannel = make(chan *types.Block)
		node.BeaconBlockChannel = make(chan *types.Block)
		node.TxPool = core.NewTxPool(core.DefaultTxPoolConfig, params.TestChainConfig, chain)
		node.Worker = worker.New(params.TestChainConfig, chain, node.Consensus, node.Consensus.SelfAddress, node.Consensus.ShardID)

		node.Consensus.VerifiedNewBlock = make(chan *types.Block)
		// the sequence number is the next block number to be added in consensus protocol, which is always one more than current chain header block
		node.Consensus.SetBlockNum(chain.CurrentBlock().NumberU64() + 1)

		// Add Faucet contract to all shards, so that on testnet, we can demo wallet in explorer
		// TODO (leo): we need to have support of cross-shard tx later so that the token can be transferred from beacon chain shard to other tx shards.
		if node.isFirstTime {
			// Setup one time smart contracts
			node.AddFaucetContractToPendingTransactions()
		} else {
			node.AddContractKeyAndAddress(scFaucet)
		}

		if node.Consensus.ShardID == 0 {
			// Contracts only exist in beacon chain
			if node.isFirstTime {
				// Setup one time smart contracts
				node.CurrentStakes = make(map[common.Address]*structs.StakeInfo)
				node.AddStakingContractToPendingTransactions() //This will save the latest information about staked nodes in current staked
			} else {
				node.AddContractKeyAndAddress(scStaking)
			}
		}
	}

	node.ContractCaller = contracts.NewContractCaller(node.Blockchain(), params.TestChainConfig)

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
func (node *Node) InitShardState(isGenesis bool) (err error) {
	logger := utils.GetLogInstance().New("isGenesis", isGenesis)
	getLogger := func() log.Logger { return utils.WithCallerSkip(logger, 1) }
	if node.Consensus == nil {
		getLogger().Crit("consensus is nil; cannot figure out shard ID")
	}
	shardID := node.Consensus.ShardID
	logger = logger.New("shardID", shardID)
	getLogger().Info("initializing shard state")

	// Get genesis epoch shard state from chain
	genesisEpoch := big.NewInt(core.GenesisEpoch)
	shardState, err := node.Beaconchain().GetShardState(genesisEpoch, nil)
	if err != nil {
		return ctxerror.New("cannot read genesis shard state").WithCause(err)
	}
	getLogger().Info("Successfully loaded epoch shard state")

	// Update validator public keys
	committee := shardState.FindCommitteeByID(shardID)
	if committee == nil {
		return ctxerror.New("our shard is not found in genesis shard state",
			"shardID", shardID)
	}
	pubKeys := []*bls.PublicKey{}
	for _, node := range committee.NodeList {
		pubKey := &bls.PublicKey{}
		pubKeyBytes := node.BlsPublicKey[:]
		err = pubKey.Deserialize(pubKeyBytes)
		if err != nil {
			return ctxerror.New("cannot deserialize BLS public key",
				"shardID", shardID,
				"pubKeyBytes", hex.EncodeToString(pubKeyBytes),
			).WithCause(err)
		}
		pubKeys = append(pubKeys, pubKey)
	}
	getLogger().Info("initialized shard state", "numPubKeys", len(pubKeys))
	node.Consensus.UpdatePublicKeys(pubKeys)
	node.DRand.UpdatePublicKeys(pubKeys)
	return nil
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
			node.numPeers++
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

// AccountManager ...
func (node *Node) AccountManager() *accounts.Manager {
	return node.accountManager
}
