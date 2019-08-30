package node

import (
	"bytes"
	"context"
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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	pb "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"

	"github.com/harmony-one/harmony/api/proto"
	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	"github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

const (
	consensusTimeout = 30 * time.Second
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
				utils.Logger().Debug().Msg("NET: received message: Node/Header")
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
	lastThreeHeaders := []*types.Header{}

	block := node.Blockchain().GetBlockByNumber(newBlock.NumberU64() - 2)
	if block != nil {
		lastThreeHeaders = append(lastThreeHeaders, block.Header())
	}
	block = node.Blockchain().GetBlockByNumber(newBlock.NumberU64() - 1)
	if block != nil {
		lastThreeHeaders = append(lastThreeHeaders, block.Header())
	}
	lastThreeHeaders = append(lastThreeHeaders, newBlock.Header())

	node.host.SendMessageToGroups([]p2p.GroupID{node.NodeConfig.GetBeaconGroupID()}, host.ConstructP2pMessage(byte(0), proto_node.ConstructCrossLinkHeadersMessage(lastThreeHeaders)))
}

// BroadcastCXReceipts broadcasts cross shard receipts to correspoding
// destination shards
func (node *Node) BroadcastCXReceipts(newBlock *types.Block) {
	epoch := newBlock.Header().Epoch
	shardingConfig := core.ShardingSchedule.InstanceForEpoch(epoch)
	shardNum := int(shardingConfig.NumShards())
	myShardID := node.Consensus.ShardID
	utils.Logger().Info().Int("shardNum", shardNum).Uint32("myShardID", myShardID).Uint64("blockNum", newBlock.NumberU64()).Msg("[BroadcastCXReceipts]")

	for i := 0; i < shardNum; i++ {
		if i == int(myShardID) {
			continue
		}
		cxReceipts, err := node.Blockchain().ReadCXReceipts(uint32(i), newBlock.NumberU64(), newBlock.Hash(), false)
		if err != nil || len(cxReceipts) == 0 {
			//utils.Logger().Warn().Err(err).Uint32("ToShardID", uint32(i)).Int("numCXReceipts", len(cxReceipts)).Msg("[BroadcastCXReceipts] No ReadCXReceipts found")
			continue
		}
		merkleProof, err := node.Blockchain().CXMerkleProof(uint32(i), newBlock)
		if err != nil {
			utils.Logger().Warn().Uint32("ToShardID", uint32(i)).Msg("[BroadcastCXReceipts] Unable to get merkleProof")
			continue
		}
		utils.Logger().Info().Uint32("ToShardID", uint32(i)).Msg("[BroadcastCXReceipts] ReadCXReceipts and MerkleProof Found")

		groupID := p2p.ShardID(i)
		go node.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(groupID)}, host.ConstructP2pMessage(byte(0), proto_node.ConstructCXReceiptsProof(cxReceipts, merkleProof)))
	}
}

// VerifyNewBlock is called by consensus participants to verify the block (account model) they are running consensus on
func (node *Node) VerifyNewBlock(newBlock *types.Block) error {
	// TODO ek – where do we verify parent-child invariants,
	//  e.g. "child.Number == child.IsGenesis() ? 0 : parent.Number+1"?

	if newBlock.NumberU64() > 1 {
		err := core.VerifyBlockLastCommitSigs(node.Blockchain(), newBlock)
		if err != nil {
			return err
		}
	}
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

	// Verify cross links
	if node.NodeConfig.ShardID == 0 {
		err := node.VerifyBlockCrossLinks(newBlock)
		if err != nil {
			utils.Logger().Debug().Err(err).Msg("ops2 VerifyBlockCrossLinks Failed")
			return err
		}
	}

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

// VerifyBlockCrossLinks verifies the cross links of the block
func (node *Node) VerifyBlockCrossLinks(block *types.Block) error {
	if len(block.Header().CrossLinks) == 0 {
		return nil
	}
	crossLinks := &types.CrossLinks{}
	err := rlp.DecodeBytes(block.Header().CrossLinks, crossLinks)
	if err != nil {
		return ctxerror.New("[CrossLinkVerification] failed to decode cross links",
			"blockHash", block.Hash(),
			"crossLinks", len(block.Header().CrossLinks),
		).WithCause(err)
	}

	if !crossLinks.IsSorted() {
		return ctxerror.New("[CrossLinkVerification] cross links are not sorted",
			"blockHash", block.Hash(),
			"crossLinks", len(block.Header().CrossLinks),
		)
	}

	firstCrossLinkBlock := core.ShardingSchedule.FirstCrossLinkBlock()

	for i, crossLink := range *crossLinks {
		lastLink := &types.CrossLink{}
		if i == 0 {
			if crossLink.BlockNum().Uint64() > firstCrossLinkBlock {
				lastLink, err = node.Blockchain().ReadShardLastCrossLink(crossLink.ShardID())
				if err != nil {
					return ctxerror.New("[CrossLinkVerification] no last cross link found 1",
						"blockHash", block.Hash(),
						"crossLink", lastLink,
					).WithCause(err)
				}
			}
		} else {
			if (*crossLinks)[i-1].Header().ShardID != crossLink.Header().ShardID {
				if crossLink.BlockNum().Uint64() > firstCrossLinkBlock {
					lastLink, err = node.Blockchain().ReadShardLastCrossLink(crossLink.ShardID())
					if err != nil {
						return ctxerror.New("[CrossLinkVerification] no last cross link found 2",
							"blockHash", block.Hash(),
							"crossLink", lastLink,
						).WithCause(err)
					}
				}
			} else {
				lastLink = &(*crossLinks)[i-1]
			}
		}

		if crossLink.BlockNum().Uint64() > firstCrossLinkBlock { // TODO: verify genesis block
			err = node.VerifyCrosslinkHeader(lastLink.Header(), crossLink.Header())
			if err != nil {
				return ctxerror.New("cannot ValidateNewBlock",
					"blockHash", block.Hash(),
					"numTx", len(block.Transactions()),
				).WithCause(err)
			}
		}
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
	shardState := &types.ShardState{}
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
// 3. [leader] send cross shard tx receipts to destination shard
func (node *Node) PostConsensusProcessing(newBlock *types.Block) {
	if err := node.AddNewBlock(newBlock); err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("Error when adding new block")
		return
	} else if core.IsEpochLastBlock(newBlock) {
		node.Consensus.UpdateConsensusInformation()
	}

	// Update last consensus time for metrics
	node.lastConsensusTime = time.Now().Unix()
	if node.Consensus.PubKey.IsEqual(node.Consensus.LeaderPubKey) {
		if node.NodeConfig.ShardID == 0 {
			node.BroadcastNewBlock(newBlock)
		} else {
			node.BroadcastCrossLinkHeader(newBlock)
		}
		node.BroadcastCXReceipts(newBlock)
	} else {
		utils.Logger().Info().
			Uint64("ViewID", node.Consensus.GetViewID()).
			Msg("BINGO !!! Reached Consensus")
	}

	node.Blockchain().CleanCXReceiptsCheckpointsByBlock(newBlock)

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
func (node *Node) AddNewBlock(newBlock *types.Block) error {
	_, err := node.Blockchain().InsertChain([]*types.Block{newBlock})
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Uint64("blockNum", newBlock.NumberU64()).
			Bytes("parentHash", newBlock.Header().ParentHash.Bytes()[:]).
			Bytes("hash", newBlock.Header().Hash().Bytes()[:]).
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

	if ping.Node.Role == proto_node.ClientRole {
		utils.Logger().Info().
			Str("Client", peer.String()).
			Msg("Add Client Peer to Node")
		node.ClientPeer = peer
	} else {
		node.AddPeers([]*p2p.Peer{peer})
		utils.Logger().Info().
			Str("Peer", peer.String()).
			Int("# Peers", len(node.Consensus.PublicKeys)).
			Msg("Add Peer to Node")
	}

	return 1
}

// bootstrapConsensus is the a goroutine to check number of peers and start the consensus
func (node *Node) bootstrapConsensus() {
	tick := time.NewTicker(5 * time.Second)
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
			}
			if numPeersNow >= node.Consensus.MinPeers {
				utils.Logger().Info().Msg("[bootstrap] StartConsensus")
				node.startConsensus <- struct{}{}
				return
			}
		}
	}
}

func (node *Node) epochShardStateMessageHandler(msgPayload []byte) error {
	epochShardState, err := proto_node.DeserializeEpochShardStateFromMessage(msgPayload)
	if err != nil {
		return ctxerror.New("Can't get shard state message").WithCause(err)
	}
	if node.Consensus == nil {
		return nil
	}
	receivedEpoch := big.NewInt(int64(epochShardState.Epoch))
	utils.Logger().Info().
		Int64("epoch", receivedEpoch.Int64()).
		Msg("received new shard state")

	node.nextShardState.master = epochShardState
	if node.Consensus.IsLeader() {
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

/*
func (node *Node) transitionIntoNextEpoch(shardState types.ShardState) {
	logger = logger.New(
		"blsPubKey", hex.EncodeToString(node.Consensus.PubKey.Serialize()),
		"curShard", node.Blockchain().ShardID(),
		"curLeader", node.Consensus.IsLeader())
	for _, c := range shardState {
		utils.Logger().Debug().
			Uint32("shardID", c.ShardID).
			Str("nodeList", c.NodeList).
         Msg("new shard information")
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
	//	node.DRand.UpdatePublicKeys(publicKeys)

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
*/

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
		utils.Logger().Error().
			Err(err).
			Str("file", execFile).
			Msg("Failed to get program path when restarting program")
	}
	utils.Logger().Info().
		Strs("args", args).
		Strs("env", os.Environ()).
		Msg("Restarting program")
	err = syscall.Exec(execFile, args, os.Environ())
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("Failed to restart program after resharding")
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
