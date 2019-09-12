package node

import (
	"bytes"
	"context"
	"math/big"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	pb "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"

	"github.com/harmony-one/harmony/api/proto"
	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	"github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/shard"
)

const (
	consensusTimeout   = 30 * time.Second
	crossLinkBatchSize = 7
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
			//utils.Logger().Info("[PUBSUB]", "received global msg", len(msg), "sender", sender)
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
			//utils.Logger().Info("[PUBSUB]", "received group msg", len(msg), "sender", sender)
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
			// utils.Logger().Info("[CLIENT]", "received group msg", len(msg), "sender", sender, "error", err)
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
		utils.Logger().Error().
			Err(err).
			Msg("messageHandler get message category failed")
		return
	}

	msgType, err := proto.GetMessageType(content)
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("messageHandler get message type failed")
		return
	}

	msgPayload, err := proto.GetMessagePayload(content)
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("messageHandler get message payload failed")
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
		utils.Logger().Debug().Msg("NET: Received staking message")
		msgPayload, _ := proto.GetStakingMessagePayload(content)
		// Only beacon leader processes staking txn
		if node.Consensus != nil && node.Consensus.ShardID == 0 && node.Consensus.IsLeader() {
			node.processStakingMessage(msgPayload)
		}
	case proto.Node:
		actionType := proto_node.MessageType(msgType)
		switch actionType {
		case proto_node.Transaction:
			utils.Logger().Debug().Msg("NET: received message: Node/Transaction")
			node.transactionMessageHandler(msgPayload)
		case proto_node.Block:
			utils.Logger().Debug().Msg("NET: received message: Node/Block")
			blockMsgType := proto_node.BlockMessageType(msgPayload[0])
			switch blockMsgType {
			case proto_node.Sync:
				utils.Logger().Debug().Msg("NET: received message: Node/Sync")
				var blocks []*types.Block
				err := rlp.DecodeBytes(msgPayload[1:], &blocks)
				if err != nil {
					utils.Logger().Error().
						Err(err).
						Msg("block sync")
				} else {
					// for non-beaconchain node, subscribe to beacon block broadcast
					role := node.NodeConfig.Role()
					if role == nodeconfig.Validator {

						for _, block := range blocks {
							if block.ShardID() == 0 {
								utils.Logger().Info().
									Uint64("block", blocks[0].NumberU64()).
									Msgf("Block being handled by block channel %d %d", block.NumberU64(), block.ShardID())
								node.BeaconBlockChannel <- block
							}
						}
					}
					if node.Client != nil && node.Client.UpdateBlocks != nil && blocks != nil {
						utils.Logger().Info().Msg("Block being handled by client")
						node.Client.UpdateBlocks(blocks)
					}
				}

			case proto_node.Header:
				// only beacon chain will accept the header from other shards
				utils.Logger().Debug().Uint32("shardID", node.NodeConfig.ShardID).Msg("NET: received message: Node/Header")
				if node.NodeConfig.ShardID != 0 {
					return
				}
				node.ProcessHeaderMessage(msgPayload[1:]) // skip first byte which is blockMsgType

			case proto_node.Receipt:
				utils.Logger().Debug().Msg("NET: received message: Node/Receipt")
				node.ProcessReceiptMessage(msgPayload[1:]) // skip first byte which is blockMsgType

			}
		case proto_node.PING:
			node.pingMessageHandler(msgPayload, sender)
		case proto_node.ShardState:
			if err := node.epochShardStateMessageHandler(msgPayload); err != nil {
				ctxerror.Log15(utils.GetLogger().Warn, err)
			}
		}
	default:
		utils.Logger().Error().
			Str("Unknown MsgCateogry", string(msgCategory))
	}
}

func (node *Node) processStakingMessage(msgPayload []byte) {
	msg := &message.Message{}
	err := pb.Unmarshal(msgPayload, msg)
	if err == nil {
		stakingRequest := msg.GetStaking()
		txs := types.Transactions{}
		if err = rlp.DecodeBytes(stakingRequest.Transaction, &txs); err == nil {
			utils.Logger().Info().Msg("Successfully added staking transaction to pending list.")
			node.addPendingTransactions(txs)
		} else {
			utils.Logger().Error().
				Err(err).
				Msg("Failed to unmarshal staking transaction list")
		}
	} else {
		utils.Logger().Error().
			Err(err).
			Msg("Failed to unmarshal staking msg payload")
	}
}

func (node *Node) transactionMessageHandler(msgPayload []byte) {
	txMessageType := proto_node.TransactionMessageType(msgPayload[0])

	switch txMessageType {
	case proto_node.Send:
		txs := types.Transactions{}
		err := rlp.Decode(bytes.NewReader(msgPayload[1:]), &txs) // skip the Send messge type
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Msg("Failed to deserialize transaction list")
		}
		node.addPendingTransactions(txs)
	}
}

// BroadcastNewBlock is called by consensus leader to sync new blocks with other clients/nodes.
// NOTE: For now, just send to the client (basically not broadcasting)
// TODO (lc): broadcast the new blocks to new nodes doing state sync
func (node *Node) BroadcastNewBlock(newBlock *types.Block) {
	groups := []p2p.GroupID{node.NodeConfig.GetClientGroupID()}
	utils.Logger().Info().Msgf("broadcasting new block %d, group %s", newBlock.NumberU64(), groups[0])
	msg := host.ConstructP2pMessage(byte(0), proto_node.ConstructBlocksSyncMessage([]*types.Block{newBlock}))
	if err := node.host.SendMessageToGroups(groups, msg); err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot broadcast new block")
	}
}

// BroadcastCrossLinkHeader is called by consensus leader to send the new header as cross link to beacon chain.
func (node *Node) BroadcastCrossLinkHeader(newBlock *types.Block) {
	utils.Logger().Info().Msgf("Broadcasting new header to beacon chain groupID %s", node.NodeConfig.GetBeaconGroupID())
	headers := []*block.Header{}
	lastLink, err := node.Beaconchain().ReadShardLastCrossLink(newBlock.ShardID())
	var latestBlockNum uint64

	// if cannot find latest crosslink header, broadcast latest 3 block headers
	if err != nil {
		utils.Logger().Debug().Err(err).Msg("[BroadcastCrossLinkHeader] ReadShardLastCrossLink Failed")
		header := node.Blockchain().GetHeaderByNumber(newBlock.NumberU64() - 2)
		if header != nil {
			headers = append(headers, header)
		}
		header = node.Blockchain().GetHeaderByNumber(newBlock.NumberU64() - 1)
		if header != nil {
			headers = append(headers, header)
		}
		headers = append(headers, newBlock.Header())
	} else {
		latestBlockNum = lastLink.BlockNum().Uint64()
		for blockNum := latestBlockNum + 1; blockNum <= newBlock.NumberU64(); blockNum++ {
			if blockNum > latestBlockNum+crossLinkBatchSize {
				break
			}
			header := node.Blockchain().GetHeaderByNumber(blockNum)
			if header != nil {
				headers = append(headers, header)
			}
		}
	}

	utils.Logger().Info().Msgf("[BroadcastCrossLinkHeader] Broadcasting Block Headers, latestBlockNum %d, currentBlockNum %d, Number of Headers %d", latestBlockNum, newBlock.NumberU64(), len(headers))
	for _, header := range headers {
		utils.Logger().Debug().Msgf("[BroadcastCrossLinkHeader] Broadcasting %d", header.Number().Uint64())
	}
	node.host.SendMessageToGroups([]p2p.GroupID{node.NodeConfig.GetBeaconGroupID()}, host.ConstructP2pMessage(byte(0), proto_node.ConstructCrossLinkHeadersMessage(headers)))
}

// VerifyNewBlock is called by consensus participants to verify the block (account model) they are running consensus on
func (node *Node) VerifyNewBlock(newBlock *types.Block) error {
	// TODO ek – where do we verify parent-child invariants,
	//  e.g. "child.Number == child.IsGenesis() ? 0 : parent.Number+1"?

	err := node.Blockchain().Validator().ValidateHeader(newBlock, true)
	if err != nil {
		return ctxerror.New("cannot ValidateHeader for the new block", "blockHash", newBlock.Hash()).WithCause(err)
	}
	if newBlock.ShardID() != node.Blockchain().ShardID() {
		return ctxerror.New("wrong shard ID",
			"my shard ID", node.Blockchain().ShardID(),
			"new block's shard ID", newBlock.ShardID())
	}
	err = node.Blockchain().ValidateNewBlock(newBlock)
	if err != nil {
		return ctxerror.New("cannot ValidateNewBlock",
			"blockHash", newBlock.Hash(),
			"numTx", len(newBlock.Transactions()),
		).WithCause(err)
	}

	// Verify cross links
	// TODO: move into ValidateNewBlock
	if node.NodeConfig.ShardID == 0 {
		err := node.VerifyBlockCrossLinks(newBlock)
		if err != nil {
			utils.Logger().Debug().Err(err).Msg("ops2 VerifyBlockCrossLinks Failed")
			return err
		}
	}

	// TODO: move into ValidateNewBlock
	err = node.verifyIncomingReceipts(newBlock)
	if err != nil {
		return ctxerror.New("[VerifyNewBlock] Cannot ValidateNewBlock", "blockHash", newBlock.Hash(),
			"numIncomingReceipts", len(newBlock.IncomingReceipts())).WithCause(err)
	}

	// TODO: verify the vrf randomness
	// _ = newBlock.Header().Vrf

	// TODO: uncomment 4 lines after we finish staking mechanism
	//err = node.validateNewShardState(newBlock, &node.CurrentStakes)
	//	if err != nil {
	//		return ctxerror.New("failed to verify sharding state").WithCause(err)
	//	}
	return nil
}

// BigMaxUint64 is maximum possible uint64 value, that is, (1**64)-1.
var BigMaxUint64 = new(big.Int).SetBytes([]byte{
	255, 255, 255, 255, 255, 255, 255, 255,
})

// PostConsensusProcessing is called by consensus participants, after consensus is done, to:
// 1. add the new block to blockchain
// 2. [leader] send new block to the client
// 3. [leader] send cross shard tx receipts to destination shard
func (node *Node) PostConsensusProcessing(newBlock *types.Block, commitSigAndBitmap []byte) {
	if err := node.AddNewBlock(newBlock); err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("Error when adding new block")
		return
	} else if core.IsEpochLastBlock(newBlock) {
		node.Consensus.UpdateConsensusInformation()
	}

	// Update last consensus time for metrics
	// TODO: randomly selected a few validators to broadcast messages instead of only leader broadcast
	node.lastConsensusTime = time.Now().Unix()
	if node.Consensus.PubKey.IsEqual(node.Consensus.LeaderPubKey) {
		if node.NodeConfig.ShardID == 0 {
			node.BroadcastNewBlock(newBlock)
		}
		if node.NodeConfig.ShardID != 0 && newBlock.Epoch().Cmp(node.Blockchain().Config().CrossLinkEpoch) >= 0 {
			node.BroadcastCrossLinkHeader(newBlock)
		}
		node.BroadcastCXReceipts(newBlock, commitSigAndBitmap)
	} else {
		utils.Logger().Info().
			Uint64("BlockNum", newBlock.NumberU64()).
			Msg("BINGO !!! Reached Consensus")
		// Print to normal log too
		utils.GetLogInstance().Info("BINGO !!! Reached Consensus", "BlockNum", newBlock.NumberU64())

		// 15% of the validator also need to do broadcasting
		rand.Seed(time.Now().UTC().UnixNano())
		rnd := rand.Intn(100)
		if rnd < 15 {
			node.BroadcastCXReceipts(newBlock, commitSigAndBitmap)
		}
	}

	node.BroadcastMissingCXReceipts()

	// TODO chao: uncomment this after beacon syncing is stable
	// node.Blockchain().UpdateCXReceiptsCheckpointsByBlock(newBlock)

	if node.NodeConfig.GetNetworkType() != nodeconfig.Mainnet {
		// Update contract deployer's nonce so default contract like faucet can issue transaction with current nonce
		nonce := node.GetNonceOfAddress(crypto.PubkeyToAddress(node.ContractDeployerKey.PublicKey))
		atomic.StoreUint64(&node.ContractDeployerCurrentNonce, nonce)

		for _, tx := range newBlock.Transactions() {
			msg, err := tx.AsMessage(types.HomesteadSigner{})
			if err != nil {
				utils.Logger().Error().Msg("Error when parsing tx into message")
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
		//			ctxerror.Log15(utils.Logger().Error, e)
		//		}
		//	}
		//	shardState, err := newBlockHeader.GetShardState()
		//	if err != nil {
		//		e := ctxerror.New("cannot get shard state from header").WithCause(err)
		//		ctxerror.Log15(utils.Logger().Error, e)
		//	} else {
		//		node.transitionIntoNextEpoch(shardState)
		//	}
		//}
	}
}

// AddNewBlock is usedd to add new block into the blockchain.
func (node *Node) AddNewBlock(newBlock *types.Block) error {
	_, err := node.Blockchain().InsertChain([]*types.Block{newBlock})
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Uint64("blockNum", newBlock.NumberU64()).
			Str("parentHash", newBlock.Header().ParentHash().Hex()).
			Str("hash", newBlock.Header().Hash().Hex()).
			Msg("Error Adding new block to blockchain")
	} else {
		utils.Logger().Info().
			Uint64("blockNum", newBlock.NumberU64()).
			Str("hash", newBlock.Header().Hash().Hex()).
			Msg("Added New Block to Blockchain!!!")
	}
	return err
}

type genesisNode struct {
	ShardID     uint32
	MemberIndex int
	NodeID      shard.NodeID
}

var (
	genesisCatalogOnce          sync.Once
	genesisNodeByStakingAddress = make(map[common.Address]*genesisNode)
	genesisNodeByConsensusKey   = make(map[shard.BlsPublicKey]*genesisNode)
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

func getGenesisNodeByConsensusKey(key shard.BlsPublicKey) *genesisNode {
	genesisCatalogOnce.Do(initGenesisCatalog)
	return genesisNodeByConsensusKey[key]
}

func (node *Node) pingMessageHandler(msgPayload []byte, sender libp2p_peer.ID) int {
	ping, err := proto_discovery.GetPingMessage(msgPayload)
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("Can't get Ping Message")
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
			utils.Logger().Error().
				Err(err).
				Msg("UnmarshalBinary Failed")
			return -1
		}
	}

	utils.Logger().Debug().
		Str("Version", ping.NodeVer).
		Str("BlsKey", peer.ConsensusPubKey.SerializeToHexStr()).
		Str("IP", peer.IP).
		Str("Port", peer.Port).
		Interface("PeerID", peer.PeerID).
		Msg("[PING] PeerInfo")

	senderStr := string(sender)
	if senderStr != "" {
		_, ok := node.duplicatedPing.LoadOrStore(senderStr, true)
		if ok {
			// duplicated ping message return
			return 0
		}
	}

	// add to incoming peer list
	//node.host.AddIncomingPeer(*peer)
	node.host.ConnectHostPeer(*peer)

	if ping.Node.Role != proto_node.ClientRole {
		node.AddPeers([]*p2p.Peer{peer})
		utils.Logger().Info().
			Str("Peer", peer.String()).
			Int("# Peers", node.numPeers).
			Msg("Add Peer to Node")
	}

	return 1
}

// bootstrapConsensus is the a goroutine to check number of peers and start the consensus
func (node *Node) bootstrapConsensus() {
	tick := time.NewTicker(5 * time.Second)
	lastPeerNum := node.numPeers
	for {
		select {
		case <-tick.C:
			numPeersNow := node.numPeers
			// no peers, wait for another tick
			if numPeersNow == 0 {
				utils.Logger().Info().
					Int("numPeersNow", numPeersNow).
					Msg("No peers, continue")
				continue
			} else if numPeersNow > lastPeerNum {
				utils.Logger().Info().
					Int("previousNumPeers", lastPeerNum).
					Int("numPeersNow", numPeersNow).
					Int("targetNumPeers", node.Consensus.MinPeers).
					Msg("New peers increased")
				lastPeerNum = numPeersNow
			}

			if numPeersNow >= node.Consensus.MinPeers {
				utils.Logger().Info().Msg("[bootstrap] StartConsensus")
				node.startConsensus <- struct{}{}
				return
			}
		}
	}
}

// ConsensusMessageHandler passes received message in node_handler to consensus
func (node *Node) ConsensusMessageHandler(msgPayload []byte) {
	node.Consensus.MsgChan <- msgPayload
}
