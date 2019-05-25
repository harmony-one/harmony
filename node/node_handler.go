package node

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/harmony-one/harmony/core"

	"github.com/ethereum/go-ethereum/rlp"
	pb "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/api/proto"
	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	"github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/pki"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

const (
	// MaxNumberOfTransactionsPerBlock is the max number of transaction per a block.
	MaxNumberOfTransactionsPerBlock = 8000
	consensusTimeout                = 7 * time.Second
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
				go node.messageHandler(msg[5:], string(sender))
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
				go node.messageHandler(msg[5:], string(sender))
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
				go node.messageHandler(msg[5:], string(sender))
			}
		}
	}
}

// messageHandler parses the message and dispatch the actions
func (node *Node) messageHandler(content []byte, sender string) {
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
		node.ConsensusMessageHandler(msgPayload)
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
			node.epochShardStateMessageHandler(msgPayload)
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
func (node *Node) VerifyNewBlock(newBlock *types.Block) bool {
	err := node.blockchain.ValidateNewBlock(newBlock, pki.GetAddressFromPublicKey(node.SelfPeer.ConsensusPubKey))
	if err != nil {
		utils.GetLogInstance().Debug("Failed verifying new block", "Error", err, "tx", newBlock.Transactions()[0])
		return false
	}

	// TODO: verify the vrf randomness
	_ = newBlock.Header().RandPreimage

	err = node.blockchain.ValidateNewShardState(newBlock, &node.CurrentStakes)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to verify new sharding state", "err", err)
	}
	return true
}

// PostConsensusProcessing is called by consensus participants, after consensus is done, to:
// 1. add the new block to blockchain
// 2. [leader] send new block to the client
func (node *Node) PostConsensusProcessing(newBlock *types.Block) {
	if nodeconfig.GetDefaultConfig().IsLeader() {
		node.BroadcastNewBlock(newBlock)
	} else {
		utils.GetLogInstance().Info("BINGO !!! Reached Consensus", "ConsensusID", node.Consensus.GetConsensusID())
	}

	node.AddNewBlock(newBlock)

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

	if node.Consensus.ShardID == 0 {
		// Update contract deployer's nonce so default contract like faucet can issue transaction with current nonce
		nonce := node.GetNonceOfAddress(crypto.PubkeyToAddress(node.ContractDeployerKey.PublicKey))
		atomic.StoreUint64(&node.ContractDeployerCurrentNonce, nonce)

		// TODO: enable drand only for beacon chain
		// ConfirmedBlockChannel which is listened by drand leader who will initiate DRG if its a epoch block (first block of a epoch)
		if node.DRand != nil {
			go func() {
				node.ConfirmedBlockChannel <- newBlock
			}()
		}

		// ConfirmedBlockChannel which is listened by drand leader who will initiate DRG if its a epoch block (first block of a epoch)
		if node.DRand != nil {
			go func() {
				node.ConfirmedBlockChannel <- newBlock
			}()
		}

		// TODO: update staking information once per epoch.
		node.UpdateStakingList(node.QueryStakeInfo())
		node.printStakingList()
		if core.IsEpochBlock(newBlock) {
			shardState := node.blockchain.StoreNewShardState(newBlock, &node.CurrentStakes)
			if shardState != nil {
				if nodeconfig.GetDefaultConfig().IsLeader() {
					epochShardState := types.EpochShardState{Epoch: core.GetEpochFromBlockNumber(newBlock.NumberU64()), ShardState: shardState}
					epochShardStateMessage := proto_node.ConstructEpochShardStateMessage(epochShardState)
					// Broadcast new shard state
					err := node.host.SendMessageToGroups([]p2p.GroupID{node.NodeConfig.GetClientGroupID()}, host.ConstructP2pMessage(byte(0), epochShardStateMessage))
					if err != nil {
						utils.GetLogInstance().Error("[Resharding] failed to broadcast shard state message", "group", node.NodeConfig.GetClientGroupID())
					} else {
						utils.GetLogInstance().Info("[Resharding] broadcasted shard state message to", "group", node.NodeConfig.GetClientGroupID())
					}
				}
				node.processEpochShardState(&types.EpochShardState{Epoch: core.GetEpochFromBlockNumber(newBlock.NumberU64()), ShardState: shardState})
			}
		}
	}
}

// AddNewBlock is usedd to add new block into the blockchain.
func (node *Node) AddNewBlock(newBlock *types.Block) {
	blockNum, err := node.blockchain.InsertChain([]*types.Block{newBlock})
	if err != nil {
		utils.GetLogInstance().Debug("Error adding new block to blockchain", "blockNum", blockNum, "hash", newBlock.Header().Hash(), "Error", err)
	} else {
		utils.GetLogInstance().Info("adding new block to blockchain", "blockNum", blockNum, "hash", newBlock.Header().Hash(), "by node", node.SelfPeer)
	}
}

func (node *Node) pingMessageHandler(msgPayload []byte, sender string) int {
	utils.GetLogInstance().Error("Got Ping Message")
	if sender != "" {
		_, ok := node.duplicatedPing.LoadOrStore(sender, true)
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

	if ping.Node.PubKey != nil {
		peer.ConsensusPubKey = &bls.PublicKey{}
		if err := peer.ConsensusPubKey.Deserialize(ping.Node.PubKey[:]); err != nil {
			utils.GetLogInstance().Error("UnmarshalBinary Failed", "error", err)
			return -1
		}
	}

	//	utils.GetLogInstance().Debug("[pingMessageHandler]", "incoming peer", peer)

	// add to incoming peer list
	//node.host.AddIncomingPeer(*peer)
	node.host.ConnectHostPeer(*peer)

	if ping.Node.Role == proto_node.ClientRole {
		utils.GetLogInstance().Info("Add Client Peer to Node", "Address", node.Consensus.GetSelfAddress(), "Client", peer)
		node.ClientPeer = peer
	} else {
		node.AddPeers([]*p2p.Peer{peer})
		utils.GetLogInstance().Info("Add Peer to Node", "Address", node.Consensus.GetSelfAddress(), "Peer", peer, "# Peers", len(node.Consensus.PublicKeys))
	}

	return 1
}

// SendPongMessage is the a goroutine to periodcally send pong message to all peers
func (node *Node) SendPongMessage() {
	tick := time.NewTicker(2 * time.Second)
	tick2 := time.NewTicker(120 * time.Second)

	numPeers := len(node.Consensus.GetValidatorPeers())
	numPubKeys := len(node.Consensus.PublicKeys)
	sentMessage := false
	firstTime := true

	// Send Pong Message only when there is change on the number of peers
	for {
		select {
		case <-tick.C:
			peers := node.Consensus.GetValidatorPeers()
			numPeersNow := len(peers)
			numPubKeysNow := len(node.Consensus.PublicKeys)

			// no peers, wait for another tick
			if numPubKeysNow == 0 {
				utils.GetLogInstance().Info("[PONG] no peers, continue", "numPeers", numPeers, "numPeersNow", numPeersNow)
				continue
			}
			// new peers added
			if numPubKeysNow != numPubKeys || numPeersNow != numPeers {
				utils.GetLogInstance().Info("[PONG] different number of peers", "numPeers", numPeers, "numPeersNow", numPeersNow)
				sentMessage = false
			} else {
				// stable number of peers/pubkeys, sent the pong message
				// also make sure number of peers is greater than the minimal required number
				if !sentMessage && numPubKeysNow >= node.Consensus.MinPeers {
					pong := proto_discovery.NewPongMessage(peers, node.Consensus.PublicKeys, node.Consensus.GetLeaderPubKey(), node.Consensus.ShardID)
					buffer := pong.ConstructPongMessage()
					err := node.host.SendMessageToGroups([]p2p.GroupID{node.NodeConfig.GetShardGroupID()}, host.ConstructP2pMessage(byte(0), buffer))
					if err != nil {
						utils.GetLogInstance().Error("[PONG] failed to send pong message", "group", node.NodeConfig.GetShardGroupID())
						continue
					} else {
						utils.GetLogInstance().Info("[PONG] sent pong message to", "group", node.NodeConfig.GetShardGroupID(), "# nodes", numPeersNow)
					}
					sentMessage = true
					// wait a bit until all validators received pong message
					time.Sleep(200 * time.Millisecond)

					// only need to notify consensus leader once to start the consensus
					if firstTime {
						// Leader stops sending ping message
						time.Sleep(5 * time.Second)
						node.serviceManager.TakeAction(&service.Action{Action: service.Stop, ServiceType: service.PeerDiscovery})
						node.startConsensus <- struct{}{}
						firstTime = false
					}
				}
			}
			numPeers = numPeersNow
			numPubKeys = numPubKeysNow
		case <-tick2.C:
			// send pong message regularly to make sure new node received all the public keys
			// also nodes offline/online will receive the public keys
			peers := node.Consensus.GetValidatorPeers()
			pong := proto_discovery.NewPongMessage(peers, node.Consensus.PublicKeys, node.Consensus.GetLeaderPubKey(), node.Consensus.ShardID)
			buffer := pong.ConstructPongMessage()
			err := node.host.SendMessageToGroups([]p2p.GroupID{node.NodeConfig.GetShardGroupID()}, host.ConstructP2pMessage(byte(0), buffer))
			if err != nil {
				utils.GetLogInstance().Error("[PONG] failed to send regular pong message", "group", node.NodeConfig.GetShardGroupID())
				continue
			} else {
				utils.GetLogInstance().Info("[PONG] sent regular pong message to", "group", node.NodeConfig.GetShardGroupID(), "# nodes", len(peers))
			}
		}
	}
}

func (node *Node) pongMessageHandler(msgPayload []byte) int {
	utils.GetLogInstance().Error("Got Pong Message")
	pong, err := proto_discovery.GetPongMessage(msgPayload)
	if err != nil {
		utils.GetLogInstance().Error("Can't get Pong Message", "error", err)
		return -1
	}

	if pong.ShardID != node.Consensus.ShardID {
		utils.GetLogInstance().Error("Received Pong message for the wrong shard", "receivedShardID", pong.ShardID)
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
		err = peer.ConsensusPubKey.Deserialize(p.PubKey[:])
		if err != nil {
			utils.GetLogInstance().Error("UnmarshalBinary Failed", "error", err)
			continue
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

func (node *Node) epochShardStateMessageHandler(msgPayload []byte) int {
	utils.GetLogInstance().Error("[Received new shard state]")
	epochShardState, err := proto_node.DeserializeEpochShardStateFromMessage(msgPayload)
	if err != nil {
		utils.GetLogInstance().Error("Can't get shard state Message", "error", err)
		return -1
	}
	if (node.Consensus != nil && node.Consensus.ShardID != 0) || node.NodeConfig.Role() == nodeconfig.NewNode {
		node.processEpochShardState(epochShardState)
	}
	return 0
}

func (node *Node) processEpochShardState(epochShardState *types.EpochShardState) {
	shardState := epochShardState.ShardState
	epoch := epochShardState.Epoch

	for _, c := range shardState {
		utils.GetLogInstance().Debug("new shard information", "shardID", c.ShardID, "NodeList", c.NodeList)
	}

	myShardID := uint32(math.MaxUint32)
	isNextLeader := false
	myBlsPubKey := node.Consensus.PubKey.Serialize()
	myShardState := types.Committee{}
	for _, shard := range shardState {
		for _, nodeID := range shard.NodeList {
			if bytes.Compare(nodeID.BlsPublicKey[:], myBlsPubKey) == 0 {
				myShardID = shard.ShardID
				isNextLeader = shard.Leader == nodeID
				myShardState = shard
			}
		}
	}

	if myShardID != uint32(math.MaxUint32) {
		// Update public keys
		ss := myShardState
		publicKeys := []*bls.PublicKey{}
		for _, nodeID := range ss.NodeList {
			key := &bls.PublicKey{}
			err := key.Deserialize(nodeID.BlsPublicKey[:])
			if err != nil {
				utils.GetLogInstance().Error("Failed to deserialize BLS public key in shard state", "error", err)
			}
			publicKeys = append(publicKeys, key)
		}
		node.Consensus.UpdatePublicKeys(publicKeys)
		node.DRand.UpdatePublicKeys(publicKeys)

		aboutLeader := ""
		if nodeconfig.GetDefaultConfig().IsLeader() {
			aboutLeader = "I am not leader anymore"
			if isNextLeader {
				aboutLeader = "I am still leader"
			}
		} else {
			aboutLeader = "I am still validator"
			if isNextLeader {
				aboutLeader = "I become the leader"
			}
		}
		if node.blockchain.ShardID() == myShardID {
			utils.GetLogInstance().Info(fmt.Sprintf("[Resharded][epoch:%d] I stay at shard %d, %s", epoch, myShardID, aboutLeader), "BlsPubKey", hex.EncodeToString(myBlsPubKey))
		} else {
			utils.GetLogInstance().Info(fmt.Sprintf("[Resharded][epoch:%d] I got resharded to shard %d from shard %d, %s", epoch, myShardID, node.blockchain.ShardID(), aboutLeader), "BlsPubKey", hex.EncodeToString(myBlsPubKey))
			node.storeEpochShardState(epochShardState)

			execFile, err := getBinaryPath()
			if err != nil {
				utils.GetLogInstance().Crit("Failed to get program path when restarting program", "error", err, "file", execFile)
			}
			args := getRestartArguments(myShardID)
			utils.GetLogInstance().Info("Restarting program", "args", args, "env", os.Environ())
			err = syscall.Exec(execFile, args, os.Environ())
			if err != nil {
				utils.GetLogInstance().Crit("Failed to restart program after resharding", "error", err)
			}
		}
	} else {
		utils.GetLogInstance().Info(fmt.Sprintf("[Resharded][epoch:%d]  Somehow I got kicked out. Exiting", epoch), "BlsPubKey", hex.EncodeToString(myBlsPubKey))
		os.Exit(8) // 8 represents it's a loop and the program restart itself
	}
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

// Stores the epoch shard state into local file
// TODO: think about storing it into level db.
func (node *Node) storeEpochShardState(epochShardState *types.EpochShardState) {
	byteBuffer := bytes.NewBuffer([]byte{})
	encoder := gob.NewEncoder(byteBuffer)
	err := encoder.Encode(epochShardState)
	if err != nil {
		utils.GetLogInstance().Error("[Resharded] Failed to encode epoch shard state", "error", err)
	}
	err = ioutil.WriteFile("./epoch_shard_state"+node.SelfPeer.IP+node.SelfPeer.Port, byteBuffer.Bytes(), 0644)
	if err != nil {
		utils.GetLogInstance().Error("[Resharded] Failed to store epoch shard state in local file", "error", err)
	}
}

func (node *Node) retrieveEpochShardState() (*types.EpochShardState, error) {
	b, err := ioutil.ReadFile("./epoch_shard_state" + node.SelfPeer.IP + node.SelfPeer.Port)
	if err != nil {
		utils.GetLogInstance().Error("[Resharded] Failed to retrieve epoch shard state", "error", err)
	}
	epochShardState := new(types.EpochShardState)

	r := bytes.NewBuffer(b)
	decoder := gob.NewDecoder(r)
	err = decoder.Decode(epochShardState)

	if err != nil {
		return nil, fmt.Errorf("Decode local epoch shard state error")
	}
	return epochShardState, nil
}

// ConsensusMessageHandler passes received message in node_handler to consensus
func (node *Node) ConsensusMessageHandler(msgPayload []byte) {
	if node.Consensus.ConsensusVersion == "v1" {
		if nodeconfig.GetDefaultConfig().IsLeader() {
			node.Consensus.ProcessMessageLeader(msgPayload)
		} else {
			node.Consensus.ProcessMessageValidator(msgPayload)
		}
		return
	}
	if node.Consensus.ConsensusVersion == "v2" {
		select {
		case node.Consensus.MsgChan <- msgPayload:
		case <-time.After(consensusTimeout):
			utils.GetLogInstance().Debug("[Consensus] ConsensusMessageHandler timeout", "duration", consensusTimeout)
		}
		return
	}
}
