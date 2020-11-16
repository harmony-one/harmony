package node

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/abool"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
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
	lru "github.com/hashicorp/golang-lru"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/semaphore"

	"github.com/rcrowley/go-metrics"
)

const (
	// NumTryBroadCast is the number of times trying to broadcast
	NumTryBroadCast = 3
	// MsgChanBuffer is the buffer of consensus message handlers.
	MsgChanBuffer = 1024
)

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
	SelfPeer    p2p.Peer
	// TODO: Neighbors should store only neighbor nodes in the same shard
	Neighbors  sync.Map   // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer
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
	// BroadcastInvalidTx flag is considered when adding pending tx to tx-pool
	BroadcastInvalidTx bool
	// InSync flag indicates the node is in-sync or not
	IsInSync *abool.AtomicBool

	deciderCache   *lru.Cache
	committeeCache *lru.Cache

	Metrics metrics.Registry
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
	errs := []error{}
	acceptCx := node.Blockchain().Config().AcceptsCrossTx(node.Blockchain().CurrentHeader().Epoch())
	for _, tx := range newTxs {
		if tx.ShardID() != tx.ToShardID() && !acceptCx {
			errs = append(errs, errors.WithMessage(errInvalidEpoch, "cross-shard tx not accepted yet"))
			continue
		}
		poolTxs = append(poolTxs, tx)
	}
	errs = append(errs, node.TxPool.AddRemotes(poolTxs)...)

	pendingCount, queueCount := node.TxPool.Stats()
	utils.Logger().Info().
		Interface("err", errs).
		Int("length of newTxs", len(newTxs)).
		Int("totalPending", pendingCount).
		Int("totalQueued", queueCount).
		Msg("[addPendingTransactions] Adding more transactions")
	return errs
}

// Add new staking transactions to the pending staking transaction list.
func (node *Node) addPendingStakingTransactions(newStakingTxs staking.StakingTransactions) []error {
	if node.NodeConfig.ShardID == shard.BeaconChainShardID {
		if node.Blockchain().Config().IsPreStaking(node.Blockchain().CurrentHeader().Epoch()) {
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
		return []error{
			errors.WithMessage(errInvalidEpoch, "staking txs not accepted yet"),
		}
	}
	return []error{
		errors.WithMessage(errInvalidShard, fmt.Sprintf("txs only valid on shard %v", shard.BeaconChainShardID)),
	}
}

// AddPendingStakingTransaction staking transactions
func (node *Node) AddPendingStakingTransaction(
	newStakingTx *staking.StakingTransaction,
) error {
	if node.NodeConfig.ShardID == shard.BeaconChainShardID {
		errs := node.addPendingStakingTransactions(staking.StakingTransactions{newStakingTx})
		var err error
		for i := range errs {
			if errs[i] != nil {
				utils.Logger().Info().
					Err(errs[i]).
					Msg("[AddPendingStakingTransaction] Failed adding new staking transaction")
				err = errs[i]
				break
			}
		}
		if err == nil || node.BroadcastInvalidTx {
			utils.Logger().Info().
				Str("Hash", newStakingTx.Hash().Hex()).
				Msg("Broadcasting Staking Tx")
			node.tryBroadcastStaking(newStakingTx)
		}
		return err
	}
	return nil
}

// AddPendingTransaction adds one new transaction to the pending transaction list.
// This is only called from SDK.
func (node *Node) AddPendingTransaction(newTx *types.Transaction) error {
	if newTx.ShardID() == node.NodeConfig.ShardID {
		errs := node.addPendingTransactions(types.Transactions{newTx})
		var err error
		for i := range errs {
			if errs[i] != nil {
				utils.Logger().Info().Err(errs[i]).Msg("[AddPendingTransaction] Failed adding new transaction")
				err = errs[i]
				break
			}
		}
		if err == nil || node.BroadcastInvalidTx {
			utils.Logger().Info().Str("Hash", newTx.Hash().Hex()).Msg("Broadcasting Tx")
			node.tryBroadcast(newTx)
		}
		return err
	}
	return errors.New("shard do not match")
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
	errNotRightKeySize   = errors.New("key received over wire is wrong size")
	errNoSenderPubKey    = errors.New("no sender public BLS key in message")
	errWrongSizeOfBitmap = errors.New("wrong size of sender bitmap")
	errWrongShardID      = errors.New("wrong shard id")
	errInvalidNodeMsg    = errors.New("invalid node message")
	errIgnoreBeaconMsg   = errors.New("ignore beacon sync block")
	errInvalidEpoch      = errors.New("invalid epoch for transaction")
	errInvalidShard      = errors.New("invalid shard")
)

// validateNodeMessage validate node message
func (node *Node) validateNodeMessage(ctx context.Context, payload []byte) (
	[]byte, proto_node.MessageType, error) {

	// length of payload must > p2pNodeMsgPrefixSize

	// reject huge node messages
	if len(payload) >= types.MaxEncodedPoolTransactionSize {
		NodeNodeMessageCounterVec.With(prometheus.Labels{"type": "invalid_oversized"}).Inc()
		return nil, 0, core.ErrOversizedData
	}

	// just ignore payload[0], which is MsgCategoryType (consensus/node)
	msgType := proto_node.MessageType(payload[proto.MessageCategoryBytes])

	switch msgType {
	case proto_node.Transaction:
		// nothing much to validate transaction message unless decode the RLP
		NodeNodeMessageCounterVec.With(prometheus.Labels{"type": "tx"}).Inc()
	case proto_node.Staking:
		// nothing much to validate staking message unless decode the RLP
		NodeNodeMessageCounterVec.With(prometheus.Labels{"type": "staking_tx"}).Inc()
	case proto_node.Block:
		switch proto_node.BlockMessageType(payload[p2pNodeMsgPrefixSize]) {
		case proto_node.Sync:
			NodeNodeMessageCounterVec.With(prometheus.Labels{"type": "block_sync"}).Inc()
			// only non-beacon nodes process the beacon block sync messages
			if node.Blockchain().ShardID() == shard.BeaconChainShardID {
				return nil, 0, errIgnoreBeaconMsg
			}
		case proto_node.SlashCandidate:
			NodeNodeMessageCounterVec.With(prometheus.Labels{"type": "slash"}).Inc()
			// only beacon chain node process slash candidate messages
			if node.NodeConfig.ShardID != shard.BeaconChainShardID {
				return nil, 0, errIgnoreBeaconMsg
			}
		case proto_node.Receipt:
			NodeNodeMessageCounterVec.With(prometheus.Labels{"type": "node_receipt"}).Inc()
		case proto_node.CrossLink:
			NodeNodeMessageCounterVec.With(prometheus.Labels{"type": "crosslink"}).Inc()
			// only beacon chain node process crosslink messages
			if node.NodeConfig.ShardID != shard.BeaconChainShardID ||
				node.NodeConfig.Role() == nodeconfig.ExplorerNode {
				return nil, 0, errIgnoreBeaconMsg
			}
		default:
			NodeNodeMessageCounterVec.With(prometheus.Labels{"type": "invalid_block_type"}).Inc()
			return nil, 0, errInvalidNodeMsg
		}
	default:
		NodeNodeMessageCounterVec.With(prometheus.Labels{"type": "invalid_node_type"}).Inc()
		return nil, 0, errInvalidNodeMsg
	}

	return payload[p2pNodeMsgPrefixSize:], msgType, nil
}

// validateShardBoundMessage validate consensus message
// validate shardID
// validate public key size
// verify message signature
func (node *Node) validateShardBoundMessage(
	ctx context.Context, payload []byte,
) (*msg_pb.Message, *bls.SerializedPublicKey, bool, error) {
	var (
		m msg_pb.Message
	)
	if err := protobuf.Unmarshal(payload, &m); err != nil {
		NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_unmarshal"}).Inc()
		return nil, nil, true, errors.WithStack(err)
	}

	// ignore messages not intended for explorer
	if node.NodeConfig.Role() == nodeconfig.ExplorerNode {
		switch m.Type {
		case
			msg_pb.MessageType_ANNOUNCE,
			msg_pb.MessageType_PREPARE,
			msg_pb.MessageType_COMMIT,
			msg_pb.MessageType_VIEWCHANGE,
			msg_pb.MessageType_NEWVIEW:
			NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
			return nil, nil, true, nil
		}
	}

	// when node is in ViewChanging mode, it still accepts normal messages into FBFTLog
	// in order to avoid possible trap forever but drop PREPARE and COMMIT
	// which are message types specifically for a node acting as leader
	// so we just ignore those messages
	if node.Consensus.IsViewChangingMode() {
		switch m.Type {
		case msg_pb.MessageType_PREPARE, msg_pb.MessageType_COMMIT:
			NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
			return nil, nil, true, nil
		}
	} else {
		// ignore viewchange/newview message if the node is not in viewchanging mode
		switch m.Type {
		case msg_pb.MessageType_NEWVIEW, msg_pb.MessageType_VIEWCHANGE:
			NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
			return nil, nil, true, nil
		}
	}

	// ignore message not intended for leader, but still forward them to the network
	if node.Consensus.IsLeader() {
		switch m.Type {
		case msg_pb.MessageType_ANNOUNCE, msg_pb.MessageType_PREPARED, msg_pb.MessageType_COMMITTED:
			NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
			return nil, nil, true, nil
		}
	}

	maybeCon, maybeVC := m.GetConsensus(), m.GetViewchange()
	senderKey := []byte{}
	senderBitmap := []byte{}

	if maybeCon != nil {
		if maybeCon.ShardId != node.Consensus.ShardID {
			NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_shard"}).Inc()
			return nil, nil, true, errors.WithStack(errWrongShardID)
		}
		senderKey = maybeCon.SenderPubkey

		if len(maybeCon.SenderPubkeyBitmap) > 0 {
			senderBitmap = maybeCon.SenderPubkeyBitmap
		}
	} else if maybeVC != nil {
		if maybeVC.ShardId != node.Consensus.ShardID {
			NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_shard"}).Inc()
			return nil, nil, true, errors.WithStack(errWrongShardID)
		}
		senderKey = maybeVC.SenderPubkey
	} else {
		NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid"}).Inc()
		return nil, nil, true, errors.WithStack(errNoSenderPubKey)
	}

	// ignore mesage not intended for validator
	// but still forward them to the network
	if !node.Consensus.IsLeader() {
		switch m.Type {
		case msg_pb.MessageType_PREPARE, msg_pb.MessageType_COMMIT:
			NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
			return nil, nil, true, nil
		}
	}

	serializedKey := bls.SerializedPublicKey{}
	if len(senderKey) > 0 {
		if len(senderKey) != bls.PublicKeySizeInBytes {
			NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_key_size"}).Inc()
			return nil, nil, true, errors.WithStack(errNotRightKeySize)
		}

		copy(serializedKey[:], senderKey)
		if !node.Consensus.IsValidatorInCommittee(serializedKey) {
			NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_committee"}).Inc()
			return nil, nil, true, errors.WithStack(shard.ErrValidNotInCommittee)
		}
	} else {
		count := node.Consensus.Decider.ParticipantsCount()
		if (count+7)>>3 != int64(len(senderBitmap)) {
			NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_participant_count"}).Inc()
			return nil, nil, true, errors.WithStack(errWrongSizeOfBitmap)
		}
	}

	NodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "valid"}).Inc()

	// serializedKey will be empty for multiSig sender
	return &m, &serializedKey, false, nil
}

var (
	errMsgHadNoHMYPayLoadAssumption      = errors.New("did not have sufficient size for hmy msg")
	errConsensusMessageOnUnexpectedTopic = errors.New("received consensus on wrong topic")
)

// Start kicks off the node message handling
func (node *Node) Start() error {
	// groupID and whether this topic is used for consensus
	type t struct {
		tp    nodeconfig.GroupID
		isCon bool
	}
	groups := map[nodeconfig.GroupID]bool{}

	// three topic subscribed by each validator
	for _, t := range []t{
		{node.NodeConfig.GetShardGroupID(), true},
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

	if !node.NodeConfig.IsOffline {
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
	}
	pubsub := node.host.PubSub()
	ownID := node.host.GetID()
	errChan := make(chan withError, 100)

	// p2p consensus message handler function
	type p2pHandlerConsensus func(
		ctx context.Context,
		msg *msg_pb.Message,
		key *bls.SerializedPublicKey,
	) error

	// other p2p message handler function
	type p2pHandlerElse func(
		ctx context.Context,
		rlpPayload []byte,
		actionType proto_node.MessageType,
	) error

	// interface pass to p2p message validator
	type validated struct {
		consensusBound bool
		handleC        p2pHandlerConsensus
		handleCArg     *msg_pb.Message
		handleE        p2pHandlerElse
		handleEArg     []byte
		senderPubKey   *bls.SerializedPublicKey
		actionType     proto_node.MessageType
	}

	isThisNodeAnExplorerNode := node.NodeConfig.Role() == nodeconfig.ExplorerNode
	NodeStringCounterVec.WithLabelValues("peerid", nodeconfig.GetPeerID().String()).Inc()

	for i := range allTopics {
		sub, err := allTopics[i].Topic.Subscribe()
		if err != nil {
			return err
		}

		topicNamed := allTopics[i].Name
		isConsensusBound := allTopics[i].consensusBound

		utils.Logger().Info().
			Str("topic", topicNamed).
			Msg("enabled topic validation pubsub messages")

		// register topic validator for each topic
		if err := pubsub.RegisterTopicValidator(
			topicNamed,
			// this is the validation function called to quickly validate every p2p message
			func(ctx context.Context, peer libp2p_peer.ID, msg *libp2p_pubsub.Message) libp2p_pubsub.ValidationResult {
				NodeP2PMessageCounterVec.With(prometheus.Labels{"type": "total"}).Inc()
				hmyMsg := msg.GetData()

				// first to validate the size of the p2p message
				if len(hmyMsg) < p2pMsgPrefixSize {
					// TODO (lc): block peers sending empty messages
					NodeP2PMessageCounterVec.With(prometheus.Labels{"type": "invalid_size"}).Inc()
					return libp2p_pubsub.ValidationReject
				}

				openBox := hmyMsg[p2pMsgPrefixSize:]

				// validate message category
				switch proto.MessageCategory(openBox[proto.MessageCategoryBytes-1]) {
				case proto.Consensus:
					// received consensus message in non-consensus bound topic
					if !isConsensusBound {
						NodeP2PMessageCounterVec.With(prometheus.Labels{"type": "invalid_bound"}).Inc()
						errChan <- withError{
							errors.WithStack(errConsensusMessageOnUnexpectedTopic), msg,
						}
						return libp2p_pubsub.ValidationReject
					}
					NodeP2PMessageCounterVec.With(prometheus.Labels{"type": "consensus_total"}).Inc()

					// validate consensus message
					validMsg, senderPubKey, ignore, err := node.validateShardBoundMessage(
						context.TODO(), openBox[proto.MessageCategoryBytes:],
					)

					if err != nil {
						errChan <- withError{err, msg.GetFrom()}
						return libp2p_pubsub.ValidationReject
					}

					// ignore the further processing of the p2p messages as it is not intended for this node
					if ignore {
						return libp2p_pubsub.ValidationAccept
					}

					msg.ValidatorData = validated{
						consensusBound: true,
						handleC:        node.Consensus.HandleMessageUpdate,
						handleCArg:     validMsg,
						senderPubKey:   senderPubKey,
					}
					return libp2p_pubsub.ValidationAccept

				case proto.Node:
					// node message is almost empty
					if len(openBox) <= p2pNodeMsgPrefixSize {
						NodeP2PMessageCounterVec.With(prometheus.Labels{"type": "invalid_size"}).Inc()
						return libp2p_pubsub.ValidationReject
					}
					NodeP2PMessageCounterVec.With(prometheus.Labels{"type": "node_total"}).Inc()
					validMsg, actionType, err := node.validateNodeMessage(
						context.TODO(), openBox,
					)
					if err != nil {
						switch err {
						case errIgnoreBeaconMsg:
							// ignore the further processing of the ignored messages as it is not intended for this node
							// but propogate the messages to other nodes
							return libp2p_pubsub.ValidationAccept
						default:
							// TODO (lc): block peers sending error messages
							errChan <- withError{err, msg.GetFrom()}
							return libp2p_pubsub.ValidationReject
						}
					}
					msg.ValidatorData = validated{
						consensusBound: false,
						handleE:        node.HandleNodeMessage,
						handleEArg:     validMsg,
						actionType:     actionType,
					}
					return libp2p_pubsub.ValidationAccept
				default:
					// ignore garbled messages
					NodeP2PMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
					return libp2p_pubsub.ValidationReject
				}

				select {
				case <-ctx.Done():
					if errors.Is(ctx.Err(), context.DeadlineExceeded) {
						utils.Logger().Warn().
							Str("topic", topicNamed).Msg("[context] exceeded validation deadline")
					}
					errChan <- withError{errors.WithStack(ctx.Err()), nil}
				default:
					return libp2p_pubsub.ValidationAccept
				}

				return libp2p_pubsub.ValidationReject
			},
			// WithValidatorTimeout is an option that sets a timeout for an (asynchronous) topic validator. By default there is no timeout in asynchronous validators.
			libp2p_pubsub.WithValidatorTimeout(250*time.Millisecond),
			// WithValidatorConcurrency set the concurernt validator, default is 1024
			libp2p_pubsub.WithValidatorConcurrency(p2p.SetAsideForConsensus),
			// WithValidatorInline is an option that sets the validation disposition to synchronous:
			// it will be executed inline in validation front-end, without spawning a new goroutine.
			// This is suitable for simple or cpu-bound validators that do not block.
			libp2p_pubsub.WithValidatorInline(true),
		); err != nil {
			return err
		}

		semConsensus := semaphore.NewWeighted(p2p.SetAsideForConsensus)
		msgChanConsensus := make(chan validated, MsgChanBuffer)

		// goroutine to handle consensus messages
		go func() {
			for m := range msgChanConsensus {
				// should not take more than 10 seconds to process one message
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				msg := m
				go func() {
					defer cancel()

					if semConsensus.TryAcquire(1) {
						defer semConsensus.Release(1)

						if isThisNodeAnExplorerNode {
							if err := node.explorerMessageHandler(
								ctx, msg.handleCArg,
							); err != nil {
								errChan <- withError{err, nil}
							}
						} else {
							if err := msg.handleC(ctx, msg.handleCArg, msg.senderPubKey); err != nil {
								errChan <- withError{err, msg.senderPubKey}
							}
						}
					}

					select {
					case <-ctx.Done():
						if errors.Is(ctx.Err(), context.DeadlineExceeded) {
							utils.Logger().Warn().
								Str("topic", topicNamed).Msg("[context] exceeded consensus message handler deadline")
						}
						errChan <- withError{errors.WithStack(ctx.Err()), nil}
					default:
						return
					}
				}()
			}
		}()

		semNode := semaphore.NewWeighted(p2p.SetAsideOtherwise)
		msgChanNode := make(chan validated, MsgChanBuffer)

		// goroutine to handle node messages
		go func() {
			for m := range msgChanNode {
				// should not take more than 10 seconds to process one message
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				msg := m
				go func() {
					defer cancel()
					if semNode.TryAcquire(1) {
						defer semNode.Release(1)

						if err := msg.handleE(ctx, msg.handleEArg, msg.actionType); err != nil {
							errChan <- withError{err, nil}
						}
					}

					select {
					case <-ctx.Done():
						if errors.Is(ctx.Err(), context.DeadlineExceeded) {
							utils.Logger().Warn().
								Str("topic", topicNamed).Msg("[context] exceeded node message handler deadline")
						}
						errChan <- withError{errors.WithStack(ctx.Err()), nil}
					default:
						return
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
					if validatedMessage.consensusBound {
						msgChanConsensus <- validatedMessage
					} else {
						msgChanNode <- validatedMessage
					}
				} else {
					// continue if ValidatorData is nil
					if nextMsg.ValidatorData == nil {
						continue
					}
				}
			}
		}()
	}

	for e := range errChan {
		utils.SampledLogger().Info().
			Interface("item", e.payload).
			Msgf("[p2p]: issue while handling incoming p2p message: %v", e.err)
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
	node.TransactionErrorSink = types.NewTransactionErrorSink()
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
	node.IsInSync = abool.NewBool(false)

	if host != nil && consensusObj != nil {
		// Consensus and associated channel to communicate blocks
		node.Consensus = consensusObj

		// Load the chains.
		blockchain := node.Blockchain() // this also sets node.isFirstTime if the DB is fresh
		beaconChain := node.Beaconchain()
		if b1, b2 := beaconChain == nil, blockchain == nil; b1 || b2 {
			var err error
			if b2 {
				shardID := node.NodeConfig.ShardID
				// HACK get the real error reason
				_, err = node.shardChains.ShardChain(shardID)
			} else {
				_, err = node.shardChains.ShardChain(shard.BeaconChainShardID)
			}
			fmt.Fprintf(os.Stderr, "Cannot initialize node: %v\n", err)
			os.Exit(-1)
		}

		node.BlockChannel = make(chan *types.Block)
		node.ConfirmedBlockChannel = make(chan *types.Block)
		node.BeaconBlockChannel = make(chan *types.Block)
		txPoolConfig := core.DefaultTxPoolConfig
		txPoolConfig.Blacklist = blacklist
		txPoolConfig.Journal = fmt.Sprintf("%v/%v", node.NodeConfig.DBDir, txPoolConfig.Journal)
		node.TxPool = core.NewTxPool(txPoolConfig, node.Blockchain().Config(), blockchain, node.TransactionErrorSink)
		node.CxPool = core.NewCxPool(core.CxPoolSize)
		node.Worker = worker.New(node.Blockchain().Config(), blockchain, chain.Engine)

		node.deciderCache, _ = lru.New(16)
		node.committeeCache, _ = lru.New(16)

		if node.Blockchain().ShardID() != shard.BeaconChainShardID {
			node.BeaconWorker = worker.New(
				node.Beaconchain().Config(), beaconChain, chain.Engine,
			)
		}

		node.pendingCXReceipts = map[string]*types.CXReceiptsProof{}
		node.Consensus.VerifiedNewBlock = make(chan *types.Block, 1)
		chain.Engine.SetBeaconchain(beaconChain)
		// the sequence number is the next block number to be added in consensus protocol, which is
		// always one more than current chain header block
		node.Consensus.SetBlockNum(blockchain.CurrentBlock().NumberU64() + 1)
	}

	utils.Logger().Info().
		Interface("genesis block header", node.Blockchain().GetHeaderByNumber(0)).
		Msg("Genesis block hash")
	// Setup initial state of syncing.
	node.peerRegistrationRecord = map[string]*syncConfig{}
	node.startConsensus = make(chan struct{})
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

	// init metrics
	NodeStringCounterVec.WithLabelValues("version", nodeconfig.GetVersion()).Inc()

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
		epoch, node.Consensus.Blockchain,
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
		if node.Consensus.GetPublicKeys().Contains(key.Object) {
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
		Beacon:       nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID),
		ShardGroupID: node.NodeConfig.GetShardGroupID(),
		Actions:      map[nodeconfig.GroupID]nodeconfig.ActionType{},
	}

	groups := []nodeconfig.GroupID{
		node.NodeConfig.GetShardGroupID(),
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
	shardState, err := node.Consensus.Blockchain.ReadShardState(epoch)
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

	for _, blskey := range node.Consensus.GetPublicKeys() {
		blsStr := blskey.Bytes.Hex()
		shardkey := bls.FromLibBLSPublicKeyUnsafe(blskey.Object)
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
func (node *Node) GetAddressForBLSKey(blskey *bls_core.PublicKey, epoch *big.Int) common.Address {
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
