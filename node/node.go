package node

import (
	"container/ring"
	"crypto/ecdsa"
	"fmt"
	"os"
	"strings"
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
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
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
	"github.com/harmony-one/harmony/shard/committee"
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

	TxPool *core.TxPool

	CxPool *core.CxPool // pool for missing cross shard receipts resend

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

	accountManager *accounts.Manager

	isFirstTime bool // the node was started with a fresh database
	// How long in second the leader needs to wait to propose a new block.
	BlockPeriod time.Duration

	// last time consensus reached for metrics
	lastConsensusTime int64
	// Last 1024 staking transaction error, only in memory
	errorSink struct {
		sync.Mutex
		failedStakingTxns *ring.Ring
		failedTxns        *ring.Ring
	}
}

// Blockchain returns the blockchain for the node's current shard.
func (node *Node) Blockchain() *core.BlockChain {
	shardID := node.NodeConfig.ShardID
	bc, err := node.shardChains.ShardChain(shardID)
	if err != nil {
		utils.Logger().Error().
			Uint32("shardID", shardID).
			Err(err).
			Msg("cannot get shard chain")
	}
	return bc
}

// Beaconchain returns the beaconchain from node.
func (node *Node) Beaconchain() *core.BlockChain {
	bc, err := node.shardChains.ShardChain(0)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("cannot get beaconchain")
	}
	return bc
}

// TODO: make this batch more transactions
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

func (node *Node) tryBroadcastStaking(stakingTx *staking.StakingTransaction) {
	msg := proto_node.ConstructStakingTransactionListMessageAccount(staking.StakingTransactions{stakingTx})

	shardGroupID := nodeconfig.NewGroupIDByShardID(
		nodeconfig.ShardID(shard.BeaconChainShardID),
	) // broadcast to beacon chain
	utils.Logger().Info().Str("shardGroupID", string(shardGroupID)).Msg("tryBroadcastStaking")

	for attempt := 0; attempt < NumTryBroadCast; attempt++ {
		if err := node.host.SendMessageToGroups([]nodeconfig.GroupID{shardGroupID}, p2p_host.ConstructP2pMessage(byte(0), msg)); err != nil && attempt < NumTryBroadCast {
			utils.Logger().Error().Int("attempt", attempt).Msg("Error when trying to broadcast staking tx")
		} else {
			break
		}
	}
}

// Add new transactions to the pending transaction list.
func (node *Node) addPendingTransactions(newTxs types.Transactions) {
	poolTxs := types.PoolTransactions{}
	for _, tx := range newTxs {
		poolTxs = append(poolTxs, tx)
	}
	node.TxPool.AddRemotes(poolTxs)

	pendingCount, queueCount := node.TxPool.Stats()
	utils.Logger().Info().
		Int("length of newTxs", len(newTxs)).
		Int("totalPending", pendingCount).
		Int("totalQueued", queueCount).
		Msg("Got more transactions")
}

// Add new staking transactions to the pending staking transaction list.
func (node *Node) addPendingStakingTransactions(newStakingTxs staking.StakingTransactions) {
	if node.NodeConfig.ShardID == shard.BeaconChainShardID &&
		node.Blockchain().Config().IsPreStaking(node.Blockchain().CurrentHeader().Epoch()) {
		poolTxs := types.PoolTransactions{}
		for _, tx := range newStakingTxs {
			poolTxs = append(poolTxs, tx)
		}
		node.TxPool.AddRemotes(poolTxs)
		pendingCount, queueCount := node.TxPool.Stats()
		utils.Logger().Info().
			Int("length of newStakingTxs", len(poolTxs)).
			Int("totalPending", pendingCount).
			Int("totalQueued", queueCount).
			Msg("Got more staking transactions")
	}
}

// AddPendingStakingTransaction staking transactions
func (node *Node) AddPendingStakingTransaction(newStakingTx *staking.StakingTransaction) {
	if node.NodeConfig.ShardID == shard.BeaconChainShardID {
		node.addPendingStakingTransactions(staking.StakingTransactions{newStakingTx})
	}
	utils.Logger().Info().Str("Hash", newStakingTx.Hash().Hex()).Msg("Broadcasting Staking Tx")
	node.tryBroadcastStaking(newStakingTx)
}

// AddPendingTransaction adds one new transaction to the pending transaction list.
// This is only called from SDK.
func (node *Node) AddPendingTransaction(newTx *types.Transaction) {
	if newTx.ShardID() == node.NodeConfig.ShardID {
		node.addPendingTransactions(types.Transactions{newTx})
	}
	utils.Logger().Info().Str("Hash", newTx.Hash().Hex()).Msg("Broadcasting Tx")
	node.tryBroadcast(newTx)
}

// AddPendingReceipts adds one receipt message to pending list.
func (node *Node) AddPendingReceipts(receipts *types.CXReceiptsProof) {
	node.pendingCXMutex.Lock()
	defer node.pendingCXMutex.Unlock()

	if receipts.ContainsEmptyField() {
		utils.Logger().Info().
			Int("totalPendingReceipts", len(node.pendingCXReceipts)).
			Msg("CXReceiptsProof contains empty field")
		return
	}

	blockNum := receipts.Header.Number().Uint64()
	shardID := receipts.Header.ShardID()

	// Sanity checks

	if err := node.Blockchain().Validator().ValidateCXReceiptsProof(receipts); err != nil {
		if !strings.Contains(err.Error(), rawdb.MsgNoShardStateFromDB) {
			utils.Logger().Error().Err(err).Msg("[AddPendingReceipts] Invalid CXReceiptsProof")
			return
		}
	}

	// cross-shard receipt should not be coming from our shard
	if s := node.Consensus.ShardID; s == shardID {
		utils.Logger().Info().
			Uint32("my-shard", s).
			Uint32("receipt-shard", shardID).
			Msg("ShardID of incoming receipt was same as mine")
		return
	}

	if e := receipts.Header.Epoch(); blockNum == 0 ||
		!node.Blockchain().Config().AcceptsCrossTx(e) {
		utils.Logger().Info().
			Uint64("incoming-epoch", e.Uint64()).
			Msg("Incoming receipt had meaningless epoch")
		return
	}

	key := utils.GetPendingCXKey(shardID, blockNum)

	// DDoS protection
	const maxCrossTxnSize = 4096
	if s := len(node.pendingCXReceipts); s >= maxCrossTxnSize {
		utils.Logger().Info().
			Int("pending-cx-receipts-size", s).
			Int("pending-cx-receipts-limit", maxCrossTxnSize).
			Msg("Current pending cx-receipts reached size limit")
		return
	}

	if _, ok := node.pendingCXReceipts[key]; ok {
		utils.Logger().Info().
			Int("totalPendingReceipts", len(node.pendingCXReceipts)).
			Msg("Already Got Same Receipt message")
		return
	}
	node.pendingCXReceipts[key] = receipts
	utils.Logger().Info().
		Int("totalPendingReceipts", len(node.pendingCXReceipts)).
		Msg("Got ONE more receipt message")
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
func New(host p2p.Host, consensusObj *consensus.Consensus,
	chainDBFactory shardchain.DBFactory, blacklist *map[common.Address]struct{}, isArchival bool) *Node {
	node := Node{}
	const sinkSize = 4096
	node.errorSink = struct {
		sync.Mutex
		failedStakingTxns *ring.Ring
		failedTxns        *ring.Ring
	}{sync.Mutex{}, ring.New(sinkSize), ring.New(sinkSize)}
	node.syncFreq = SyncFrequency
	node.beaconSyncFreq = SyncFrequency

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

	networkType := node.NodeConfig.GetNetworkType()
	chainConfig := networkType.ChainConfig()
	node.chainConfig = chainConfig

	collection := shardchain.NewCollection(
		chainDBFactory, &genesisInitializer{&node}, chain.Engine, &chainConfig,
	)
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
		if b1, b2 := beaconChain == nil, blockchain == nil; b1 || b2 {
			fmt.Fprintf(
				os.Stderr, "beaconchain-is-nil:%t shardchain-is-nil:%t", b1, b2,
			)
			os.Exit(-1)
		}

		node.BlockChannel = make(chan *types.Block)
		node.ConfirmedBlockChannel = make(chan *types.Block)
		node.BeaconBlockChannel = make(chan *types.Block)
		txPoolConfig := core.DefaultTxPoolConfig
		txPoolConfig.Blacklist = blacklist
		node.TxPool = core.NewTxPool(txPoolConfig, node.Blockchain().Config(), blockchain,
			func(payload []types.RPCTransactionError) {
				if len(payload) > 0 {
					node.errorSink.Lock()
					for i := range payload {
						node.errorSink.failedTxns.Value = payload[i]
						node.errorSink.failedTxns = node.errorSink.failedTxns.Next()
					}
					node.errorSink.Unlock()
				}
			},
			func(payload []staking.RPCTransactionError) {
				node.errorSink.Lock()
				for i := range payload {
					node.errorSink.failedStakingTxns.Value = payload[i]
					node.errorSink.failedStakingTxns = node.errorSink.failedStakingTxns.Next()
				}
				node.errorSink.Unlock()
			},
		)
		node.CxPool = core.NewCxPool(core.CxPoolSize)
		node.Worker = worker.New(node.Blockchain().Config(), blockchain, chain.Engine)

		if node.Blockchain().ShardID() != shard.BeaconChainShardID {
			node.BeaconWorker = worker.New(
				node.Beaconchain().Config(), beaconChain, chain.Engine,
			)
		}

		node.pendingCXReceipts = make(map[string]*types.CXReceiptsProof)
		node.Consensus.VerifiedNewBlock = make(chan *types.Block)
		chain.Engine.SetRewarder(node.Consensus.Decider.(reward.Distributor))
		chain.Engine.SetBeaconchain(beaconChain)

		// the sequence number is the next block number to be added in consensus protocol, which is
		// always one more than current chain header block
		node.Consensus.SetBlockNum(blockchain.CurrentBlock().NumberU64() + 1)

		// Add Faucet contract to all shards, so that on testnet, we can demo wallet in explorer
		if networkType != nodeconfig.Mainnet {
			if node.isFirstTime {
				// Setup one time smart contracts
				node.AddFaucetContractToPendingTransactions()
			} else {
				node.AddContractKeyAndAddress(scFaucet)
			}
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
	// Broadcast double-signers reported by consensus
	if node.Consensus != nil {
		go func() {
			for {
				select {
				case doubleSign := <-node.Consensus.SlashChan:
					go node.BroadcastSlash(&doubleSign)
				}
			}
		}()
	}
	return &node
}

// InitConsensusWithValidators initialize shard state from latest epoch and update committee pub
// keys for consensus and drand
func (node *Node) InitConsensusWithValidators() (err error) {
	if node.Consensus == nil {
		utils.Logger().Error().Msg("[InitConsensusWithValidators] consenus is nil; Cannot figure out shardID")
		return ctxerror.New("[InitConsensusWithValidators] consenus is nil; Cannot figure out shardID")
	}
	shardID := node.Consensus.ShardID
	blockNum := node.Blockchain().CurrentBlock().NumberU64()
	node.Consensus.SetMode(consensus.Listening)
	epoch := shard.Schedule.CalcEpochNumber(blockNum)
	utils.Logger().Info().
		Uint64("blockNum", blockNum).
		Uint32("shardID", shardID).
		Uint64("epoch", epoch.Uint64()).
		Msg("[InitConsensusWithValidators] Try To Get PublicKeys")
	shardState, err := committee.WithStakingEnabled.Compute(
		epoch, node.Consensus.ChainReader,
	)
	pubKeys := committee.WithStakingEnabled.GetCommitteePublicKeys(
		shardState.FindCommitteeByID(shardID),
	)
	if len(pubKeys) == 0 {
		utils.Logger().Error().
			Uint32("shardID", shardID).
			Uint64("blockNum", blockNum).
			Msg("[InitConsensusWithValidators] PublicKeys is Empty, Cannot update public keys")
		return ctxerror.New(
			"[InitConsensusWithValidators] PublicKeys is Empty, Cannot update public keys",
			"shardID", shardID,
			"blockNum", blockNum)
	}

	for i := range pubKeys {
		if pubKeys[i].IsEqual(node.Consensus.PubKey) {
			utils.Logger().Info().
				Uint64("blockNum", blockNum).
				Int("numPubKeys", len(pubKeys)).
				Msg("[InitConsensusWithValidators] Successfully updated public keys")
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

// ShutDown gracefully shut down the node server and dump the in-memory blockchain state into DB.
func (node *Node) ShutDown() {
	node.Blockchain().Stop()
	node.Beaconchain().Stop()
	msg := "Successfully shut down!\n"
	utils.Logger().Print(msg)
	fmt.Print(msg)
	os.Exit(0)
}
