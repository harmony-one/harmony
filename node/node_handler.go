package node

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/consensus"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	pb "github.com/golang/protobuf/proto"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"

	"github.com/harmony-one/harmony/api/proto"
	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	"github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

const (
	// MaxNumberOfTransactionsPerBlock is the max number of transaction per a block.
	MaxNumberOfTransactionsPerBlock = 8000 // Disable tx processing
	consensusTimeout                = 30 * time.Second
)

// ReceiveGlobalMessage use libp2p pubsub mechanism to receive global broadcast messages
func (node *Node) ReceiveGlobalMessage() {
	ctx := context.Background()
	for {
		if node.globalGroupReceiver == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		msg, sender, err := node.globalGroupReceiver.Receive(ctx)
		if sender != node.host.GetID() {
			//utils.GetLogInstance().Info("[PUBSUB]", "received global msg", len(msg), "sender", sender)
			if err == nil {
				// skip the first 5 bytes, 1 byte is p2p type, 4 bytes are message size
				go node.messageHandler(msg[5:], sender)
			}
		}
	}
}

// ReceiveGroupMessage use libp2p pubsub mechanism to receive broadcast messages
func (node *Node) ReceiveGroupMessage() {
	ctx := context.Background()
	for {
		if node.shardGroupReceiver == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		msg, sender, err := node.shardGroupReceiver.Receive(ctx)
		if sender != node.host.GetID() {
			//utils.GetLogInstance().Info("[PUBSUB]", "received group msg", len(msg), "sender", sender)
			if err == nil {
				// skip the first 5 bytes, 1 byte is p2p type, 4 bytes are message size
				go node.messageHandler(msg[5:], sender)
			}
		}
	}
}

// ReceiveClientGroupMessage use libp2p pubsub mechanism to receive broadcast messages for client
func (node *Node) ReceiveClientGroupMessage() {
	ctx := context.Background()
	for {
		if node.clientReceiver == nil {
			// check less frequent on client messages
			time.Sleep(100 * time.Millisecond)
			continue
		}
		msg, sender, err := node.clientReceiver.Receive(ctx)
		if sender != node.host.GetID() {
			// utils.GetLogInstance().Info("[CLIENT]", "received group msg", len(msg), "sender", sender, "error", err)
			if err == nil {
				// skip the first 5 bytes, 1 byte is p2p type, 4 bytes are message size
				go node.messageHandler(msg[5:], sender)
			}
		}
	}
}

// messageHandler parses the message and dispatch the actions
func (node *Node) messageHandler(content []byte, sender libp2p_peer.ID) {
	msgCategory, err := proto.GetMessageCategory(content)
	if err != nil {
		utils.GetLogInstance().Error("Read node type failed", "err", err, "node", node)
		return
	}

	msgType, err := proto.GetMessageType(content)
	if err != nil {
		utils.GetLogInstance().Error("Read action type failed", "err", err, "node", node)
		return
	}

	msgPayload, err := proto.GetMessagePayload(content)
	if err != nil {
		utils.GetLogInstance().Error("Read message payload failed", "err", err, "node", node)
		return
	}

	switch msgCategory {
	case proto.Consensus:
		msgPayload, _ := proto.GetConsensusMessagePayload(content)
		if node.NodeConfig.Role() == nodeconfig.ExplorerNode {
			node.ExplorerMessageHandler(msgPayload)
		} else {
			node.ConsensusMessageHandler(msgPayload)
		}
	case proto.DRand:
		msgPayload, _ := proto.GetDRandMessagePayload(content)
		if node.DRand != nil {
			if node.DRand.IsLeader {
				node.DRand.ProcessMessageLeader(msgPayload)
			} else {
				node.DRand.ProcessMessageValidator(msgPayload)
			}
		}
	case proto.Staking:
		utils.GetLogInstance().Info("NET: Received staking message")
		msgPayload, _ := proto.GetStakingMessagePayload(content)
		// Only beacon leader processes staking txn
		if node.NodeConfig.Role() != nodeconfig.BeaconLeader {
			return
		}
		node.processStakingMessage(msgPayload)
	case proto.Node:
		actionType := proto_node.MessageType(msgType)
		switch actionType {
		case proto_node.Transaction:
			utils.GetLogInstance().Info("NET: received message: Node/Transaction")
			node.transactionMessageHandler(msgPayload)
		case proto_node.Block:
			utils.GetLogInstance().Info("NET: received message: Node/Block")
			blockMsgType := proto_node.BlockMessageType(msgPayload[0])
			switch blockMsgType {
			case proto_node.Sync:
				utils.GetLogInstance().Info("NET: received message: Node/Sync")
				var blocks []*types.Block
				err := rlp.DecodeBytes(msgPayload[1:], &blocks)
				if err != nil {
					utils.GetLogInstance().Error("block sync", "error", err)
				} else {
					// for non-beaconchain node, subscribe to beacon block broadcast
					role := node.NodeConfig.Role()
					if proto_node.BlockMessageType(msgPayload[0]) == proto_node.Sync && (role == nodeconfig.ShardValidator || role == nodeconfig.ShardLeader || role == nodeconfig.NewNode) {
						utils.GetLogInstance().Info("Block being handled by block channel", "self peer", node.SelfPeer, "block", blocks[0].NumberU64())
						for _, block := range blocks {
							node.BeaconBlockChannel <- block
						}
					}
					if node.Client != nil && node.Client.UpdateBlocks != nil && blocks != nil {
						utils.GetLogInstance().Info("Block being handled by client by", "self peer", node.SelfPeer)
						node.Client.UpdateBlocks(blocks)
					}
				}
			}
		case proto_node.PING:
			node.pingMessageHandler(msgPayload, sender)
		case proto_node.PONG:
			node.pongMessageHandler(msgPayload)
		case proto_node.ShardState:
			if err := node.epochShardStateMessageHandler(msgPayload); err != nil {
				ctxerror.Log15(utils.GetLogger().Warn, err)
			}
		}
	default:
		utils.GetLogInstance().Error("Unknown", "MsgCategory", msgCategory)
	}
}

func (node *Node) processStakingMessage(msgPayload []byte) {
	msg := &message.Message{}
	err := pb.Unmarshal(msgPayload, msg)
	if err == nil {
		stakingRequest := msg.GetStaking()
		txs := types.Transactions{}
		if err = rlp.DecodeBytes(stakingRequest.Transaction, &txs); err == nil {
			utils.GetLogInstance().Info("Successfully added staking transaction to pending list.")
			node.addPendingTransactions(txs)
		} else {
			utils.GetLogInstance().Error("Failed to unmarshal staking transaction list", "error", err)
		}
	} else {
		utils.GetLogInstance().Error("Failed to unmarshal staking msg payload", "error", err)
	}
}

func (node *Node) transactionMessageHandler(msgPayload []byte) {
	txMessageType := proto_node.TransactionMessageType(msgPayload[0])

	switch txMessageType {
	case proto_node.Send:
		txs := types.Transactions{}
		err := rlp.Decode(bytes.NewReader(msgPayload[1:]), &txs) // skip the Send messge type
		if err != nil {
			utils.GetLogInstance().Error("Failed to deserialize transaction list", "error", err)
		}
		node.addPendingTransactions(txs)

	case proto_node.Request:
		reader := bytes.NewBuffer(msgPayload[1:])
		txIDs := make(map[[32]byte]bool)
		buf := make([]byte, 32) // 32 byte hash Id
		for {
			_, err := reader.Read(buf)
			if err != nil {
				break
			}

			var txID [32]byte
			copy(txID[:], buf)
			txIDs[txID] = true
		}

		var txToReturn []*types.Transaction
		for _, tx := range node.pendingTransactions {
			if txIDs[tx.Hash()] {
				txToReturn = append(txToReturn, tx)
			}
		}
	}
}

// BroadcastNewBlock is called by consensus leader to sync new blocks with other clients/nodes.
// NOTE: For now, just send to the client (basically not broadcasting)
// TODO (lc): broadcast the new blocks to new nodes doing state sync
func (node *Node) BroadcastNewBlock(newBlock *types.Block) {
	if node.ClientPeer != nil {
		utils.GetLogInstance().Debug("Sending new block to client", "client", node.ClientPeer)
		node.host.SendMessageToGroups([]p2p.GroupID{node.NodeConfig.GetClientGroupID()}, host.ConstructP2pMessage(byte(0), proto_node.ConstructBlocksSyncMessage([]*types.Block{newBlock})))
	}
}

// VerifyNewBlock is called by consensus participants to verify the block (account model) they are running consensus on
func (node *Node) VerifyNewBlock(newBlock *types.Block) error {
	// TODO ek – where do we verify parent-child invariants,
	//  e.g. "child.Number == child.IsGenesis() ? 0 : parent.Number+1"?
	if newBlock.ShardID() != node.Blockchain().ShardID() {
		return ctxerror.New("wrong shard ID",
			"my shard ID", node.Blockchain().ShardID(),
			"new block's shard ID", newBlock.ShardID())
	}
	err := node.Blockchain().ValidateNewBlock(newBlock)
	if err != nil {
		return ctxerror.New("cannot ValidateNewBlock",
			"blockHash", newBlock.Hash(),
			"numTx", len(newBlock.Transactions()),
		).WithCause(err)
	}

	// TODO: verify the vrf randomness
	// _ = newBlock.Header().Vrf

	err = node.validateNewShardState(newBlock, &node.CurrentStakes)
	if err != nil {
		return ctxerror.New("failed to verify sharding state").WithCause(err)
	}
	return nil
}

// BigMaxUint64 is maximum possible uint64 value, that is, (1**64)-1.
var BigMaxUint64 = new(big.Int).SetBytes([]byte{
	255, 255, 255, 255, 255, 255, 255, 255,
})

// validateNewShardState validate whether the new shard state root matches
func (node *Node) validateNewShardState(block *types.Block, stakeInfo *map[common.Address]*structs.StakeInfo) error {
	// Common case first – blocks without resharding proposal
	header := block.Header()
	if header.ShardStateHash == (common.Hash{}) {
		// No new shard state was proposed
		if block.ShardID() == 0 {
			if core.IsEpochLastBlock(block) {
				// TODO ek - invoke view change
				return errors.New("beacon leader did not propose resharding")
			}
		} else {
			if node.nextShardState.master != nil &&
				!time.Now().Before(node.nextShardState.proposeTime) {
				// TODO ek – invoke view change
				return errors.New("regular leader did not propose resharding")
			}
		}
		// We aren't expecting to reshard, so proceed to sign
		return nil
	}
	var shardState *types.ShardState
	err := rlp.DecodeBytes(header.ShardState, shardState)
	if err != nil {
		return err
	}
	proposed := *shardState
	if block.ShardID() == 0 {
		// Beacon validators independently recalculate the master state and
		// compare it against the proposed copy.
		nextEpoch := new(big.Int).Add(block.Header().Epoch, common.Big1)
		// TODO ek – this may be called from regular shards,
		//  for vetting beacon chain blocks received during block syncing.
		//  DRand may or or may not get in the way.  Test this out.
		expected, err := core.CalculateNewShardState(
			node.Blockchain(), nextEpoch, stakeInfo)
		if err != nil {
			return ctxerror.New("cannot calculate expected shard state").
				WithCause(err)
		}
		if types.CompareShardState(expected, proposed) != 0 {
			// TODO ek – log state proposal differences
			// TODO ek – this error should trigger view change
			err := errors.New("shard state proposal is different from expected")
			// TODO ek/chao – calculated shard state is different even with the
			//  same input, i.e. it is nondeterministic.
			//  Don't treat this as a blocker until we fix the nondeterminism.
			//return err
			ctxerror.Log15(utils.GetLogger().Warn, err)
		}
	} else {
		// Regular validators fetch the local-shard copy on the beacon chain
		// and compare it against the proposed copy.
		//
		// We trust the master proposal in our copy of beacon chain.
		// The sanity check for the master proposal is done earlier,
		// when the beacon block containing the master proposal is received
		// and before it is admitted into the local beacon chain.
		//
		// TODO ek – fetch masterProposal from beaconchain instead
		masterProposal := node.nextShardState.master.ShardState
		expected := masterProposal.FindCommitteeByID(block.ShardID())
		switch len(proposed) {
		case 0:
			// Proposal to discontinue shard
			if expected != nil {
				// TODO ek – invoke view change
				return errors.New(
					"leader proposed to disband against beacon decision")
			}
		case 1:
			// Proposal to continue shard
			proposed := proposed[0]
			// Sanity check: Shard ID should match
			if proposed.ShardID != block.ShardID() {
				// TODO ek – invoke view change
				return ctxerror.New("proposal has incorrect shard ID",
					"proposedShard", proposed.ShardID,
					"blockShard", block.ShardID())
			}
			// Did beaconchain say we are no more?
			if expected == nil {
				// TODO ek – invoke view change
				return errors.New(
					"leader proposed to continue against beacon decision")
			}
			// Did beaconchain say the same proposal?
			if types.CompareCommittee(expected, &proposed) != 0 {
				// TODO ek – log differences
				// TODO ek – invoke view change
				return errors.New("proposal differs from one in beacon chain")
			}
		default:
			// TODO ek – invoke view change
			return ctxerror.New(
				"regular resharding proposal has incorrect number of shards",
				"numShards", len(proposed))
		}
	}
	return nil
}

// PostConsensusProcessing is called by consensus participants, after consensus is done, to:
// 1. add the new block to blockchain
// 2. [leader] send new block to the client
func (node *Node) PostConsensusProcessing(newBlock *types.Block) {
	if node.Consensus.PubKey.IsEqual(node.Consensus.LeaderPubKey) {
		node.BroadcastNewBlock(newBlock)
	} else {
		utils.GetLogInstance().Info("BINGO !!! Reached Consensus", "ViewID", node.Consensus.GetViewID())
	}

	node.AddNewBlock(newBlock)

	if node.NodeConfig.GetNetworkType() != nodeconfig.Mainnet {
		// Update contract deployer's nonce so default contract like faucet can issue transaction with current nonce
		nonce := node.GetNonceOfAddress(crypto.PubkeyToAddress(node.ContractDeployerKey.PublicKey))
		atomic.StoreUint64(&node.ContractDeployerCurrentNonce, nonce)

		for _, tx := range newBlock.Transactions() {
			msg, err := tx.AsMessage(types.HomesteadSigner{})
			if err != nil {
				utils.GetLogInstance().Error("Error when parsing tx into message")
			}
			if _, ok := node.AddressNonce.Load(msg.From()); ok {
				nonce := node.GetNonceOfAddress(msg.From())
				node.AddressNonce.Store(msg.From(), nonce)
			}
		}

		// TODO: Enable the following after v0
		if node.Consensus.ShardID == 0 {
			// TODO: enable drand only for beacon chain
			// ConfirmedBlockChannel which is listened by drand leader who will initiate DRG if its a epoch block (first block of a epoch)
			//if node.DRand != nil {
			//	go func() {
			//		node.ConfirmedBlockChannel <- newBlock
			//	}()
			//}

			// TODO: enable staking
			// TODO: update staking information once per epoch.
			//node.UpdateStakingList(node.QueryStakeInfo())
			//node.printStakingList()
		}

		// TODO: enable shard state update
		//newBlockHeader := newBlock.Header()
		//if newBlockHeader.ShardStateHash != (common.Hash{}) {
		//	if node.Consensus.ShardID == 0 {
		//		// TODO ek – this is a temp hack until beacon chain sync is fixed
		//		// End-of-epoch block on beacon chain; block's EpochState is the
		//		// master resharding table.  Broadcast it to the network.
		//		if err := node.broadcastEpochShardState(newBlock); err != nil {
		//			e := ctxerror.New("cannot broadcast shard state").WithCause(err)
		//			ctxerror.Log15(utils.GetLogInstance().Error, e)
		//		}
		//	}
		//	shardState, err := newBlockHeader.GetShardState()
		//	if err != nil {
		//		e := ctxerror.New("cannot get shard state from header").WithCause(err)
		//		ctxerror.Log15(utils.GetLogInstance().Error, e)
		//	} else {
		//		node.transitionIntoNextEpoch(shardState)
		//	}
		//}
	}
}

func (node *Node) broadcastEpochShardState(newBlock *types.Block) error {
	shardState, err := newBlock.Header().GetShardState()
	if err != nil {
		return err
	}
	epochShardStateMessage := proto_node.ConstructEpochShardStateMessage(
		types.EpochShardState{
			Epoch:      newBlock.Header().Epoch.Uint64() + 1,
			ShardState: shardState,
		},
	)
	return node.host.SendMessageToGroups(
		[]p2p.GroupID{node.NodeConfig.GetClientGroupID()},
		host.ConstructP2pMessage(byte(0), epochShardStateMessage))
}

// AddNewBlock is usedd to add new block into the blockchain.
func (node *Node) AddNewBlock(newBlock *types.Block) {
	blockNum, err := node.Blockchain().InsertChain([]*types.Block{newBlock})
	if err != nil {
		utils.GetLogInstance().Debug("Error Adding new block to blockchain", "blockNum", blockNum, "parentHash", newBlock.Header().ParentHash, "hash", newBlock.Header().Hash(), "Error", err)
	} else {
		utils.GetLogInstance().Info("Added New Block to Blockchain!!!", "blockNum", blockNum, "hash", newBlock.Header().Hash().Hex())
	}
}

type genesisNode struct {
	ShardID     uint32
	MemberIndex int
	NodeID      types.NodeID
}

var (
	genesisCatalogOnce          sync.Once
	genesisNodeByStakingAddress = make(map[common.Address]*genesisNode)
	genesisNodeByConsensusKey   = make(map[types.BlsPublicKey]*genesisNode)
)

func initGenesisCatalog() {
	genesisShardState := core.GetInitShardState()
	for _, committee := range genesisShardState {
		for i, nodeID := range committee.NodeList {
			genesisNode := &genesisNode{
				ShardID:     committee.ShardID,
				MemberIndex: i,
				NodeID:      nodeID,
			}
			genesisNodeByStakingAddress[nodeID.EcdsaAddress] = genesisNode
			genesisNodeByConsensusKey[nodeID.BlsPublicKey] = genesisNode
		}
	}
}

func getGenesisNodeByStakingAddress(address common.Address) *genesisNode {
	genesisCatalogOnce.Do(initGenesisCatalog)
	return genesisNodeByStakingAddress[address]
}

func getGenesisNodeByConsensusKey(key types.BlsPublicKey) *genesisNode {
	genesisCatalogOnce.Do(initGenesisCatalog)
	return genesisNodeByConsensusKey[key]
}

func (node *Node) pingMessageHandler(msgPayload []byte, sender libp2p_peer.ID) int {
	logger := utils.GetLogInstance().New("sender", sender)
	getLogger := func() log.Logger { return utils.WithCallerSkip(logger, 1) }

	senderStr := string(sender)
	if senderStr != "" {
		_, ok := node.duplicatedPing.LoadOrStore(senderStr, true)
		if ok {
			// duplicated ping message return
			return 0
		}
	}

	ping, err := proto_discovery.GetPingMessage(msgPayload)
	if err != nil {
		utils.GetLogInstance().Error("Can't get Ping Message", "error", err)
		return -1
	}

	peer := new(p2p.Peer)
	peer.IP = ping.Node.IP
	peer.Port = ping.Node.Port
	peer.PeerID = ping.Node.PeerID
	peer.ConsensusPubKey = nil
	logger = logger.New("ip", peer.IP, "port", peer.Port, "peerID", peer.PeerID)

	if ping.Node.PubKey != nil {
		peer.ConsensusPubKey = &bls.PublicKey{}
		if err := peer.ConsensusPubKey.Deserialize(ping.Node.PubKey[:]); err != nil {
			utils.GetLogInstance().Error("UnmarshalBinary Failed", "error", err)
			return -1
		}
		logger = logger.New(
			"peerConsensusPubKey", peer.ConsensusPubKey.SerializeToHexStr())
	}

	var k types.BlsPublicKey
	if err := k.FromLibBLSPublicKey(peer.ConsensusPubKey); err != nil {
		err = ctxerror.New("cannot convert BLS public key").WithCause(err)
		ctxerror.Log15(getLogger().Warn, err)
	}
	if genesisNode := getGenesisNodeByConsensusKey(k); genesisNode != nil {
		logger = logger.New(
			"genesisShardID", genesisNode.ShardID,
			"genesisMemberIndex", genesisNode.MemberIndex,
			"genesisStakingAccount", common2.MustAddressToBech32(genesisNode.NodeID.EcdsaAddress))
	} else {
		logger.Info("cannot find genesis node", "BlsPubKey", peer.ConsensusPubKey)
	}
	getLogger().Info("received ping message")

	// add to incoming peer list
	//node.host.AddIncomingPeer(*peer)
	node.host.ConnectHostPeer(*peer)

	if ping.Node.Role == proto_node.ClientRole {
		utils.GetLogInstance().Info("Add Client Peer to Node", "Client", peer)
		node.ClientPeer = peer
	} else {
		node.AddPeers([]*p2p.Peer{peer})
		utils.GetLogInstance().Info("Add Peer to Node", "Peer", peer, "# Peers", len(node.Consensus.PublicKeys))
	}

	return 1
}

// SendPongMessage is the a goroutine to periodcally send pong message to all peers
func (node *Node) SendPongMessage() {
	tick := time.NewTicker(2 * time.Second)
	tick2 := time.NewTicker(120 * time.Second)

	numPeers := node.numPeers
	sentMessage := false
	firstTime := true

	// Send Pong Message only when there is change on the number of peers
	for {
		select {
		case <-tick.C:
			peers := node.Consensus.GetValidatorPeers()
			numPeersNow := node.numPeers

			// no peers, wait for another tick
			if numPeersNow == 0 {
				utils.GetLogInstance().Info("[PONG] No peers, continue", "numPeers", numPeers, "numPeersNow", numPeersNow)
				continue
			}
			// new peers added
			if numPeersNow != numPeers {
				utils.GetLogInstance().Info("[PONG] Different number of peers", "numPeers", numPeers, "numPeersNow", numPeersNow)
				sentMessage = false
			} else {
				// stable number of peers, sent the pong message
				// also make sure number of peers is greater than the minimal required number
				if !sentMessage && numPeersNow >= node.Consensus.MinPeers {
					pong := proto_discovery.NewPongMessage(peers, node.Consensus.PublicKeys, node.Consensus.GetLeaderPubKey(), node.Consensus.ShardID)
					buffer := pong.ConstructPongMessage()
					err := node.host.SendMessageToGroups([]p2p.GroupID{node.NodeConfig.GetShardGroupID()}, host.ConstructP2pMessage(byte(0), buffer))
					if err != nil {
						utils.GetLogInstance().Error("[PONG] Failed to send pong message", "group", node.NodeConfig.GetShardGroupID())
						continue
					} else {
						utils.GetLogInstance().Info("[PONG] Sent pong message to", "group", node.NodeConfig.GetShardGroupID(), "# nodes", numPeersNow)
					}
					sentMessage = true

					// only need to notify consensus leader once to start the consensus
					if firstTime {
						// Leader stops sending ping message
						node.serviceManager.TakeAction(&service.Action{Action: service.Stop, ServiceType: service.PeerDiscovery})
						utils.GetLogInstance().Info("[PONG] StartConsensus")
						node.startConsensus <- struct{}{}
						firstTime = false
					}
				}
			}
			numPeers = numPeersNow
		case <-tick2.C:
			// send pong message regularly to make sure new node received all the public keys
			// also nodes offline/online will receive the public keys
			peers := node.Consensus.GetValidatorPeers()
			pong := proto_discovery.NewPongMessage(peers, node.Consensus.PublicKeys, node.Consensus.GetLeaderPubKey(), node.Consensus.ShardID)
			buffer := pong.ConstructPongMessage()
			err := node.host.SendMessageToGroups([]p2p.GroupID{node.NodeConfig.GetShardGroupID()}, host.ConstructP2pMessage(byte(0), buffer))
			if err != nil {
				utils.GetLogInstance().Error("[PONG] Failed to send regular pong message", "group", node.NodeConfig.GetShardGroupID())
				continue
			} else {
				utils.GetLogInstance().Info("[PONG] Sent regular pong message to", "group", node.NodeConfig.GetShardGroupID(), "# nodes", len(peers))
			}
		}
	}
}

func (node *Node) pongMessageHandler(msgPayload []byte) int {
	utils.GetLogInstance().Info("Got Pong Message")
	pong, err := proto_discovery.GetPongMessage(msgPayload)
	if err != nil {
		utils.GetLogInstance().Error("Can't get Pong Message", "error", err)
		return -1
	}

	if pong.ShardID != node.Consensus.ShardID {
		utils.GetLogInstance().Error(
			"Received Pong message for the wrong shard",
			"receivedShardID", pong.ShardID,
			"expectedShardID", node.Consensus.ShardID)
		return 0
	}

	// set the leader pub key is the first thing to do
	// otherwise, we may not be able to validate the consensus messages received
	// which will result in first consensus timeout
	// TODO: remove this after fully migrating to beacon chain-based committee membership
	//err = node.Consensus.SetLeaderPubKey(pong.LeaderPubKey)
	//if err != nil {
	//	utils.GetLogInstance().Error("Unmarshal Consensus Leader PubKey Failed", "error", err)
	//} else {
	//	utils.GetLogInstance().Info("Set Consensus Leader PubKey", "key", node.Consensus.GetLeaderPubKey())
	//}
	//err = node.DRand.SetLeaderPubKey(pong.LeaderPubKey)
	//if err != nil {
	//	utils.GetLogInstance().Error("Unmarshal DRand Leader PubKey Failed", "error", err)
	//} else {
	//	utils.GetLogInstance().Info("Set DRand Leader PubKey", "key", node.Consensus.GetLeaderPubKey())
	//}

	peers := make([]*p2p.Peer, 0)

	for _, p := range pong.Peers {
		peer := new(p2p.Peer)
		peer.IP = p.IP
		peer.Port = p.Port
		peer.PeerID = p.PeerID

		peer.ConsensusPubKey = &bls.PublicKey{}
		if len(p.PubKey) != 0 { // TODO: add the check in bls library
			err = peer.ConsensusPubKey.Deserialize(p.PubKey[:])
			if err != nil {
				utils.GetLogInstance().Error("UnmarshalBinary Failed", "error", err)
				continue
			}
		}
		peers = append(peers, peer)
	}

	if len(peers) > 0 {
		node.AddPeers(peers)
	}

	// Reset Validator PublicKeys every time we receive PONG message from Leader
	// The PublicKeys has to be idential across the shard on every node
	// TODO (lc): we need to handle RemovePeer situation
	publicKeys := make([]*bls.PublicKey, 0)

	// Create the the PubKey from the []byte sent from leader
	for _, k := range pong.PubKeys {
		key := bls.PublicKey{}
		err = key.Deserialize(k[:])
		if err != nil {
			utils.GetLogInstance().Error("UnmarshalBinary Failed PubKeys", "error", err)
			continue
		}
		publicKeys = append(publicKeys, &key)
	}

	utils.GetLogInstance().Debug("[pongMessageHandler]", "#keys", len(publicKeys), "#peers", len(peers))

	if node.State == NodeWaitToJoin {
		node.State = NodeReadyForConsensus
	}

	// Stop discovery service after received pong message
	data := make(map[string]interface{})
	data["peer"] = p2p.GroupAction{Name: node.NodeConfig.GetShardGroupID(), Action: p2p.ActionPause}

	node.serviceManager.TakeAction(&service.Action{Action: service.Notify, ServiceType: service.PeerDiscovery, Params: data})

	// TODO: remove this after fully migrating to beacon chain-based committee membership
	return 0
}

func (node *Node) epochShardStateMessageHandler(msgPayload []byte) error {
	logger := utils.GetLogInstance()
	getLogger := func() log.Logger { return utils.WithCallerSkip(logger, 1) }
	epochShardState, err := proto_node.DeserializeEpochShardStateFromMessage(msgPayload)
	if err != nil {
		return ctxerror.New("Can't get shard state message").WithCause(err)
	}
	if node.Consensus == nil && node.NodeConfig.Role() != nodeconfig.NewNode {
		return nil
	}
	receivedEpoch := big.NewInt(int64(epochShardState.Epoch))
	getLogger().Info("received new shard state", "epoch", receivedEpoch)
	node.nextShardState.master = epochShardState
	if node.NodeConfig.IsLeader() {
		// Wait a bit to allow the master table to reach other validators.
		node.nextShardState.proposeTime = time.Now().Add(5 * time.Second)
	} else {
		// Wait a bit to allow the master table to reach the leader,
		// and to allow the leader to propose next shard state based upon it.
		node.nextShardState.proposeTime = time.Now().Add(15 * time.Second)
	}
	// TODO ek – this should be done from replaying beaconchain once
	//  beaconchain sync is fixed
	err = node.Beaconchain().WriteShardState(
		receivedEpoch, epochShardState.ShardState)
	if err != nil {
		return ctxerror.New("cannot store shard state", "epoch", receivedEpoch).
			WithCause(err)
	}
	return nil
}

func (node *Node) transitionIntoNextEpoch(shardState types.ShardState) {
	logger := utils.GetLogInstance()
	getLogger := func() log.Logger { return utils.WithCallerSkip(logger, 1) }

	logger = logger.New(
		"blsPubKey", hex.EncodeToString(node.Consensus.PubKey.Serialize()),
		"curShard", node.Blockchain().ShardID(),
		"curLeader", node.NodeConfig.IsLeader())
	for _, c := range shardState {
		logger.Debug("new shard information",
			"shardID", c.ShardID,
			"nodeList", c.NodeList)
	}
	myShardID, isNextLeader := findRoleInShardState(
		node.Consensus.PubKey, shardState)
	logger = logger.New(
		"nextShard", myShardID,
		"nextLeader", isNextLeader)

	if myShardID == math.MaxUint32 {
		getLogger().Info("Somehow I got kicked out. Exiting")
		os.Exit(8) // 8 represents it's a loop and the program restart itself
	}

	myShardState := shardState[myShardID]

	// Update public keys
	var publicKeys []*bls.PublicKey
	for idx, nodeID := range myShardState.NodeList {
		key := &bls.PublicKey{}
		err := key.Deserialize(nodeID.BlsPublicKey[:])
		if err != nil {
			getLogger().Error("Failed to deserialize BLS public key in shard state",
				"idx", idx,
				"error", err)
		}
		publicKeys = append(publicKeys, key)
	}
	node.Consensus.UpdatePublicKeys(publicKeys)
	node.DRand.UpdatePublicKeys(publicKeys)

	if node.Blockchain().ShardID() == myShardID {
		getLogger().Info("staying in the same shard")
	} else {
		getLogger().Info("moving to another shard")
		if err := node.shardChains.Close(); err != nil {
			getLogger().Error("cannot close shard chains", "error", err)
		}
		restartProcess(getRestartArguments(myShardID))
	}
}

func findRoleInShardState(
	key *bls.PublicKey, state types.ShardState,
) (shardID uint32, isLeader bool) {
	keyBytes := key.Serialize()
	for idx, shard := range state {
		for nodeIdx, nodeID := range shard.NodeList {
			if bytes.Compare(nodeID.BlsPublicKey[:], keyBytes) == 0 {
				return uint32(idx), nodeIdx == 0
			}
		}
	}
	return math.MaxUint32, false
}

func restartProcess(args []string) {
	execFile, err := getBinaryPath()
	if err != nil {
		utils.GetLogInstance().Crit("Failed to get program path when restarting program", "error", err, "file", execFile)
	}
	utils.GetLogInstance().Info("Restarting program", "args", args, "env", os.Environ())
	err = syscall.Exec(execFile, args, os.Environ())
	if err != nil {
		utils.GetLogInstance().Crit("Failed to restart program after resharding", "error", err)
	}
	panic("syscall.Exec() is not supposed to return")
}

func getRestartArguments(myShardID uint32) []string {
	args := os.Args
	hasShardID := false
	shardIDFlag := "-shard_id"
	// newNodeFlag := "-is_newnode"
	for i, arg := range args {
		if arg == shardIDFlag {
			if i+1 < len(args) {
				args[i+1] = strconv.Itoa(int(myShardID))
			} else {
				args = append(args, strconv.Itoa(int(myShardID)))
			}
			hasShardID = true
		}
		// TODO: enable this
		//if arg == newNodeFlag {
		//	args[i] = ""  // remove new node flag
		//}
	}
	if !hasShardID {
		args = append(args, shardIDFlag)
		args = append(args, strconv.Itoa(int(myShardID)))
	}
	return args
}

// Gets the path of this currently running binary program.
func getBinaryPath() (argv0 string, err error) {
	argv0, err = exec.LookPath(os.Args[0])
	if nil != err {
		return
	}
	if _, err = os.Stat(argv0); nil != err {
		return
	}
	return
}

// ConsensusMessageHandler passes received message in node_handler to consensus
func (node *Node) ConsensusMessageHandler(msgPayload []byte) {
	node.Consensus.MsgChan <- msgPayload
}

// ExplorerMessageHandler passes received message in node_handler to explorer service
func (node *Node) ExplorerMessageHandler(payload []byte) {
	if len(payload) == 0 {
		utils.GetLogger().Debug("Payload is empty")
		return
	}
	msg := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, msg)
	if err != nil {
		utils.GetLogger().Error("Failed to unmarshal message payload.", "err", err)
		return
	}

	if msg.Type == msg_pb.MessageType_COMMITTED {

		recvMsg, err := consensus.ParsePbftMessage(msg)
		if err != nil {
			utils.GetLogInstance().Debug("[Explorer] onCommitted unable to parse msg", "error", err)
			return
		}

		aggSig, mask, err := node.Consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
		if err != nil {
			utils.GetLogInstance().Debug("[Explorer] readSignatureBitmapPayload failed", "error", err)
			return
		}

		// check has 2f+1 signatures
		if count := utils.CountOneBits(mask.Bitmap); count < node.Consensus.Quorum() {
			utils.GetLogInstance().Debug("[Explorer] not have enough signature", "need", node.Consensus.Quorum(), "have", count)
			return
		}

		blockNumHash := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumHash, recvMsg.BlockNum)
		commitPayload := append(blockNumHash, recvMsg.BlockHash[:]...)
		if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
			utils.GetLogInstance().Debug("[Explorer] Failed to verify the multi signature for commit phase", "msgBlock", recvMsg.BlockNum)
			return
		}
		block := node.Consensus.PbftLog.GetBlockByHash(recvMsg.BlockHash)

		// Dump new block into level db.
		utils.GetLogInstance().Info("[Explorer] Committing block into explorer DB", "msgBlock", recvMsg.BlockNum)
		explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port, true).Dump(block, block.NumberU64())

		node.Consensus.PbftLog.DeleteBlockByNumber(block.NumberU64())
	} else if msg.Type == msg_pb.MessageType_PREPARED {

		recvMsg, err := consensus.ParsePbftMessage(msg)
		if err != nil {
			utils.GetLogInstance().Debug("[Explorer] onAnnounce unable to parse msg", "error", err)
			return
		}
		block := recvMsg.Block

		var blockObj types.Block
		err = rlp.DecodeBytes(block, &blockObj)
		node.Consensus.PbftLog.AddBlock(&blockObj)
	}
	return
}
