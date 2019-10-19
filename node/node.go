package node

import (
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/api/client"
	clientService "github.com/harmony-one/harmony/api/client/service"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/contracts"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/values"
	"github.com/harmony-one/harmony/drand"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/msgq"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
	p2p_host "github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
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

const (
	// TxPoolLimit is the limit of transaction pool.
	TxPoolLimit = 20000
	// NumTryBroadCast is the number of times trying to broadcast
	NumTryBroadCast = 3
	// ClientRxQueueSize is the number of client messages to queue before tail-dropping.
	ClientRxQueueSize = 16384
	// ShardRxQueueSize is the number of shard messages to queue before tail-dropping.
	ShardRxQueueSize = 16384
	// GlobalRxQueueSize is the number of global messages to queue before tail-dropping.
	GlobalRxQueueSize = 16384
	// ClientRxWorkers is the number of concurrent client message handlers.
	ClientRxWorkers = 8
	// ShardRxWorkers is the number of concurrent shard message handlers.
	ShardRxWorkers = 32
	// GlobalRxWorkers is the number of concurrent global message handlers.
	GlobalRxWorkers = 32
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
	maxBroadcastNodes       = 10              // broadcast at most maxBroadcastNodes peers that need in sync
	broadcastTimeout  int64 = 60 * 1000000000 // 1 mins
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
	Consensus             *consensus.Consensus // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BlockChannel          chan *types.Block    // The channel to send newly proposed blocks
	ConfirmedBlockChannel chan *types.Block    // The channel to send confirmed blocks
	BeaconBlockChannel    chan *types.Block    // The channel to send beacon blocks for non-beaconchain nodes
	DRand                 *drand.DRand         // The instance for distributed randomness protocol
	pendingCrossLinks     []*block.Header
	pendingClMutex        sync.Mutex

	pendingCXReceipts map[string]*types.CXReceiptsProof // All the receipts received but not yet processed for Consensus
	pendingCXMutex    sync.Mutex

	// Shard databases
	shardChains shardchain.Collection

	Client   *client.Client // The presence of a client object means this node will also act as a client
	SelfPeer p2p.Peer       // TODO(minhdoan): it could be duplicated with Self below whose is Alok work.
	BCPeers  []p2p.Peer     // list of Beacon Chain Peers.  This is needed by all nodes.

	// TODO: Neighbors should store only neighbor nodes in the same shard
	Neighbors  sync.Map   // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer
	numPeers   int        // Number of Peers
	State      State      // State of the Node
	stateMutex sync.Mutex // mutex for change node state

	// BeaconNeighbors store only neighbor nodes in the beacon chain shard
	BeaconNeighbors sync.Map // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer

	TxPool *core.TxPool // TODO migrate to TxPool from pendingTransactions list below

	CxPool *core.CxPool // pool for missing cross shard receipts resend

	pendingTransactions map[common.Hash]*types.Transaction // All the transactions received but not yet processed for Consensus
	pendingTxMutex      sync.Mutex
	recentTxsStats      types.RecentTxsStats

	pendingStakingTransactions map[common.Hash]*staking.StakingTransaction // All the staking transactions received but not yet processed for Consensus
	pendingStakingTxMutex      sync.Mutex

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
	SyncingPeerProvider    SyncingPeerProvider

	// syncing frequency parameters
	syncFreq       int
	beaconSyncFreq int

	// The p2p host used to send/receive p2p messages
	host p2p.Host

	// Incoming messages to process.
	clientRxQueue *msgq.Queue
	shardRxQueue  *msgq.Queue
	globalRxQueue *msgq.Queue

	// Service manager.
	serviceManager *service.Manager

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

	// Chain configuration.
	chainConfig params.ChainConfig

	// map of service type to its message channel.
	serviceMessageChan map[service.Type]chan *msg_pb.Message

	// Used to call smart contract locally
	ContractCaller *contracts.ContractCaller

	accountManager *accounts.Manager

	// Next shard state
	nextShardState struct {
		// The received master shard state
		master *shard.EpochShardState

		// When for a leader to propose the next shard state,
		// or for a validator to wait for a proposal before view change.
		// TODO ek – replace with retry-based logic instead of delay
		proposeTime time.Time
	}

	isFirstTime bool // the node was started with a fresh database
	// How long in second the leader needs to wait to propose a new block.
	BlockPeriod time.Duration

	// last time consensus reached for metrics
	lastConsensusTime int64
}

// Blockchain returns the blockchain for the node's current shard.
func (node *Node) Blockchain() *core.BlockChain {
	shardID := node.NodeConfig.ShardID
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

func (node *Node) tryBroadcast(tx *types.Transaction) {
	msg := proto_node.ConstructTransactionListMessageAccount(types.Transactions{tx})

	shardGroupID := nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(tx.ShardID()))
	utils.Logger().Info().Str("shardGroupID", string(shardGroupID)).Msg("tryBroadcast")

	for attempt := 0; attempt < NumTryBroadCast; attempt++ {
		if err := node.host.SendMessageToGroups([]nodeconfig.GroupID{shardGroupID}, p2p_host.ConstructP2pMessage(byte(0), msg)); err != nil && attempt < NumTryBroadCast {
			utils.Logger().Error().Int("attempt", attempt).Msg("Error when trying to broadcast tx")
		} else {
			break
		}
	}
}

// Add new transactions to the pending transaction list.
func (node *Node) addPendingTransactions(newTxs types.Transactions) {
	txPoolLimit := core.ShardingSchedule.MaxTxPoolSizeLimit()
	node.pendingTxMutex.Lock()
	for _, tx := range newTxs {
		if _, ok := node.pendingTransactions[tx.Hash()]; !ok {
			node.pendingTransactions[tx.Hash()] = tx
		}
		if len(node.pendingTransactions) > txPoolLimit {
			break
		}
	}
	node.pendingTxMutex.Unlock()
	utils.Logger().Info().Int("length of newTxs", len(newTxs)).Int("totalPending", len(node.pendingTransactions)).Msg("Got more transactions")
}

// Add new staking transactions to the pending staking transaction list.
func (node *Node) addPendingStakingTransactions(newStakingTxs staking.StakingTransactions) {
	txPoolLimit := core.ShardingSchedule.MaxTxPoolSizeLimit()
	node.pendingStakingTxMutex.Lock()
	for _, tx := range newStakingTxs {
		if _, ok := node.pendingStakingTransactions[tx.Hash()]; !ok {
			node.pendingStakingTransactions[tx.Hash()] = tx
		}
		if len(node.pendingStakingTransactions) > txPoolLimit {
			break
		}
	}
	node.pendingStakingTxMutex.Unlock()
	utils.Logger().Info().Int("length of newStakingTxs", len(newStakingTxs)).Int("totalPending", len(node.pendingTransactions)).Msg("Got more staking transactions")
}

// AddPendingStakingTransaction staking transactions
func (node *Node) AddPendingStakingTransaction(
	newStakingTx *staking.StakingTransaction) {
	node.addPendingStakingTransactions(staking.StakingTransactions{newStakingTx})
}

// AddPendingTransaction adds one new transaction to the pending transaction list.
// This is only called from SDK.
func (node *Node) AddPendingTransaction(newTx *types.Transaction) {
	if node.Consensus.IsLeader() && newTx.ShardID() == node.NodeConfig.ShardID {
		node.addPendingTransactions(types.Transactions{newTx})
	} else {
		utils.Logger().Info().Str("Hash", newTx.Hash().Hex()).Msg("Broadcasting Tx")
		node.tryBroadcast(newTx)
	}
	utils.Logger().Debug().Int("totalPending", len(node.pendingTransactions)).Msg("Got ONE more transaction")
}

// AddPendingReceipts adds one receipt message to pending list.
func (node *Node) AddPendingReceipts(receipts *types.CXReceiptsProof) {
	node.pendingCXMutex.Lock()
	defer node.pendingCXMutex.Unlock()

	if receipts.ContainsEmptyField() {
		utils.Logger().Info().Int("totalPendingReceipts", len(node.pendingCXReceipts)).Msg("CXReceiptsProof contains empty field")
		return
	}

	blockNum := receipts.Header.Number().Uint64()
	shardID := receipts.Header.ShardID()
	key := utils.GetPendingCXKey(shardID, blockNum)

	if _, ok := node.pendingCXReceipts[key]; ok {
		utils.Logger().Info().Int("totalPendingReceipts", len(node.pendingCXReceipts)).Msg("Already Got Same Receipt message")
		return
	}
	node.pendingCXReceipts[key] = receipts
	utils.Logger().Info().Int("totalPendingReceipts", len(node.pendingCXReceipts)).Msg("Got ONE more receipt message")
}

// Take out a subset of valid transactions from the pending transaction list
// Note the pending transaction list will then contain the rest of the txs
func (node *Node) getTransactionsForNewBlock(
	coinbase common.Address,
) (types.Transactions, staking.StakingTransactions) {

	txsThrottleConfig := core.ShardingSchedule.TxsThrottleConfig()

	// the next block number to be added in consensus protocol, which is always one more than current chain header block
	newBlockNum := node.Blockchain().CurrentBlock().NumberU64() + 1

	// remove old (> txsThrottleConfigRecentTxDuration) blockNum keys from recentTxsStats and initiailize for the new block
	for blockNum := range node.recentTxsStats {
		recentTxsBlockNumGap := uint64(txsThrottleConfig.RecentTxDuration / node.BlockPeriod)
		if recentTxsBlockNumGap < newBlockNum-blockNum {
			delete(node.recentTxsStats, blockNum)
		}
	}
	node.recentTxsStats[newBlockNum] = make(types.BlockTxsCounts)

	// Must update to the correct current state before processing potential txns
	if err := node.Worker.UpdateCurrent(coinbase); err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("Failed updating worker's state before txn selection")
		return types.Transactions{}, staking.StakingTransactions{}
	}

	node.pendingTxMutex.Lock()
	defer node.pendingTxMutex.Unlock()
	node.pendingStakingTxMutex.Lock()
	defer node.pendingStakingTxMutex.Unlock()

	pendingTransactions := types.Transactions{}
	pendingStakingTransactions := staking.StakingTransactions{}

	for _, tx := range node.pendingTransactions {
		pendingTransactions = append(pendingTransactions, tx)
	}
	for _, tx := range node.pendingStakingTransactions {
		pendingStakingTransactions = append(pendingStakingTransactions, tx)
	}

	selected, unselected, invalid := node.Worker.SelectTransactionsForNewBlock(newBlockNum, pendingTransactions, node.recentTxsStats, txsThrottleConfig, coinbase)
	selectedStaking, unselectedStaking, invalidStaking :=
		node.Worker.SelectStakingTransactionsForNewBlock(newBlockNum, pendingStakingTransactions, coinbase)

	node.pendingTransactions = make(map[common.Hash]*types.Transaction)
	for _, unselectedTx := range unselected {
		node.pendingTransactions[unselectedTx.Hash()] = unselectedTx
	}
	utils.Logger().Info().
		Int("remainPending", len(node.pendingTransactions)).
		Int("selected", len(selected)).
		Int("invalidDiscarded", len(invalid)).
		Msg("Selecting Transactions")

	node.pendingStakingTransactions = make(map[common.Hash]*staking.StakingTransaction)
	for _, unselectedStakingTx := range unselectedStaking {
		node.pendingStakingTransactions[unselectedStakingTx.Hash()] = unselectedStakingTx
	}
	utils.Logger().Info().
		Int("remainPending", len(node.pendingStakingTransactions)).
		Int("selected", len(unselectedStaking)).
		Int("invalidDiscarded", len(invalidStaking)).
		Msg("Selecting Transactions")

	return selected, selectedStaking
}

func (node *Node) startRxPipeline(
	receiver p2p.GroupReceiver, queue *msgq.Queue, numWorkers int,
) {
	// consumers
	for i := 0; i < numWorkers; i++ {
		go queue.HandleMessages(node)
	}
	// provider
	go node.receiveGroupMessage(receiver, queue)
}

// StartServer starts a server and process the requests by a handler.
func (node *Node) StartServer() {

	// client messages are sent by clients, like txgen, wallet
	node.startRxPipeline(node.clientReceiver, node.clientRxQueue, ClientRxWorkers)

	// start the goroutine to receive group message
	node.startRxPipeline(node.shardGroupReceiver, node.shardRxQueue, ShardRxWorkers)

	// start the goroutine to receive global message, used for cross-shard TX
	// FIXME (leo): we use beacon client topic as the global topic for now
	node.startRxPipeline(node.globalGroupReceiver, node.globalRxQueue, GlobalRxWorkers)

	select {}
}

// Count the total number of transactions in the blockchain
// Currently used for stats reporting purpose
func (node *Node) countNumTransactionsInBlockchain() int {
	count := 0
	for block := node.Blockchain().CurrentBlock(); block != nil; block = node.Blockchain().GetBlockByHash(block.Header().ParentHash()) {
		count += len(block.Transactions())
	}
	return count
}

// GetSyncID returns the syncID of this node
func (node *Node) GetSyncID() [SyncIDLength]byte {
	return node.syncID
}

// New creates a new node.
func New(host p2p.Host, consensusObj *consensus.Consensus, chainDBFactory shardchain.DBFactory, isArchival bool) *Node {
	node := Node{}

	// Get the node config that's created in the harmony.go program.
	if consensusObj != nil {
		node.NodeConfig = nodeconfig.GetShardConfig(consensusObj.ShardID)
	} else {
		node.NodeConfig = nodeconfig.GetDefaultConfig()
	}

	copy(node.syncID[:], GenerateRandomString(SyncIDLength))
	if host != nil {
		node.host = host
		node.SelfPeer = host.GetSelfPeer()
	}

	chainConfig := *params.TestnetChainConfig
	switch node.NodeConfig.GetNetworkType() {
	case nodeconfig.Mainnet:
		chainConfig = *params.MainnetChainConfig
	case nodeconfig.Pangaea:
		chainConfig = *params.PangaeaChainConfig
	}
	node.chainConfig = chainConfig

	collection := shardchain.NewCollection(
		chainDBFactory, &genesisInitializer{&node}, chain.Engine, &chainConfig)
	if isArchival {
		collection.DisableCache()
	}
	node.shardChains = collection

	if host != nil && consensusObj != nil {
		// Consensus and associated channel to communicate blocks
		node.Consensus = consensusObj

		// Load the chains.
		blockchain := node.Blockchain() // this also sets node.isFirstTime if the DB is fresh
		beaconChain := node.Beaconchain()

		node.BlockChannel = make(chan *types.Block)
		node.ConfirmedBlockChannel = make(chan *types.Block)
		node.BeaconBlockChannel = make(chan *types.Block)
		node.recentTxsStats = make(types.RecentTxsStats)
		node.TxPool = core.NewTxPool(core.DefaultTxPoolConfig, node.Blockchain().Config(), blockchain)
		node.CxPool = core.NewCxPool(core.CxPoolSize)
		node.Worker = worker.New(node.Blockchain().Config(), blockchain, chain.Engine)

		if node.Blockchain().ShardID() != values.BeaconChainShardID {
			node.BeaconWorker = worker.New(node.Beaconchain().Config(), beaconChain, chain.Engine)
		}

		node.pendingCXReceipts = make(map[string]*types.CXReceiptsProof)
		node.pendingTransactions = make(map[common.Hash]*types.Transaction)
		node.pendingStakingTransactions = make(map[common.Hash]*staking.StakingTransaction)
		node.Consensus.VerifiedNewBlock = make(chan *types.Block)
		// the sequence number is the next block number to be added in consensus protocol, which is always one more than current chain header block
		node.Consensus.SetBlockNum(blockchain.CurrentBlock().NumberU64() + 1)

		// Add Faucet contract to all shards, so that on testnet, we can demo wallet in explorer
		// TODO (leo): we need to have support of cross-shard tx later so that the token can be transferred from beacon chain shard to other tx shards.
		if node.NodeConfig.GetNetworkType() != nodeconfig.Mainnet {
			if node.isFirstTime {
				// Setup one time smart contracts
				node.AddFaucetContractToPendingTransactions()
			} else {
				node.AddContractKeyAndAddress(scFaucet)
			}
			node.ContractCaller = contracts.NewContractCaller(node.Blockchain(), node.Blockchain().Config())
			// Create test keys.  Genesis will later need this.
			var err error
			node.TestBankKeys, err = CreateTestBankKeys(TestAccountNumber)
			if err != nil {
				utils.Logger().Error().Err(err).Msg("Error while creating test keys")
			}
		}
	}

	utils.Logger().Info().
		Interface("genesis block header", node.Blockchain().GetHeaderByNumber(0)).
		Msg("Genesis block hash")

	node.clientRxQueue = msgq.New(ClientRxQueueSize)
	node.shardRxQueue = msgq.New(ShardRxQueueSize)
	node.globalRxQueue = msgq.New(GlobalRxQueueSize)

	// Setup initial state of syncing.
	node.peerRegistrationRecord = make(map[string]*syncConfig)

	node.startConsensus = make(chan struct{})

	go node.bootstrapConsensus()

	return &node
}

// CalculateInitShardState initialize shard state from latest epoch and update committee pub keys for consensus and drand
func (node *Node) CalculateInitShardState() (err error) {
	if node.Consensus == nil {
		return ctxerror.New("[CalculateInitShardState] consenus is nil; Cannot figure out shardID")
	}
	shardID := node.Consensus.ShardID

	// Get genesis epoch shard state from chain
	blockNum := node.Blockchain().CurrentBlock().NumberU64()
	node.Consensus.SetMode(consensus.Listening)
	epoch := core.ShardingSchedule.CalcEpochNumber(blockNum)
	utils.Logger().Info().
		Uint64("blockNum", blockNum).
		Uint32("shardID", shardID).
		Uint64("epoch", epoch.Uint64()).
		Msg("[CalculateInitShardState] Try To Get PublicKeys from database")
	pubKeys := core.CalculatePublicKeys(epoch, shardID)
	if len(pubKeys) == 0 {
		return ctxerror.New(
			"[CalculateInitShardState] PublicKeys is Empty, Cannot update public keys",
			"shardID", shardID,
			"blockNum", blockNum)
	}

	for _, key := range pubKeys {
		if key.IsEqual(node.Consensus.PubKey) {
			utils.Logger().Info().
				Uint64("blockNum", blockNum).
				Int("numPubKeys", len(pubKeys)).
				Msg("[CalculateInitShardState] Successfully updated public keys")
			node.Consensus.UpdatePublicKeys(pubKeys)
			node.Consensus.SetMode(consensus.Normal)
			return nil
		}
	}
	// TODO: Disable drand. Currently drand isn't functioning but we want to compeletely turn it off for full protection.
	// node.DRand.UpdatePublicKeys(pubKeys)
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
// isClient = true if the node light client(wallet)
func (node *Node) initNodeConfiguration() (service.NodeConfig, chan p2p.Peer) {
	chanPeer := make(chan p2p.Peer)

	nodeConfig := service.NodeConfig{
		PushgatewayIP:   node.NodeConfig.GetPushgatewayIP(),
		PushgatewayPort: node.NodeConfig.GetPushgatewayPort(),
		IsClient:        node.NodeConfig.IsClient(),
		Beacon:          nodeconfig.NewGroupIDByShardID(0),
		ShardGroupID:    node.NodeConfig.GetShardGroupID(),
		Actions:         make(map[nodeconfig.GroupID]nodeconfig.ActionType),
	}

	if nodeConfig.IsClient {
		nodeConfig.Actions[nodeconfig.NewClientGroupIDByShardID(0)] = nodeconfig.ActionStart
	} else {
		nodeConfig.Actions[node.NodeConfig.GetShardGroupID()] = nodeconfig.ActionStart
	}

	var err error
	node.shardGroupReceiver, err = node.host.GroupReceiver(node.NodeConfig.GetShardGroupID())
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to create shard receiver")
	}

	node.globalGroupReceiver, err = node.host.GroupReceiver(nodeconfig.NewClientGroupIDByShardID(0))
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to create global receiver")
	}

	node.clientReceiver, err = node.host.GroupReceiver(node.NodeConfig.GetClientGroupID())
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to create client receiver")
	}
	return nodeConfig, chanPeer
}

// AccountManager ...
func (node *Node) AccountManager() *accounts.Manager {
	return node.accountManager
}

// ServiceManager ...
func (node *Node) ServiceManager() *service.Manager {
	return node.serviceManager
}

// SetSyncFreq sets the syncing frequency in the loop
func (node *Node) SetSyncFreq(syncFreq int) {
	node.syncFreq = syncFreq
}

// SetBeaconSyncFreq sets the syncing frequency in the loop
func (node *Node) SetBeaconSyncFreq(syncFreq int) {
	node.beaconSyncFreq = syncFreq
}
