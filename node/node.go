package node

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/client"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	protonode "github.com/harmony-one/harmony/api/proto/protonode"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
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
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

// State is a state of a node.
type State byte

// All constants except the NodeLeader below are for validators only.
const (
	NodeWaitToJoin        State = iota // Node contacted BeaconChain, wait to join Shard
	NodeNotInSync                      // Node out of sync, might be just joined Shard or offline for a period of time
	NodeReadyForConsensus              // Node is ready for doing consensus
)

const (
	// NumTryBroadCast is the number of times trying to broadcast
	NumTryBroadCast         = 3
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
	// Channel to notify consensus service to really start consensus
	startConsensus chan struct{}
	// node configuration, including group ID, shard ID, etc
	NodeConfig *nodeconfig.ConfigType
	// Chain configuration.
	chainConfig params.ChainConfig
	// map of service type to its message channel.
	serviceMessageChan  map[service.Type]chan *msg_pb.Message
	isFirstTime         bool // the node was started with a fresh database
	unixTimeAtNodeStart int64
	// KeysToAddrs holds the addresses of bls keys run by the node
	KeysToAddrs      map[string]common.Address
	keysToAddrsEpoch *big.Int
	keysToAddrsMutex sync.Mutex
	// TransactionErrorSink contains error messages for any failed transaction, in memory only
	TransactionErrorSink *types.TransactionErrorSink
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
	msg := protonode.ConstructTransactionListMessageAccount(types.Transactions{tx})

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
	msg := protonode.ConstructStakingTransactionListMessageAccount(staking.StakingTransactions{stakingTx})

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

type withError struct {
	err     error
	payload interface{}
}

var (
	errNotRightKeySize = errors.New("key received over wire is wrong size")
	errNoSenderPubKey  = errors.New("no sender public BLS key in message")
)

func validateShardBoundMessage(
	ctx context.Context, payload []byte,
) (*msg_pb.Message, error) {
	var (
		m         msg_pb.Message
		senderKey *bls.PublicKey
	)

	if err := protobuf.Unmarshal(payload, &m); err != nil {
		return nil, errors.WithStack(err)
	}
	var senderPubKeyViaWire []byte
	maybeCon, maybeVC := m.GetConsensus(), m.GetViewchange()

	if maybeCon != nil {
		senderPubKeyViaWire = maybeCon.GetSenderPubkey()
	} else if maybeVC != nil {
		senderPubKeyViaWire = maybeVC.GetSenderPubkey()
	} else {
		return nil, errors.WithStack(errNoSenderPubKey)
	}

	if len(senderPubKeyViaWire) != shard.PublicKeySizeInBytes {
		return nil, errors.WithStack(errNotRightKeySize)
	}

	key, err := bls_cosi.BytesToBLSPublicKey(
		senderPubKeyViaWire,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	senderKey = key

	if err := consensus.VerifyMessageSig(senderKey, &m); err != nil {
		return nil, errors.WithStack(err)
	}

	return &m, nil
}

// Start kicks off the node message handling
func (node *Node) Start() error {
	type t struct {
		tp    nodeconfig.GroupID
		isCon bool
	}
	groups := map[nodeconfig.GroupID]bool{}
	for _, t := range []t{
		{node.NodeConfig.GetShardGroupID(), true},
		{nodeconfig.NewClientGroupIDByShardID(shard.BeaconChainShardID), false},
		{node.NodeConfig.GetClientGroupID(), false},
	} {
		if _, ok := groups[t.tp]; !ok {
			groups[t.tp] = t.isCon
		}
	}

	type u struct {
		p2p.NamedTopic
		consensusBound bool
	}

	var allTopics []u

	utils.Logger().Debug().
		Interface("topics-ended-up-with", groups).
		Uint32("shard-id", node.Consensus.ShardID).
		Msg("starting with these topics")

	for key, isCon := range groups {
		topicHandle, err := node.host.GetOrJoin(string(key))
		if err != nil {
			return err
		}
		allTopics = append(
			allTopics, u{
				NamedTopic:     p2p.NamedTopic{string(key), topicHandle},
				consensusBound: isCon,
			},
		)
	}

	errMsgHadNoHMYPayLoadAssumption := errors.New("did not have sufficient size for hmy msg")
	errConsensusMessageOnUnexpectedTopic := errors.New("received consensus on wrong topic")

	pubsub := node.host.PubSub()
	ownID := node.host.GetID()
	errChan := make(chan withError)

	type p2pHandlerConsensus func(
		ctx context.Context,
		msg *msg_pb.Message,
	) error

	type p2pHandlerElse func(
		ctx context.Context,
		rlpPayload []byte,
	) error

	type validated struct {
		consensusBound bool
		handleC        p2pHandlerConsensus
		handleCArg     *msg_pb.Message
		handleE        p2pHandlerElse
		handleEArg     []byte
	}

	isThisNodeAnExplorerNode := node.NodeConfig.Role() == nodeconfig.ExplorerNode

	for i := range allTopics {
		sub, err := allTopics[i].Topic.Subscribe()
		if err != nil {
			return err
		}

		topicNamed := allTopics[i].Name
		isConsensusBound := allTopics[i].consensusBound

		utils.Logger().Info().
			Str("topic", topicNamed).
			Msg("enabled topic validation on consensus bound messages")

		if err := pubsub.RegisterTopicValidator(
			topicNamed,
			func(ctx context.Context, peer libp2p_peer.ID, msg *libp2p_pubsub.Message) bool {
				hmyMsg := msg.GetData()
				if len(hmyMsg) < p2pMsgPrefixSize {
					pubsub.BlacklistPeer(peer)
					errChan <- withError{
						errors.WithStack(errMsgHadNoHMYPayLoadAssumption), nil,
					}
					return false
				}

				openBox := hmyMsg[p2pMsgPrefixSize:]

				switch proto.MessageCategory(openBox[proto.MessageCategoryBytes-1]) {
				case proto.Consensus:
					if !isConsensusBound {
						errChan <- withError{
							errors.WithStack(errConsensusMessageOnUnexpectedTopic), msg,
						}
						return false
					}

					validMsg, err := validateShardBoundMessage(
						context.TODO(), openBox[proto.MessageCategoryBytes:],
					)

					if err != nil {
						errChan <- withError{
							errors.WithStack(errMsgHadNoHMYPayLoadAssumption), msg,
						}
						return false
					}
					msg.ValidatorData = validated{
						consensusBound: true,
						handleC:        node.Consensus.HandleMessageUpdate,
						handleCArg:     validMsg,
					}
					return true

				case proto.Node:
					// TODO push the message parsing here, so can ban
					msg.ValidatorData = validated{
						consensusBound: false,
						handleE:        node.HandleNodeMessage,
						handleEArg:     openBox,
					}
				default:
					pubsub.BlacklistPeer(peer)
					return false
				}

				return true

			},
			libp2p_pubsub.WithValidatorTimeout(24),
			libp2p_pubsub.WithValidatorConcurrency(p2p.SetAsideForConsensus),
		); err != nil {
			return err
		}

		sem := semaphore.NewWeighted(p2p.MaxMessageHandlers)
		msgChan := make(chan validated)

		go func() {

			for msg := range msgChan {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				msg := msg

				go func() {
					defer cancel()

					if sem.TryAcquire(1) {
						defer sem.Release(1)

						if msg.consensusBound {
							if isThisNodeAnExplorerNode {
								if err := node.explorerMessageHandler(
									ctx, msg.handleCArg,
								); err != nil {
									errChan <- withError{err, nil}
								}
							} else {
								if err := msg.handleC(ctx, msg.handleCArg); err != nil {
									errChan <- withError{err, nil}
								}
							}
						} else {
							if err := msg.handleE(ctx, msg.handleEArg); err != nil {
								errChan <- withError{err, nil}
							}
						}

						select {
						case <-ctx.Done():
							if errors.Is(ctx.Err(), context.DeadlineExceeded) {
								utils.Logger().Info().
									Str("topic", topicNamed).Msg("exceeded deadline")
							}
							errChan <- withError{errors.WithStack(ctx.Err()), nil}
						default:
							return
						}
					}
				}()
			}
		}()

		go func() {

			for {
				nextMsg, err := sub.Next(context.Background())
				if err != nil {
					errChan <- withError{errors.WithStack(err), nil}
					continue
				}

				if nextMsg.GetFrom() == ownID {
					continue
				}

				if validatedMessage, ok := nextMsg.ValidatorData.(validated); ok {
					msgChan <- validatedMessage
				}

			}
		}()
	}

	for e := range errChan {
		utils.Logger().Debug().
			Interface("item", e.payload).
			Msgf("issue while handling incoming p2p message %v", e.err)
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
	node := Node{
		BlockChannel:           make(chan *types.Block),
		ConfirmedBlockChannel:  make(chan *types.Block),
		BeaconBlockChannel:     make(chan *types.Block),
		peerRegistrationRecord: map[string]*syncConfig{},
		startConsensus:         make(chan struct{}),
		unixTimeAtNodeStart:    time.Now().Unix(),
		TransactionErrorSink:   types.NewTransactionErrorSink(),
		serviceManager:         &service.Manager{},
		serviceMessageChan:     map[service.Type]chan *msg_pb.Message{},
	}
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

		txPoolConfig := core.DefaultTxPoolConfig
		txPoolConfig.Blacklist = blacklist
		node.TxPool = core.NewTxPool(
			txPoolConfig, node.Blockchain().Config(), blockchain, node.TransactionErrorSink,
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
