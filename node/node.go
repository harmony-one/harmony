package node

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/internal/shardchain/tikv_manage"
	"github.com/harmony-one/harmony/internal/tikv"
	"github.com/harmony-one/harmony/internal/tikv/redis_helper"
	"github.com/harmony-one/harmony/internal/utils/lrucache"

	"github.com/ethereum/go-ethereum/rlp"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	"github.com/harmony-one/harmony/internal/utils/crosslinks"

	"github.com/ethereum/go-ethereum/common"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/abool"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	lru "github.com/hashicorp/golang-lru"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcrowley/go-metrics"
	"golang.org/x/sync/semaphore"

	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/legacysync"
	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	"github.com/harmony-one/harmony/api/service/stagedsync"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/harmony-one/harmony/staking/reward"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/harmony-one/harmony/webhooks"
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
	// Determine to send encoded BlockWithSig or Block
	withSig bool
}

type ISync interface {
	UpdateBlockAndStatus(block *types.Block, bc core.BlockChain, verifyAllSig bool) error
	AddLastMileBlock(block *types.Block)
	GetActivePeerNumber() int
	CreateSyncConfig(peers []p2p.Peer, shardID uint32, selfPeerID libp2p_peer.ID, waitForEachPeerToConnect bool) error
	SyncLoop(bc core.BlockChain, worker *worker.Worker, isBeacon bool, consensus *consensus.Consensus, loopMinTime time.Duration)
	IsSynchronized() bool
	IsSameBlockchainHeight(bc core.BlockChain) (uint64, bool)
	AddNewBlock(peerHash []byte, block *types.Block)
	RegisterNodeInfo() int
	GetParsedSyncStatus() (IsSynchronized bool, OtherHeight uint64, HeightDiff uint64)
	GetParsedSyncStatusDoubleChecked() (IsSynchronized bool, OtherHeight uint64, HeightDiff uint64)
}

// Node represents a protocol-participating node in the network
type Node struct {
	Consensus          *consensus.Consensus              // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BeaconBlockChannel chan *types.Block                 // The channel to send beacon blocks for non-beaconchain nodes
	pendingCXReceipts  map[string]*types.CXReceiptsProof // All the receipts received but not yet processed for Consensus
	pendingCXMutex     sync.Mutex
	crosslinks         *crosslinks.Crosslinks // Memory storage for crosslink processing.
	// Shard databases
	shardChains      shardchain.Collection
	SelfPeer         p2p.Peer
	stateMutex       sync.Mutex // mutex for change node state
	TxPool           *core.TxPool
	CxPool           *core.CxPool // pool for missing cross shard receipts resend
	Worker           *worker.Worker
	downloaderServer *downloader.Server
	// Syncing component.
	syncID                 [SyncIDLength]byte // a unique ID for the node during the state syncing process with peers
	stateSync              *legacysync.StateSync
	epochSync              *legacysync.EpochSync
	stateStagedSync        *stagedsync.StagedSync
	peerRegistrationRecord map[string]*syncConfig // record registration time (unixtime) of peers begin in syncing
	SyncingPeerProvider    SyncingPeerProvider
	// The p2p host used to send/receive p2p messages
	host p2p.Host
	// Service manager.
	serviceManager               *service.Manager
	ContractDeployerCurrentNonce uint64 // The nonce of the deployer contract at current block
	ContractAddresses            []common.Address
	HarmonyConfig                *harmonyconfig.HarmonyConfig
	// node configuration, including group ID, shard ID, etc
	NodeConfig *nodeconfig.ConfigType
	// Chain configuration.
	chainConfig         params.ChainConfig
	unixTimeAtNodeStart int64
	// KeysToAddrs holds the addresses of bls keys run by the node
	keysToAddrs      *lrucache.Cache[uint64, map[string]common.Address]
	keysToAddrsMutex sync.Mutex
	// TransactionErrorSink contains error messages for any failed transaction, in memory only
	TransactionErrorSink *types.TransactionErrorSink
	// BroadcastInvalidTx flag is considered when adding pending tx to tx-pool
	BroadcastInvalidTx bool
	// InSync flag indicates the node is in-sync or not
	IsSynchronized *abool.AtomicBool

	deciderCache   *lru.Cache
	committeeCache *lru.Cache

	Metrics metrics.Registry

	// context control for pub-sub handling
	psCtx    context.Context
	psCancel func()
	registry *registry.Registry
}

// Blockchain returns the blockchain for the node's current shard.
func (node *Node) Blockchain() core.BlockChain {
	return node.registry.GetBlockchain()
}

func (node *Node) SyncInstance() ISync {
	return node.GetOrCreateSyncInstance(true)
}

func (node *Node) CurrentSyncInstance() bool {
	return node.GetOrCreateSyncInstance(false) != nil
}

// GetOrCreateSyncInstance returns an instance of state sync, either legacy or staged
// if initiate sets to true, it generates a new instance
func (node *Node) GetOrCreateSyncInstance(initiate bool) ISync {
	if node.NodeConfig.StagedSync {
		if initiate && node.stateStagedSync == nil {
			utils.Logger().Info().Msg("initializing staged state sync")
			node.stateStagedSync = node.createStagedSync(node.Blockchain())
		}
		return node.stateStagedSync
	}
	if initiate && node.stateSync == nil {
		utils.Logger().Info().Msg("initializing legacy state sync")
		node.stateSync = node.createStateSync(node.Beaconchain())
	}
	return node.stateSync
}

// Beaconchain returns the beacon chain from node.
func (node *Node) Beaconchain() core.BlockChain {
	// tikv mode not have the BeaconChain storage
	if node.HarmonyConfig != nil && node.HarmonyConfig.General.RunElasticMode && node.HarmonyConfig.General.ShardID != shard.BeaconChainShardID {
		return nil
	}

	return node.chain(shard.BeaconChainShardID, core.Options{})
}

func (node *Node) chain(shardID uint32, options core.Options) core.BlockChain {
	bc, err := node.shardChains.ShardChain(shardID, options)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("cannot get beaconchain")
	}
	// only available in validator node and shard 1-3
	isEnablePruneBeaconChain := node.HarmonyConfig != nil && node.HarmonyConfig.General.EnablePruneBeaconChain
	isNotBeaconChainValidator := node.NodeConfig.Role() == nodeconfig.Validator && node.NodeConfig.ShardID != shard.BeaconChainShardID
	if isEnablePruneBeaconChain && isNotBeaconChainValidator {
		bc.EnablePruneBeaconChainFeature()
	} else if isEnablePruneBeaconChain && !isNotBeaconChainValidator {
		utils.Logger().Info().Msg("`IsEnablePruneBeaconChain` only available in validator node and shard 1-3")
	}
	return bc
}

// EpochChain returns the epoch chain from node. Epoch chain is the same as BeaconChain,
// but with differences in behaviour.
func (node *Node) EpochChain() core.BlockChain {
	return node.chain(shard.BeaconChainShardID, core.Options{
		EpochChain: true,
	})
}

// TODO: make this batch more transactions
func (node *Node) tryBroadcast(tx *types.Transaction) {
	msg := proto_node.ConstructTransactionListMessageAccount(types.Transactions{tx})

	shardGroupID := nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(tx.ShardID()))
	utils.Logger().Info().Str("shardGroupID", string(shardGroupID)).Msg("tryBroadcast")

	for attempt := 0; attempt < NumTryBroadCast; attempt++ {
		err := node.host.SendMessageToGroups([]nodeconfig.GroupID{shardGroupID}, p2p.ConstructMessage(msg))
		if err != nil {
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
			p2p.ConstructMessage(msg)); err != nil {
			utils.Logger().Error().Int("attempt", attempt).Msg("Error when trying to broadcast staking tx")
		} else {
			break
		}
	}
}

// Add new transactions to the pending transaction list.
func addPendingTransactions(registry *registry.Registry, newTxs types.Transactions) []error {
	var (
		errs     []error
		bc       = registry.GetBlockchain()
		txPool   = registry.GetTxPool()
		poolTxs  = types.PoolTransactions{}
		acceptCx = bc.Config().AcceptsCrossTx(bc.CurrentHeader().Epoch())
	)
	for _, tx := range newTxs {
		if tx.ShardID() != tx.ToShardID() && !acceptCx {
			errs = append(errs, errors.WithMessage(errInvalidEpoch, "cross-shard tx not accepted yet"))
			continue
		}
		if tx.IsEthCompatible() && !bc.Config().IsEthCompatible(bc.CurrentBlock().Epoch()) {
			errs = append(errs, errors.WithMessage(errInvalidEpoch, "ethereum tx not accepted yet"))
			continue
		}
		poolTxs = append(poolTxs, tx)
	}
	errs = append(errs, registry.GetTxPool().AddRemotes(poolTxs)...)
	pendingCount, queueCount := txPool.Stats()
	utils.Logger().Debug().
		Interface("err", errs).
		Int("length of newTxs", len(newTxs)).
		Int("totalPending", pendingCount).
		Int("totalQueued", queueCount).
		Msg("[addPendingTransactions] Adding more transactions")
	return errs
}

// Add new staking transactions to the pending staking transaction list.
func (node *Node) addPendingStakingTransactions(newStakingTxs staking.StakingTransactions) []error {
	if node.IsRunningBeaconChain() {
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
	if node.IsRunningBeaconChain() {
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
		errs := addPendingTransactions(node.registry, types.Transactions{newTx})
		var err error
		for i := range errs {
			if errs[i] != nil {
				utils.Logger().Info().Err(errs[i]).Msg("[AddPendingTransaction] Failed adding new transaction")
				err = errs[i]
				break
			}
		}
		if err == nil || node.BroadcastInvalidTx {
			utils.Logger().Info().Str("Hash", newTx.Hash().Hex()).Str("HashByType", newTx.HashByType().Hex()).Msg("Broadcasting Tx")
			node.tryBroadcast(newTx)
		}
		return err
	}
	return errors.Errorf("shard do not match, txShard: %d, nodeShard: %d", newTx.ShardID(), node.NodeConfig.ShardID)
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
	errViewIDTooOld      = errors.New("view id too old")
	errWrongSizeOfBitmap = errors.New("wrong size of sender bitmap")
	errWrongShardID      = errors.New("wrong shard id")
	errInvalidNodeMsg    = errors.New("invalid node message")
	errIgnoreBeaconMsg   = errors.New("ignore beacon sync block")
	errInvalidEpoch      = errors.New("invalid epoch for transaction")
	errInvalidShard      = errors.New("invalid shard")
)

const beaconBlockHeightTolerance = 2

// validateNodeMessage validate node message
func (node *Node) validateNodeMessage(ctx context.Context, payload []byte) (
	[]byte, proto_node.MessageType, error) {

	// length of payload must > p2pNodeMsgPrefixSize

	// reject huge node messages
	if len(payload) >= types.MaxP2PNodeDataSize {
		nodeNodeMessageCounterVec.With(prometheus.Labels{"type": "invalid_oversized"}).Inc()
		return nil, 0, core.ErrOversizedData
	}

	// just ignore payload[0], which is MsgCategoryType (consensus/node)
	msgType := proto_node.MessageType(payload[proto.MessageCategoryBytes])

	switch msgType {
	case proto_node.Transaction:
		// nothing much to validate transaction message unless decode the RLP
		nodeNodeMessageCounterVec.With(prometheus.Labels{"type": "tx"}).Inc()
	case proto_node.Staking:
		// nothing much to validate staking message unless decode the RLP
		nodeNodeMessageCounterVec.With(prometheus.Labels{"type": "staking_tx"}).Inc()
	case proto_node.Block:
		switch proto_node.BlockMessageType(payload[p2pNodeMsgPrefixSize]) {
		case proto_node.Sync:
			nodeNodeMessageCounterVec.With(prometheus.Labels{"type": "block_sync"}).Inc()

			// in tikv mode, not need BeaconChain message
			if node.HarmonyConfig.General.RunElasticMode && node.HarmonyConfig.General.ShardID != shard.BeaconChainShardID {
				return nil, 0, errIgnoreBeaconMsg
			}

			// checks whether the beacon block is larger than current block number
			blocksPayload := payload[p2pNodeMsgPrefixSize+1:]
			var blocks []*types.Block
			if err := rlp.DecodeBytes(blocksPayload, &blocks); err != nil {
				return nil, 0, errors.Wrap(err, "block decode error")
			}
			curBeaconHeight := node.Beaconchain().CurrentBlock().NumberU64()
			for _, block := range blocks {
				// Ban blocks number that is smaller than tolerance
				if block.NumberU64()+beaconBlockHeightTolerance <= curBeaconHeight {
					utils.Logger().Debug().Uint64("receivedNum", block.NumberU64()).
						Uint64("currentNum", curBeaconHeight).Msg("beacon block sync message rejected")
					return nil, 0, errors.New("beacon block height smaller than current height beyond tolerance")
				} else if block.NumberU64()-beaconBlockHeightTolerance > curBeaconHeight {
					utils.Logger().Debug().Uint64("receivedNum", block.NumberU64()).
						Uint64("currentNum", curBeaconHeight).Msg("beacon block sync message rejected")
					return nil, 0, errors.New("beacon block height too much higher than current height beyond tolerance")
				} else if block.NumberU64() <= curBeaconHeight {
					utils.Logger().Debug().Uint64("receivedNum", block.NumberU64()).
						Uint64("currentNum", curBeaconHeight).Msg("beacon block sync message ignored")
					return nil, 0, errIgnoreBeaconMsg
				}
			}

			// only non-beacon nodes process the beacon block sync messages
			if node.Blockchain().ShardID() == shard.BeaconChainShardID {
				return nil, 0, errIgnoreBeaconMsg
			}

		case proto_node.SlashCandidate:
			nodeNodeMessageCounterVec.With(prometheus.Labels{"type": "slash"}).Inc()
			// only beacon chain node process slash candidate messages
			if !node.IsRunningBeaconChain() {
				return nil, 0, errIgnoreBeaconMsg
			}
		case proto_node.Receipt:
			nodeNodeMessageCounterVec.With(prometheus.Labels{"type": "node_receipt"}).Inc()
		case proto_node.CrossLink:
			nodeNodeMessageCounterVec.With(prometheus.Labels{"type": "crosslink"}).Inc()
			if node.NodeConfig.Role() == nodeconfig.ExplorerNode {
				return nil, 0, errIgnoreBeaconMsg
			}
		case proto_node.CrosslinkHeartbeat:
			nodeNodeMessageCounterVec.With(prometheus.Labels{"type": "crosslink_heartbeat"}).Inc()
			// only non beacon chain processes cross link heartbeat
			if node.IsRunningBeaconChain() {
				return nil, 0, errInvalidShard
			}
		default:
			nodeNodeMessageCounterVec.With(prometheus.Labels{"type": "invalid_block_type"}).Inc()
			return nil, 0, errInvalidNodeMsg
		}
	default:
		nodeNodeMessageCounterVec.With(prometheus.Labels{"type": "invalid_node_type"}).Inc()
		return nil, 0, errInvalidNodeMsg
	}

	return payload[p2pNodeMsgPrefixSize:], msgType, nil
}

// validateShardBoundMessage validate consensus message
// validate shardID
// validate public key size
// verify message signature
func validateShardBoundMessage(consensus *consensus.Consensus, nodeConfig *nodeconfig.ConfigType, payload []byte,
) (*msg_pb.Message, *bls.SerializedPublicKey, bool, error) {
	var (
		m msg_pb.Message
		//consensus = registry.GetConsensus()
	)
	if err := protobuf.Unmarshal(payload, &m); err != nil {
		nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_unmarshal"}).Inc()
		return nil, nil, true, errors.WithStack(err)
	}

	// ignore messages not intended for explorer
	if nodeConfig.Role() == nodeconfig.ExplorerNode {
		switch m.Type {
		case
			msg_pb.MessageType_ANNOUNCE,
			msg_pb.MessageType_PREPARE,
			msg_pb.MessageType_COMMIT,
			msg_pb.MessageType_VIEWCHANGE,
			msg_pb.MessageType_NEWVIEW:
			nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
			return nil, nil, true, nil
		}
	}

	// when node is in ViewChanging mode, it still accepts normal messages into FBFTLog
	// in order to avoid possible trap forever but drop PREPARE and COMMIT
	// which are message types specifically for a node acting as leader
	// so we just ignore those messages
	if consensus.IsViewChangingMode() {
		switch m.Type {
		case msg_pb.MessageType_PREPARE, msg_pb.MessageType_COMMIT:
			nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
			return nil, nil, true, nil
		}
	} else {
		// ignore viewchange/newview message if the node is not in viewchanging mode
		switch m.Type {
		case msg_pb.MessageType_NEWVIEW, msg_pb.MessageType_VIEWCHANGE:
			nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
			return nil, nil, true, nil
		}
	}

	// ignore message not intended for leader, but still forward them to the network
	if consensus.IsLeader() {
		switch m.Type {
		case msg_pb.MessageType_ANNOUNCE, msg_pb.MessageType_PREPARED, msg_pb.MessageType_COMMITTED:
			nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
			return nil, nil, true, nil
		}
	}

	maybeCon, maybeVC := m.GetConsensus(), m.GetViewchange()
	senderKey := []byte{}
	senderBitmap := []byte{}

	if maybeCon != nil {
		if maybeCon.ShardId != consensus.ShardID {
			nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_shard"}).Inc()
			return nil, nil, true, errors.WithStack(errWrongShardID)
		}
		senderKey = maybeCon.SenderPubkey

		if len(maybeCon.SenderPubkeyBitmap) > 0 {
			senderBitmap = maybeCon.SenderPubkeyBitmap
		}
		// If the viewID is too old, reject the message.
		if maybeCon.ViewId+5 < consensus.GetCurBlockViewID() {
			return nil, nil, true, errors.WithStack(errViewIDTooOld)
		}
	} else if maybeVC != nil {
		if maybeVC.ShardId != consensus.ShardID {
			nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_shard"}).Inc()
			return nil, nil, true, errors.WithStack(errWrongShardID)
		}
		senderKey = maybeVC.SenderPubkey
		// If the viewID is too old, reject the message.
		if maybeVC.ViewId+5 < consensus.GetViewChangingID() {
			return nil, nil, true, errors.WithStack(errViewIDTooOld)
		}
	} else {
		nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid"}).Inc()
		return nil, nil, true, errors.WithStack(errNoSenderPubKey)
	}

	// ignore mesage not intended for validator
	// but still forward them to the network
	if !consensus.IsLeader() {
		switch m.Type {
		case msg_pb.MessageType_PREPARE, msg_pb.MessageType_COMMIT:
			nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
			return nil, nil, true, nil
		}
	}

	serializedKey := bls.SerializedPublicKey{}
	if len(senderKey) > 0 {
		if len(senderKey) != bls.PublicKeySizeInBytes {
			nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_key_size"}).Inc()
			return nil, nil, true, errors.WithStack(errNotRightKeySize)
		}

		copy(serializedKey[:], senderKey)
		if !consensus.IsValidatorInCommittee(serializedKey) {
			nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_committee"}).Inc()
			return nil, nil, true, errors.WithStack(shard.ErrValidNotInCommittee)
		}
	} else {
		count := consensus.Decider.ParticipantsCount()
		if (count+7)>>3 != int64(len(senderBitmap)) {
			nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "invalid_participant_count"}).Inc()
			return nil, nil, true, errors.WithStack(errWrongSizeOfBitmap)
		}
	}

	nodeConsensusMessageCounterVec.With(prometheus.Labels{"type": "valid"}).Inc()

	// serializedKey will be empty for multiSig sender
	return &m, &serializedKey, false, nil
}

var (
	errMsgHadNoHMYPayLoadAssumption      = errors.New("did not have sufficient size for hmy msg")
	errConsensusMessageOnUnexpectedTopic = errors.New("received consensus on wrong topic")
)

// StartPubSub kicks off the node message handling
func (node *Node) StartPubSub() error {
	node.psCtx, node.psCancel = context.WithCancel(context.Background())

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
					NamedTopic:     p2p.NamedTopic{Name: string(key), Topic: topicHandle},
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
	nodeStringCounterVec.WithLabelValues("peerid", nodeconfig.GetPeerID().String()).Inc()

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
				nodeP2PMessageCounterVec.With(prometheus.Labels{"type": "total"}).Inc()
				hmyMsg := msg.GetData()

				// first to validate the size of the p2p message
				if len(hmyMsg) < p2pMsgPrefixSize {
					// TODO (lc): block peers sending empty messages
					nodeP2PMessageCounterVec.With(prometheus.Labels{"type": "invalid_size"}).Inc()
					return libp2p_pubsub.ValidationReject
				}

				openBox := hmyMsg[p2pMsgPrefixSize:]

				// validate message category
				switch proto.MessageCategory(openBox[proto.MessageCategoryBytes-1]) {
				case proto.Consensus:
					// received consensus message in non-consensus bound topic
					if !isConsensusBound {
						nodeP2PMessageCounterVec.With(prometheus.Labels{"type": "invalid_bound"}).Inc()
						errChan <- withError{
							errors.WithStack(errConsensusMessageOnUnexpectedTopic), msg,
						}
						return libp2p_pubsub.ValidationReject
					}
					nodeP2PMessageCounterVec.With(prometheus.Labels{"type": "consensus_total"}).Inc()

					// validate consensus message
					validMsg, senderPubKey, ignore, err := validateShardBoundMessage(
						node.Consensus, node.NodeConfig, openBox[proto.MessageCategoryBytes:],
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
						nodeP2PMessageCounterVec.With(prometheus.Labels{"type": "invalid_size"}).Inc()
						return libp2p_pubsub.ValidationReject
					}
					nodeP2PMessageCounterVec.With(prometheus.Labels{"type": "node_total"}).Inc()
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
					nodeP2PMessageCounterVec.With(prometheus.Labels{"type": "ignored"}).Inc()
					return libp2p_pubsub.ValidationReject
				}
			},
			// WithValidatorTimeout is an option that sets a timeout for an (asynchronous) topic validator. By default there is no timeout in asynchronous validators.
			// TODO: Currently this timeout is useless. Verify me.
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
			for {
				select {
				case <-node.psCtx.Done():
					return
				case m := <-msgChanConsensus:
					// should not take more than 30 seconds to process one message
					ctx, cancel := context.WithTimeout(node.psCtx, 30*time.Second)
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
						// FIXME: wrong use of context. This message have already passed handle actually.
						case <-ctx.Done():
							if errors.Is(ctx.Err(), context.DeadlineExceeded) ||
								errors.Is(ctx.Err(), context.Canceled) {
								utils.Logger().Warn().
									Str("topic", topicNamed).Msg("[context] exceeded consensus message handler deadline")
							}
							errChan <- withError{errors.WithStack(ctx.Err()), nil}
						default:
							return
						}
					}()
				}
			}
		}()

		semNode := semaphore.NewWeighted(p2p.SetAsideOtherwise)
		msgChanNode := make(chan validated, MsgChanBuffer)

		// goroutine to handle node messages
		go func() {
			for {
				select {
				case m := <-msgChanNode:
					ctx, cancel := context.WithTimeout(node.psCtx, 10*time.Second)
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
							if errors.Is(ctx.Err(), context.DeadlineExceeded) ||
								errors.Is(ctx.Err(), context.Canceled) {
								utils.Logger().Warn().
									Str("topic", topicNamed).Msg("[context] exceeded node message handler deadline")
							}
							errChan <- withError{errors.WithStack(ctx.Err()), nil}
						default:
							return
						}
					}()
				case <-node.psCtx.Done():
					return
				}
			}
		}()

		go func() {
			for {
				nextMsg, err := sub.Next(node.psCtx)
				if err != nil {
					if err == context.Canceled {
						return
					}
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

	go func() {
		for {
			select {
			case <-node.psCtx.Done():
				return
			case e := <-errChan:
				utils.SampledLogger().Info().
					Interface("item", e.payload).
					Msgf("[p2p]: issue while handling incoming p2p message: %v", e.err)
			}
		}
	}()

	node.TraceLoopForExplorer()
	return nil
}

// StopPubSub stops the pubsub handling
func (node *Node) StopPubSub() {
	if node.psCancel != nil {
		node.psCancel()
	}
}

// GetSyncID returns the syncID of this node
func (node *Node) GetSyncID() [SyncIDLength]byte {
	return node.syncID
}

// New creates a new node.
func New(
	host p2p.Host,
	consensusObj *consensus.Consensus,
	engine engine.Engine,
	collection *shardchain.CollectionImpl,
	blacklist map[common.Address]struct{},
	allowedTxs map[common.Address]core.AllowedTxData,
	localAccounts []common.Address,
	isArchival map[uint32]bool,
	harmonyconfig *harmonyconfig.HarmonyConfig,
	registry *registry.Registry,
) *Node {
	node := Node{
		registry:             registry,
		unixTimeAtNodeStart:  time.Now().Unix(),
		TransactionErrorSink: types.NewTransactionErrorSink(),
		crosslinks:           crosslinks.New(),
		syncID:               GenerateSyncID(),
		keysToAddrs:          lrucache.NewCache[uint64, map[string]common.Address](10),
	}
	if consensusObj == nil {
		panic("consensusObj is nil")
	}
	// Get the node config that's created in the harmony.go program.
	node.NodeConfig = nodeconfig.GetShardConfig(consensusObj.ShardID)
	node.HarmonyConfig = harmonyconfig

	if host != nil {
		node.host = host
		node.SelfPeer = host.GetSelfPeer()
	}

	networkType := node.NodeConfig.GetNetworkType()
	chainConfig := networkType.ChainConfig()
	node.chainConfig = chainConfig
	node.shardChains = collection
	node.IsSynchronized = abool.NewBool(false)

	if host != nil {
		// Consensus and associated channel to communicate blocks
		node.Consensus = consensusObj

		// Load the chains.
		blockchain := node.Blockchain() // this also sets node.isFirstTime if the DB is fresh
		var beaconChain core.BlockChain
		if blockchain.ShardID() == shard.BeaconChainShardID {
			beaconChain = node.Beaconchain()
		} else {
			beaconChain = node.EpochChain()
		}

		if b1, b2 := beaconChain == nil, blockchain == nil; b1 || b2 {
			// in tikv mode, not need BeaconChain
			if !(node.HarmonyConfig != nil && node.HarmonyConfig.General.RunElasticMode) || node.HarmonyConfig.General.ShardID == shard.BeaconChainShardID {
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
		}

		node.BeaconBlockChannel = make(chan *types.Block)
		txPoolConfig := core.DefaultTxPoolConfig

		if harmonyconfig != nil {
			txPoolConfig.AccountSlots = harmonyconfig.TxPool.AccountSlots
			txPoolConfig.GlobalSlots = harmonyconfig.TxPool.GlobalSlots
			txPoolConfig.Locals = append(txPoolConfig.Locals, localAccounts...)
			txPoolConfig.AccountQueue = harmonyconfig.TxPool.AccountQueue
			txPoolConfig.GlobalQueue = harmonyconfig.TxPool.GlobalQueue
			txPoolConfig.Lifetime = harmonyconfig.TxPool.Lifetime
			txPoolConfig.PriceLimit = uint64(harmonyconfig.TxPool.PriceLimit)
			txPoolConfig.PriceBump = harmonyconfig.TxPool.PriceBump
		}
		// Temporarily not updating other networks to make the rpc tests pass
		if node.NodeConfig.GetNetworkType() != nodeconfig.Mainnet && node.NodeConfig.GetNetworkType() != nodeconfig.Testnet {
			txPoolConfig.PriceLimit = 1e9
			txPoolConfig.PriceBump = 10
		}

		txPoolConfig.Blacklist = blacklist
		txPoolConfig.AllowedTxs = allowedTxs
		txPoolConfig.Journal = fmt.Sprintf("%v/%v", node.NodeConfig.DBDir, txPoolConfig.Journal)
		txPoolConfig.AddEvent = func(tx types.PoolTransaction, local bool) {
			// in tikv mode, writer will publish tx pool update to all reader
			if node.Blockchain().IsTikvWriterMaster() {
				err := redis_helper.PublishTxPoolUpdate(uint32(harmonyconfig.General.ShardID), tx, local)
				if err != nil {
					utils.Logger().Warn().Err(err).Msg("redis publish txpool update error")
				}
			}
		}

		node.TxPool = core.NewTxPool(txPoolConfig, node.Blockchain().Config(), blockchain, node.TransactionErrorSink)
		node.registry.SetTxPool(node.TxPool)
		node.CxPool = node.registry.GetCxPool()
		node.Worker = worker.New(node.Blockchain().Config(), blockchain, beaconChain, engine)

		node.deciderCache, _ = lru.New(16)
		node.committeeCache, _ = lru.New(16)

		node.pendingCXReceipts = map[string]*types.CXReceiptsProof{}
		node.Consensus.VerifiedNewBlock = make(chan *types.Block, 1)
		// the sequence number is the next block number to be added in consensus protocol, which is
		// always one more than current chain header block
		node.Consensus.SetBlockNum(blockchain.CurrentBlock().NumberU64() + 1)
	}

	utils.Logger().Info().
		Interface("genesis block header", node.Blockchain().GetHeaderByNumber(0)).
		Msg("Genesis block hash")
	// Setup initial state of syncing.
	node.peerRegistrationRecord = map[string]*syncConfig{}
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
				if !node.IsRunningBeaconChain() {
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

	// in tikv mode, not need BeaconChain
	if !(node.HarmonyConfig != nil && node.HarmonyConfig.General.RunElasticMode) || node.HarmonyConfig.General.ShardID == shard.BeaconChainShardID {
		// update reward values now that node is ready
		node.updateInitialRewardValues()
	}

	// init metrics
	initMetrics()
	nodeStringCounterVec.WithLabelValues("version", nodeconfig.GetVersion()).Inc()

	node.serviceManager = service.NewManager()

	return &node
}

// updateInitialRewardValues using the node data
func (node *Node) updateInitialRewardValues() {
	numShards := shard.Schedule.InstanceForEpoch(node.Beaconchain().CurrentHeader().Epoch()).NumShards()
	initTotal := big.NewInt(0)
	for i := uint32(0); i < numShards; i++ {
		initTotal = new(big.Int).Add(core.GetInitialFunds(i), initTotal)
	}
	reward.SetTotalInitialTokens(initTotal)
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
	currentBlock := node.Blockchain().CurrentBlock()
	blockNum := currentBlock.NumberU64()
	node.Consensus.SetMode(consensus.Listening)
	epoch := currentBlock.Epoch()
	utils.Logger().Info().
		Uint64("blockNum", blockNum).
		Uint32("shardID", shardID).
		Uint64("epoch", epoch.Uint64()).
		Msg("[InitConsensusWithValidators] Try To Get PublicKeys")
	shardState, err := committee.WithStakingEnabled.Compute(
		epoch, node.Consensus.Blockchain(),
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
		utils.Logger().Err(err).
			Interface("shardState", shardState).
			Msg("[InitConsensusWithValidators] Find CommitteeByID")
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
				Str("mode", node.Consensus.Mode().String()).
				Msg("[InitConsensusWithValidators] Successfully updated public keys")
			node.Consensus.UpdatePublicKeys(pubKeys, shard.Schedule.InstanceForEpoch(epoch).ExternalAllowlist())
			node.Consensus.SetMode(consensus.Normal)
			return nil
		}
	}
	return nil
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
	if err := node.StopRPC(); err != nil {
		utils.Logger().Error().Err(err).Msg("failed to stop RPC")
	}

	utils.Logger().Info().Msg("stopping rosetta")
	if err := node.StopRosetta(); err != nil {
		utils.Logger().Error().Err(err).Msg("failed to stop rosetta")
	}

	utils.Logger().Info().Msg("stopping services")
	if err := node.StopServices(); err != nil {
		utils.Logger().Error().Err(err).Msg("failed to stop services")
	}

	// Currently pubSub need to be stopped after consensus.
	utils.Logger().Info().Msg("stopping pub-sub")
	node.StopPubSub()

	utils.Logger().Info().Msg("stopping host")
	if err := node.host.Close(); err != nil {
		utils.Logger().Error().Err(err).Msg("failed to stop p2p host")
	}

	node.Blockchain().Stop()
	node.Beaconchain().Stop()

	if node.HarmonyConfig.General.RunElasticMode {
		_, _ = node.Blockchain().RedisPreempt().Unlock()
		_, _ = node.Beaconchain().RedisPreempt().Unlock()

		_ = redis_helper.Close()
		time.Sleep(time.Second)

		if storage := tikv_manage.GetDefaultTiKVFactory(); storage != nil {
			storage.CloseAllDB()
		}
	}

	const msg = "Successfully shut down!\n"
	utils.Logger().Print(msg)
	fmt.Print(msg)
	os.Exit(0)
}

func (node *Node) populateSelfAddresses(epoch *big.Int) {
	shardID := node.Consensus.ShardID
	shardState, err := node.Consensus.Blockchain().ReadShardState(epoch)
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
	keysToAddrs := map[string]common.Address{}
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
		keysToAddrs[blsStr] = *addr
		utils.Logger().Debug().
			Int64("epoch", epoch.Int64()).
			Uint32("shard-id", shardID).
			Str("bls-key", blsStr).
			Str("address", common2.MustAddressToBech32(*addr)).
			Msg("[PopulateSelfAddresses]")
	}
	node.keysToAddrs.Set(epoch.Uint64(), keysToAddrs)
}

// GetAddressForBLSKey retrieves the ECDSA address associated with bls key for epoch
func (node *Node) GetAddressForBLSKey(blskey *bls_core.PublicKey, epoch *big.Int) common.Address {
	return node.GetAddresses(epoch)[blskey.SerializeToHexStr()]
}

// GetAddresses retrieves all ECDSA addresses of the bls keys for epoch
func (node *Node) GetAddresses(epoch *big.Int) map[string]common.Address {
	// populate if new epoch
	if rs, ok := node.keysToAddrs.Get(epoch.Uint64()); ok {
		return rs
	}
	node.keysToAddrsMutex.Lock()
	node.populateSelfAddresses(epoch)
	node.keysToAddrsMutex.Unlock()
	if rs, ok := node.keysToAddrs.Get(epoch.Uint64()); ok {
		return rs
	}
	return make(map[string]common.Address)
}

// IsRunningBeaconChain returns whether the node is running on beacon chain.
func (node *Node) IsRunningBeaconChain() bool {
	return node.NodeConfig.ShardID == shard.BeaconChainShardID
}

// syncFromTiKVWriter used for tikv mode, subscribe data from tikv writer
func (node *Node) syncFromTiKVWriter() {
	bc := node.Blockchain()

	// subscribe block update
	go redis_helper.SubscribeShardUpdate(bc.ShardID(), func(blkNum uint64, logs []*types.Log) {
		// todo temp code to find and fix problems
		// redis: 2022/07/09 03:51:23 pubsub.go:605: redis: &{%!s(*redis.PubSub=&{0xc002198d20 0xe0b160 0xe0b380 {0 0} 0xc0454265a0 map[shard_update_0:{}] map[] false 0xc0047c7800 0xc004946240 {1 {0 0}} 0xc0074269c0 <nil>}) %!s(chan *redis.Message=0xc004946180) %!s(chan interface {}=<nil>) %!s(chan struct {}=0xc0047c7b60) %!s(int=100) %!s(time.Duration=60000000000) %!s(time.Duration=3000000000)} channel is full for 1m0s (message is dropped)
		doneChan := make(chan struct{}, 1)
		go func() {
			select {
			case <-doneChan:
				return
			case <-time.After(2 * time.Minute):
				buf := bytes.NewBuffer(nil)
				err := pprof.Lookup("goroutine").WriteTo(buf, 1)
				if err != nil {
					panic(err)
				}
				err = ioutil.WriteFile(fmt.Sprintf("/local/%s", time.Now().Format("hmy_0102150405.error.log")), buf.Bytes(), 0644)
				if err != nil {
					panic(err)
				}
				// todo temp code to fix problems, restart self
				os.Exit(1)
			}
		}()
		defer close(doneChan)

		if err := bc.SyncFromTiKVWriter(blkNum, logs); err != nil {
			utils.Logger().Warn().
				Err(err).
				Msg("cannot sync block from tikv writer")
			return
		}
	})

	// subscribe txpool update
	if node.HarmonyConfig.TiKV.Role == tikv.RoleReader {
		go redis_helper.SubscribeTxPoolUpdate(bc.ShardID(), func(tx types.PoolTransaction, local bool) {
			var err error
			if local {
				err = node.TxPool.AddLocal(tx)
			} else {
				err = node.TxPool.AddRemote(tx)
			}
			if err != nil {
				utils.Logger().Debug().
					Err(err).
					Interface("tx", tx).
					Msg("cannot sync txpool from tikv writer")
				return
			}
		})
	}
}
