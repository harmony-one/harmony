package node

import (
	"bufio"
	"bytes"
	"container/ring"
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
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
	"github.com/harmony-one/harmony/node/relay"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	staking "github.com/harmony-one/harmony/staking/types"
	ipfs_interface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// State ..
type State = string

// All constants except the NodeLeader below are for validators only.
const (
	WaitToJoin        State = "wait-to-join"        // Node contacted BeaconChain, wait to join Shard
	NotInSync         State = "not-in-sync"         // Node out of sync, might be just joined Shard or offline for a period of time
	ReadyForConsensus State = "ready-for-consensus" // Node is ready for doing consensus
)

const (
	// NumTryBroadCast is the number of times trying to broadcast
	NumTryBroadCast = 3
	//SyncIDLength is the length of bytes for syncID
	SyncIDLength = 20
)

// Node represents a protocol-participating node in the network
type Node struct {
	Consensus          *consensus.Consensus
	BeaconBlockChannel chan *types.Block // The channel to send beacon blocks for non-beaconchain nodes
	IncomingBlocks     chan *types.Block
	Gossiper           relay.BroadCaster
	// All the receipts received but not yet processed for Consensus
	pendingCXReceipts map[string]*types.CXReceiptsProof
	pendingCXMutex    sync.Mutex
	// Shard databases
	shardChains          shardchain.Collection
	Peer                 *p2p.Peer
	State                atomic.Value // State of the Node
	TxPool               *core.TxPool
	CxPool               *core.CxPool // pool for missing cross shard receipts resend
	Worker, BeaconWorker *worker.Worker
	// The p2p host used to send/receive p2p messages
	host *p2p.Host
	// Service manager.
	ContractDeployerKey          *ecdsa.PrivateKey
	ContractDeployerCurrentNonce uint64 // The nonce of the deployer contract at current block
	ContractAddresses            []common.Address
	// node configuration, including group ID, shard ID, etc
	NodeConfig *nodeconfig.ConfigType
	// Chain configuration.
	chainConfig params.ChainConfig
	// map of service type to its message channel.
	isFirstTime bool // the node was started with a fresh database
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

const maxWaitBootstrap = 90 * time.Second

// BootstrapConsensus is the a goroutine to check number of peers and start the consensus
func (node *Node) BootstrapConsensus() error {
	haveEnoughPeers := make(chan struct{})
	errored := make(chan error)
	t := time.NewTimer(maxWaitBootstrap)
	defer t.Stop()

	go func() {
		min := node.Consensus.MinPeers

		for {
			current := 0
			time.Sleep(2 * time.Second)
			conns, err := node.host.CoreAPI.Swarm().Peers(context.Background())

			if err != nil {
				errored <- err
				return
			}

			for _, conn := range conns {
				protocols, err := node.host.IPFSNode.PeerHost.Peerstore().SupportsProtocols(
					conn.ID(), p2p.Protocol,
				)

				if err != nil {
					errored <- err
					return
				}

				for _, protocol := range protocols {
					if protocol == p2p.Protocol {
						utils.Logger().Info().
							Str("peer-id", conn.ID().Pretty()).
							Msg("peer supports hmy protocol")

						current++
						break
					}
				}

				if current >= min-2 {
					utils.Logger().Info().
						Int("have", current).
						Int("needed", min).
						Msg("got enough peers for consensus")
					haveEnoughPeers <- struct{}{}
					return
				}
			}

		}
	}()

	select {
	case err := <-errored:
		return err
	case <-haveEnoughPeers:
		go func() {
			node.Consensus.ProposalNewBlock <- struct{}{}
		}()

		node.Consensus.SetNextBlockDue(time.Now().Add(consensus.BlockTime))

		utils.Logger().Info().
			Time("next-block-due", node.Consensus.NextBlockDue()).
			Msg("this node is leader, kicked off consensus")
		return nil
	case <-t.C:
		return errors.New("exceeded 60 seconds waiting for enough min peers")
	}
}

// StartP2PMessageHandling kicks off the node message handling
func (node *Node) StartP2PMessageHandling() error {
	allTopics := node.host.AllSubscriptions()
	if len(allTopics) == 0 {
		return errors.New("have no topics to listen to")
	}
	weighted := make([]*semaphore.Weighted, len(allTopics))
	const maxMessageHandlers = 200
	ctx := context.Background()
	ownID := node.host.IPFSNode.PeerHost.ID()
	errChan := make(chan error)

	for i, named := range allTopics {
		topicName, sub := named.Topic, named.Sub

		// if node.Consensus.ShardID == 1 {
		// 	fmt.Println("listneing on topic named", topicName)
		// }

		weighted[i] = semaphore.NewWeighted(maxMessageHandlers)
		msgChan := make(chan ipfs_interface.PubSubMessage)

		go func(msgChan chan ipfs_interface.PubSubMessage, sem *semaphore.Weighted) {
			for msg := range msgChan {
				payload := msg.Data()
				if len(payload) < p2pMsgPrefixSize {
					continue
				}
				m := msg
				if sem.TryAcquire(1) {
					go func() {
						node.HandleMessage(
							payload[p2pMsgPrefixSize:], m.From(), topicName,
						)
						sem.Release(1)
					}()
				}
			}
		}(msgChan, weighted[i])

		go func(msgChan chan ipfs_interface.PubSubMessage) {
			for {
				nextMsg, err := sub.Next(ctx)
				if err != nil {
					errChan <- err
					continue
				}
				if nextMsg.From() == ownID {
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

const sinkSize = 4096

// New creates a new node.
func New(
	host *p2p.Host,
	consensusObj *consensus.Consensus,
	chainDBFactory shardchain.DBFactory,
	blacklist map[common.Address]struct{},
	isArchival bool,
	configUsed *nodeconfig.ConfigType,
) (*Node, error) {

	var state atomic.Value
	state.Store(WaitToJoin)

	networkType := configUsed.GetNetworkType()
	chainConfig := nodeconfig.ChainConfig(networkType)

	node := &Node{
		host:                host,
		Consensus:           consensusObj,
		Gossiper:            relay.NewBroadCaster(configUsed, host),
		NodeConfig:          configUsed,
		chainConfig:         chainConfig,
		Peer:                host.OwnPeer,
		unixTimeAtNodeStart: time.Now().Unix(),
		CxPool:              core.NewCxPool(core.CxPoolSize),
		pendingCXReceipts:   map[string]*types.CXReceiptsProof{},
		BeaconBlockChannel:  make(chan *types.Block),
		IncomingBlocks:      make(chan *types.Block),
		State:               state,
		errorSink: struct {
			sync.Mutex
			failedStakingTxns *ring.Ring
			failedTxns        *ring.Ring
		}{sync.Mutex{}, ring.New(sinkSize), ring.New(sinkSize)},
	}

	collection := shardchain.NewCollection(
		chainDBFactory, &genesisInitializer{node}, chain.Engine, &chainConfig,
	)

	if isArchival {
		collection.DisableCache()
	}
	node.shardChains = collection

	if node.Consensus != nil {
		// Load the chains.
		blockchain := node.Blockchain() // this also sets node.isFirstTime if the DB is fresh
		beaconChain := node.Beaconchain()
		if b1, b2 := beaconChain == nil, blockchain == nil; b1 || b2 {
			shardID := node.NodeConfig.ShardID
			// HACK get the real error reason
			if _, err := node.shardChains.ShardChain(shardID); err != nil {
				return nil, errors.Wrapf(err,
					"beaconchain-is-nil:%t shardchain-is-nil:%t",
					b1, b2,
				)
			}
		}

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
		node.Worker = worker.New(node.Blockchain().Config(), blockchain, chain.Engine)

		if node.Blockchain().ShardID() != shard.BeaconChainShardID {
			node.BeaconWorker = worker.New(
				node.Beaconchain().Config(), beaconChain, chain.Engine,
			)
		}

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

	// TODO mutating these fields after literal construction is code smell
	// TODO Move the IP/ADDr creation to this thing, actually pass the creation
	// of this syncing client, server to the node as input

	// node.stateSync = syncing.CreateStateSync(
	// 	node.SelfPeer.IP, node.SelfPeer.Port, node.SyncID,
	// )

	// node.beaconSync = syncing.CreateStateSync(
	// 	node.SelfPeer.IP, node.SelfPeer.Port, node.SyncID,
	// )

	utils.Logger().Info().
		Interface("genesis block header", node.Blockchain().GetHeaderByNumber(0)).
		Msg("Genesis block hash")
	// Broadcast double-signers reported by consensus
	if node.Consensus != nil {
		go node.handleSlashChan()
	}

	node.NodeConfig.SetBeaconGroupID(
		nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID),
	)

	return node, nil
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

// ForceJoiningTopics ..
func (node *Node) ForceJoiningTopics() error {
	groups := []nodeconfig.GroupID{
		node.NodeConfig.GetShardGroupID(),
		nodeconfig.NewClientGroupIDByShardID(shard.BeaconChainShardID),
	}
	return node.host.SendMessageToGroups(groups, []byte{})
}

// StartStateSyncStreams ..
func (node *Node) StartStateSyncStreams() error {

	var g errgroup.Group

	g.Go(func() error {
		time.Sleep(time.Second * 2)
		conns, err := node.host.CoreAPI.Swarm().Peers(context.TODO())

		if err != nil {
			return err
		}
		var h errgroup.Group

		for _, neighbor := range conns {
			stateSyncStream, err := node.host.IPFSNode.PeerHost.NewStream(
				context.Background(),
				neighbor.ID(),
				p2p.Protocol,
			)

			if err != nil {
				return err
			}

			rw := bufio.NewReadWriter(
				bufio.NewReader(stateSyncStream), bufio.NewWriter(stateSyncStream),
			)

			h.Go(func() error {

				message := &msg_pb.Message{
					ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
					Type:        msg_pb.MessageType_BLOCK_HEIGHT,
				}

				msg, err := protobuf.Marshal(message)

				if err != nil {
					return err
				}

				byteBuffer := bytes.NewBuffer([]byte{byte(proto_node.Client)})
				byteBuffer.Write(msg)
				syncingMessage := p2p.ConstructMessage(byteBuffer.Bytes())

				// fmt.Println(
				// 	"sync message len is ",
				// 	len(syncingMessage),
				// 	syncingMessage,
				// 	hex.EncodeToString(syncingMessage),
				// 	message.String(),
				// 	"just protbuf",
				// 	hex.EncodeToString(msg),
				// )

				if _, err := rw.Write(syncingMessage); err != nil {
					fmt.Println("some problem -> ", err.Error())
					return err
				}

				if err := rw.Flush(); err != nil {
					return err
				}

				return nil
			})
		}

		return h.Wait()

	})

	return g.Wait()
}
