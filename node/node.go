package node

import (
	"container/ring"
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/client"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
	common2 "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/harmony-one/harmony/webhooks"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
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
	Consensus             *consensus.Consensus              // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BlockChannel          chan *types.Block                 // The channel to send newly proposed blocks
	ConfirmedBlockChannel chan *types.Block                 // The channel to send confirmed blocks
	BeaconBlockChannel    chan *types.Block                 // The channel to send beacon blocks for non-beaconchain nodes
	pendingCXReceipts     map[string]*types.CXReceiptsProof // All the receipts received but not yet processed for Consensus
	pendingCXMutex        sync.Mutex
	// Shard databases
	shardChains shardchain.Collection
	Client      *client.Client // The presence of a client object means this node will also act as a client
	SelfPeer    p2p.Peer
	// TODO: Neighbors should store only neighbor nodes in the same shard
	Neighbors  sync.Map   // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer
	State      State      // State of the Node
	stateMutex sync.Mutex // mutex for change node state
	// BeaconNeighbors store only neighbor nodes in the beacon chain shard
	BeaconNeighbors      sync.Map // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer
	TxPool               *core.TxPool
	CxPool               *core.CxPool // pool for missing cross shard receipts resend
	Worker, BeaconWorker *worker.Worker
	downloaderServer     *downloader.Server
	// Syncing component.
	syncID                 [SyncIDLength]byte // a unique ID for the node during the state syncing process with peers
	stateSync, beaconSync  *syncing.StateSync
	peerRegistrationRecord map[string]*syncConfig // record registration time (unixtime) of peers begin in syncing
	SyncingPeerProvider    SyncingPeerProvider
	// The p2p host used to send/receive p2p messages
	host p2p.Host
	// Service manager.
	serviceManager               *service.Manager
	ContractDeployerKey          *ecdsa.PrivateKey
	ContractDeployerCurrentNonce uint64 // The nonce of the deployer contract at current block
	ContractAddresses            []common.Address
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
	isFirstTime        bool // the node was started with a fresh database
	// Last 1024 staking transaction error, only in memory
	errorSink struct {
		sync.Mutex
		failedStakingTxns *ring.Ring
		failedTxns        *ring.Ring
	}
	unixTimeAtNodeStart int64
	// KeysToAddrs holds the addresses of bls keys run by the node
	KeysToAddrs      map[string]common.Address
	keysToAddrsEpoch *big.Int
	keysToAddrsMutex sync.Mutex
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
	bc, err := node.shardChains.ShardChain(shard.BeaconChainShardID)
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
		if err := node.host.SendMessageToGroups([]nodeconfig.GroupID{shardGroupID},
			p2p.ConstructMessage(msg)); err != nil && attempt < NumTryBroadCast {
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
		if err := node.host.SendMessageToGroups([]nodeconfig.GroupID{shardGroupID},
			p2p.ConstructMessage(msg)); err != nil && attempt < NumTryBroadCast {
			utils.Logger().Error().Int("attempt", attempt).Msg("Error when trying to broadcast staking tx")
		} else {
			break
		}
	}
}

// Add new transactions to the pending transaction list.
func (node *Node) addPendingTransactions(newTxs types.Transactions) []error {
	poolTxs := types.PoolTransactions{}
	for _, tx := range newTxs {
		poolTxs = append(poolTxs, tx)
	}
	errs := node.TxPool.AddRemotes(poolTxs)

	pendingCount, queueCount := node.TxPool.Stats()
	utils.Logger().Info().
		Int("length of newTxs", len(newTxs)).
		Int("totalPending", pendingCount).
		Int("totalQueued", queueCount).
		Msg("Got more transactions")
	return errs
}

// Add new staking transactions to the pending staking transaction list.
func (node *Node) addPendingStakingTransactions(newStakingTxs staking.StakingTransactions) []error {
	if node.NodeConfig.ShardID == shard.BeaconChainShardID &&
		node.Blockchain().Config().IsPreStaking(node.Blockchain().CurrentHeader().Epoch()) {
		poolTxs := types.PoolTransactions{}
		for _, tx := range newStakingTxs {
			poolTxs = append(poolTxs, tx)
		}
		errs := node.TxPool.AddRemotes(poolTxs)
		pendingCount, queueCount := node.TxPool.Stats()
		utils.Logger().Info().
			Int("length of newStakingTxs", len(poolTxs)).
			Int("totalPending", pendingCount).
			Int("totalQueued", queueCount).
			Msg("Got more staking transactions")
		return errs
	}
	return make([]error, len(newStakingTxs))
}

// AddPendingStakingTransaction staking transactions
func (node *Node) AddPendingStakingTransaction(
	newStakingTx *staking.StakingTransaction,
) error {
	if node.NodeConfig.ShardID == shard.BeaconChainShardID {
		errs := node.addPendingStakingTransactions(staking.StakingTransactions{newStakingTx})
		for i := range errs {
			if errs[i] != nil {
				return errs[i]
			}
		}
		utils.Logger().Info().Str("Hash", newStakingTx.Hash().Hex()).Msg("Broadcasting Staking Tx")
		node.tryBroadcastStaking(newStakingTx)
	}
	return nil
}

// AddPendingTransaction adds one new transaction to the pending transaction list.
// This is only called from SDK.
func (node *Node) AddPendingTransaction(newTx *types.Transaction) error {
	if newTx.ShardID() == node.NodeConfig.ShardID {
		errs := node.addPendingTransactions(types.Transactions{newTx})
		for i := range errs {
			if errs[i] != nil {
				return errs[i]
			}
		}
		utils.Logger().Info().Str("Hash", newTx.Hash().Hex()).Msg("Broadcasting Tx")
		node.tryBroadcast(newTx)
	}
	return nil
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

// Start kicks off the node message handling
func (node *Node) Start() error {
	allTopics := node.host.AllTopics()
	if len(allTopics) == 0 {
		return errors.New("have no topics to listen to")
	}
	weighted := make([]*semaphore.Weighted, len(allTopics))
	const maxMessageHandlers = 200
	ctx := context.Background()
	ownID := node.host.GetID()
	errChan := make(chan error)

	for i, topic := range allTopics {
		sub, err := topic.Subscribe()
		if err != nil {
			return err
		}
		weighted[i] = semaphore.NewWeighted(maxMessageHandlers)
		msgChan := make(chan *libp2p_pubsub.Message)

		go func(msgChan chan *libp2p_pubsub.Message, sem *semaphore.Weighted) {
			for msg := range msgChan {
				payload := msg.GetData()
				if len(payload) < p2pMsgPrefixSize {
					continue
				}
				if sem.TryAcquire(1) {
					go func() {
						node.HandleMessage(
							payload[p2pMsgPrefixSize:], msg.GetFrom(),
						)
						sem.Release(1)
					}()
				} else {
					utils.Logger().Info().
						Msg("could not acquire semaphore to process incoming message")
				}
			}
		}(msgChan, weighted[i])

		go func(msgChan chan *libp2p_pubsub.Message) {
			for {
				nextMsg, err := sub.Next(ctx)
				if err != nil {
					errChan <- err
					continue
				}
				if nextMsg.GetFrom() == ownID {
					continue
				}
				msgChan <- nextMsg
			}
		}(msgChan)
	}

	for err := range errChan {
		utils.Logger().Info().Err(err).Msg("issue while handling incoming p2p message")
	}
	// NOTE never gets here
	return nil
}

// GetSyncID returns the syncID of this node
func (node *Node) GetSyncID() [SyncIDLength]byte {
	return node.syncID
}

// New creates a new node.
func New(
	host p2p.Host,
	consensusObj *consensus.Consensus,
	chainDBFactory shardchain.DBFactory,
	blacklist map[common.Address]struct{},
	isArchival bool,
) *Node {
	node := Node{}
	node.unixTimeAtNodeStart = time.Now().Unix()
	const sinkSize = 4096
	node.errorSink = struct {
		sync.Mutex
		failedStakingTxns *ring.Ring
		failedTxns        *ring.Ring
	}{sync.Mutex{}, ring.New(sinkSize), ring.New(sinkSize)}
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

			shardID := node.NodeConfig.ShardID
			// HACK get the real error reason
			_, err := node.shardChains.ShardChain(shardID)

			fmt.Fprintf(
				os.Stderr,
				"reason:%s beaconchain-is-nil:%t shardchain-is-nil:%t",
				err.Error(), b1, b2,
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

		node.pendingCXReceipts = map[string]*types.CXReceiptsProof{}
		node.Consensus.VerifiedNewBlock = make(chan *types.Block)
		chain.Engine.SetBeaconchain(beaconChain)
		// the sequence number is the next block number to be added in consensus protocol, which is
		// always one more than current chain header block
		node.Consensus.SetBlockNum(blockchain.CurrentBlock().NumberU64() + 1)

		// Add Faucet contract to all shards, so that on testnet, we can demo wallet in explorer
		if networkType != nodeconfig.Mainnet {
			if node.isFirstTime {
				// Setup one time smart contracts
				node.AddFaucetContractToPendingTransactions()
			}
		}
	}

	utils.Logger().Info().
		Interface("genesis block header", node.Blockchain().GetHeaderByNumber(0)).
		Msg("Genesis block hash")
	// Setup initial state of syncing.
	node.peerRegistrationRecord = map[string]*syncConfig{}
	node.startConsensus = make(chan struct{})
	go node.bootstrapConsensus()
	// Broadcast double-signers reported by consensus
	if node.Consensus != nil {
		go func() {
			for doubleSign := range node.Consensus.SlashChan {
				utils.Logger().Info().
					RawJSON("double-sign-candidate", []byte(doubleSign.String())).
					Msg("double sign notified by consensus leader")
				// no point to broadcast the slash if we aren't even in the right epoch yet
				if !node.Blockchain().Config().IsStaking(
					node.Blockchain().CurrentHeader().Epoch(),
				) {
					return
				}
				if hooks := node.NodeConfig.WebHooks.Hooks; hooks != nil {
					if s := hooks.Slashing; s != nil {
						url := s.OnNoticeDoubleSign
						go func() { webhooks.DoPost(url, &doubleSign) }()
					}
				}
				if node.NodeConfig.ShardID != shard.BeaconChainShardID {
					go node.BroadcastSlash(&doubleSign)
				} else {
					records := slash.Records{doubleSign}
					if err := node.Blockchain().AddPendingSlashingCandidates(
						records,
					); err != nil {
						utils.Logger().Err(err).Msg("could not add new slash to ending slashes")
					}
				}
			}
		}()
	}

	return &node
}

// InitConsensusWithValidators initialize shard state
// from latest epoch and update committee pub
// keys for consensus
func (node *Node) InitConsensusWithValidators() (err error) {
	if node.Consensus == nil {
		utils.Logger().Error().
			Msg("[InitConsensusWithValidators] consenus is nil; Cannot figure out shardID")
		return errors.New(
			"[InitConsensusWithValidators] consenus is nil; Cannot figure out shardID",
		)
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
	if err != nil {
		utils.Logger().Err(err).
			Uint64("blockNum", blockNum).
			Uint32("shardID", shardID).
			Uint64("epoch", epoch.Uint64()).
			Msg("[InitConsensusWithValidators] Failed getting shard state")
		return err
	}
	subComm, err := shardState.FindCommitteeByID(shardID)
	if err != nil {
		return err
	}
	pubKeys, err := subComm.BLSPublicKeys()
	if err != nil {
		utils.Logger().Error().
			Uint32("shardID", shardID).
			Uint64("blockNum", blockNum).
			Msg("[InitConsensusWithValidators] PublicKeys is Empty, Cannot update public keys")
		return errors.Wrapf(
			err,
			"[InitConsensusWithValidators] PublicKeys is Empty, Cannot update public keys",
		)
	}

	for _, key := range pubKeys {
		if node.Consensus.PubKey.Contains(key) {
			utils.Logger().Info().
				Uint64("blockNum", blockNum).
				Int("numPubKeys", len(pubKeys)).
				Msg("[InitConsensusWithValidators] Successfully updated public keys")
			node.Consensus.UpdatePublicKeys(pubKeys)
			node.Consensus.SetMode(consensus.Normal)
			return nil
		}
	}
	return nil
}

// AddPeers adds neighbors nodes
func (node *Node) AddPeers(peers []*p2p.Peer) int {
	for _, p := range peers {
		key := fmt.Sprintf("%s:%s:%s", p.IP, p.Port, p.PeerID)
		_, ok := node.Neighbors.LoadOrStore(key, *p)
		if !ok {
			// !ok means new peer is stored
			node.host.AddPeer(p)
			continue
		}
	}

	return node.host.GetPeerCount()
}

// AddBeaconPeer adds beacon chain neighbors nodes
// Return false means new neighbor peer was added
// Return true means redundant neighbor peer wasn't added
func (node *Node) AddBeaconPeer(p *p2p.Peer) bool {
	key := fmt.Sprintf("%s:%s:%s", p.IP, p.Port, p.PeerID)
	_, ok := node.BeaconNeighbors.LoadOrStore(key, *p)
	return ok
}

func (node *Node) initNodeConfiguration() (service.NodeConfig, chan p2p.Peer, error) {
	chanPeer := make(chan p2p.Peer)
	nodeConfig := service.NodeConfig{
		IsClient:     node.NodeConfig.IsClient(),
		Beacon:       nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID),
		ShardGroupID: node.NodeConfig.GetShardGroupID(),
		Actions:      map[nodeconfig.GroupID]nodeconfig.ActionType{},
	}

	if nodeConfig.IsClient {
		nodeConfig.Actions[nodeconfig.NewClientGroupIDByShardID(shard.BeaconChainShardID)] =
			nodeconfig.ActionStart
	} else {
		nodeConfig.Actions[node.NodeConfig.GetShardGroupID()] = nodeconfig.ActionStart
	}

	groups := []nodeconfig.GroupID{
		node.NodeConfig.GetShardGroupID(),
		nodeconfig.NewClientGroupIDByShardID(shard.BeaconChainShardID),
		node.NodeConfig.GetClientGroupID(),
	}

	// force the side effect of topic join
	if err := node.host.SendMessageToGroups(groups, []byte{}); err != nil {
		return nodeConfig, nil, err
	}

	return nodeConfig, chanPeer, nil
}

// ServiceManager ...
func (node *Node) ServiceManager() *service.Manager {
	return node.serviceManager
}

// ShutDown gracefully shut down the node server and dump the in-memory blockchain state into DB.
func (node *Node) ShutDown() {
	node.Blockchain().Stop()
	node.Beaconchain().Stop()
	const msg = "Successfully shut down!\n"
	utils.Logger().Print(msg)
	fmt.Print(msg)
	os.Exit(0)
}

func (node *Node) populateSelfAddresses(epoch *big.Int) {
	// reset the self addresses
	node.KeysToAddrs = map[string]common.Address{}
	node.keysToAddrsEpoch = epoch

	shardID := node.Consensus.ShardID
	shardState, err := node.Consensus.ChainReader.ReadShardState(epoch)
	if err != nil {
		utils.Logger().Error().Err(err).
			Int64("epoch", epoch.Int64()).
			Uint32("shard-id", shardID).
			Msg("[PopulateSelfAddresses] failed to read shard")
		return
	}

	committee, err := shardState.FindCommitteeByID(shardID)
	if err != nil {
		utils.Logger().Error().Err(err).
			Int64("epoch", epoch.Int64()).
			Uint32("shard-id", shardID).
			Msg("[PopulateSelfAddresses] failed to find shard committee")
		return
	}

	for _, blskey := range node.Consensus.PubKey.PublicKey {
		blsStr := blskey.SerializeToHexStr()
		shardkey := shard.FromLibBLSPublicKeyUnsafe(blskey)
		if shardkey == nil {
			utils.Logger().Error().
				Int64("epoch", epoch.Int64()).
				Uint32("shard-id", shardID).
				Str("blskey", blsStr).
				Msg("[PopulateSelfAddresses] failed to get shard key from bls key")
			return
		}
		addr, err := committee.AddressForBLSKey(*shardkey)
		if err != nil {
			utils.Logger().Error().Err(err).
				Int64("epoch", epoch.Int64()).
				Uint32("shard-id", shardID).
				Str("blskey", blsStr).
				Msg("[PopulateSelfAddresses] could not find address")
			return
		}
		node.KeysToAddrs[blsStr] = *addr
		utils.Logger().Debug().
			Int64("epoch", epoch.Int64()).
			Uint32("shard-id", shardID).
			Str("bls-key", blsStr).
			Str("address", common2.MustAddressToBech32(*addr)).
			Msg("[PopulateSelfAddresses]")
	}
}

// GetAddressForBLSKey retrieves the ECDSA address associated with bls key for epoch
func (node *Node) GetAddressForBLSKey(blskey *bls.PublicKey, epoch *big.Int) common.Address {
	// populate if first time setting or new epoch
	node.keysToAddrsMutex.Lock()
	defer node.keysToAddrsMutex.Unlock()
	if node.keysToAddrsEpoch == nil || epoch.Cmp(node.keysToAddrsEpoch) != 0 {
		node.populateSelfAddresses(epoch)
	}
	blsStr := blskey.SerializeToHexStr()
	addr, ok := node.KeysToAddrs[blsStr]
	if !ok {
		return common.Address{}
	}
	return addr
}

// GetAddresses retrieves all ECDSA addresses of the bls keys for epoch
func (node *Node) GetAddresses(epoch *big.Int) map[string]common.Address {
	// populate if first time setting or new epoch
	node.keysToAddrsMutex.Lock()
	defer node.keysToAddrsMutex.Unlock()
	if node.keysToAddrsEpoch == nil || epoch.Cmp(node.keysToAddrsEpoch) != 0 {
		node.populateSelfAddresses(epoch)
	}
	// self addresses map can never be nil
	return node.KeysToAddrs
}
