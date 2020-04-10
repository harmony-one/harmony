package node

import (
	"encoding/binary"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

var once sync.Once

// ExplorerMessageHandler passes received message in node_handler to explorer service
func (node *Node) ExplorerMessageHandler(payload []byte) {
	if len(payload) == 0 {
		utils.Logger().Error().Msg("Payload is empty")
		return
	}
	msg := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, msg)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to unmarshal message payload.")
		return
	}

	if msg.Type == msg_pb.MessageType_COMMITTED {
		recvMsg, err := consensus.ParseFBFTMessage(msg)
		if err != nil {
			utils.Logger().Error().Err(err).
				Msg("[Explorer] onCommitted unable to parse msg")
			return
		}

		aggSig, mask, err := node.Consensus.ReadSignatureBitmapPayload(
			recvMsg.Payload, 0,
		)
		if err != nil {
			utils.Logger().Error().Err(err).
				Msg("[Explorer] readSignatureBitmapPayload failed")
			return
		}

		if !node.Consensus.Decider.IsQuorumAchievedByMask(mask) {
			utils.Logger().Error().Msg("[Explorer] not have enough signature power")
			return
		}

		// TODO(audit): verify signature on hash+blockNum+viewID (add a hard fork)
		blockNumHash := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumHash, recvMsg.BlockNum)
		commitPayload := append(blockNumHash, recvMsg.BlockHash[:]...)
		if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
			utils.Logger().
				Error().Err(err).
				Uint64("msgBlock", recvMsg.BlockNum).
				Msg("[Explorer] Failed to verify the multi signature for commit phase")
			return
		}

		block := node.Consensus.FBFTLog.GetBlockByHash(recvMsg.BlockHash)

		if block == nil {
			utils.Logger().Info().
				Uint64("msgBlock", recvMsg.BlockNum).
				Msg("[Explorer] Haven't received the block before the committed msg")
			node.Consensus.FBFTLog.AddMessage(recvMsg)
			return
		}

		node.AddNewBlockForExplorer(block)
		node.commitBlockForExplorer(block)
	} else if msg.Type == msg_pb.MessageType_PREPARED {

		recvMsg, err := consensus.ParseFBFTMessage(msg)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("[Explorer] Unable to parse Prepared msg")
			return
		}
		block, blockObj := recvMsg.Block, &types.Block{}
		if err := rlp.DecodeBytes(block, blockObj); err != nil {
			utils.Logger().Error().Err(err).Msg("explorer could not rlp decode block")
			return
		}
		// Add the block into FBFT log.
		node.Consensus.FBFTLog.AddBlock(blockObj)
		// Try to search for MessageType_COMMITTED message from pbft log.
		msgs := node.Consensus.FBFTLog.GetMessagesByTypeSeqHash(
			msg_pb.MessageType_COMMITTED,
			blockObj.NumberU64(),
			blockObj.Hash(),
		)
		// If found, then add the new block into blockchain db.
		if len(msgs) > 0 {
			node.AddNewBlockForExplorer(blockObj)
			node.commitBlockForExplorer(blockObj)
		}
	}
}

// AddNewBlockForExplorer add new block for explorer.
func (node *Node) AddNewBlockForExplorer(block *types.Block) {
	utils.Logger().Debug().Uint64("blockHeight", block.NumberU64()).Msg("[Explorer] Adding new block for explorer node")
	if _, err := node.Blockchain().InsertChain([]*types.Block{block}, true); err == nil {
		if len(block.Header().ShardState()) > 0 {
			node.Consensus.UpdateConsensusInformation()
		}
		// Clean up the blocks to avoid OOM.
		node.Consensus.FBFTLog.DeleteBlockByNumber(block.NumberU64())
		// Do dump all blocks from state syncing for explorer one time
		// TODO: some blocks can be dumped before state syncing finished.
		// And they would be dumped again here. Please fix it.
		once.Do(func() {
			utils.Logger().Info().Int64("starting height", int64(block.NumberU64())-1).
				Msg("[Explorer] Populating explorer data from state synced blocks")
			go func() {
				for blockHeight := int64(block.NumberU64()) - 1; blockHeight >= 0; blockHeight-- {
					explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port, true).Dump(
						node.Blockchain().GetBlockByNumber(uint64(blockHeight)), uint64(blockHeight))
				}
			}()
		})
	} else {
		utils.Logger().Error().Err(err).Msg("[Explorer] Error when adding new block for explorer node")
	}
}

// ExplorerMessageHandler passes received message in node_handler to explorer service.
func (node *Node) commitBlockForExplorer(block *types.Block) {
	if block.ShardID() != node.NodeConfig.ShardID {
		return
	}
	// Dump new block into level db.
	utils.Logger().Info().Uint64("blockNum", block.NumberU64()).Msg("[Explorer] Committing block into explorer DB")
	explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port, true).Dump(block, block.NumberU64())

	curNum := block.NumberU64()
	if curNum-100 > 0 {
		node.Consensus.FBFTLog.DeleteBlocksLessThan(curNum - 100)
		node.Consensus.FBFTLog.DeleteMessagesLessThan(curNum - 100)
	}
}

// GetTransactionsHistory returns list of transactions hashes of address.
func (node *Node) GetTransactionsHistory(address, txType, order string) ([]common.Hash, error) {
	addressData := &explorer.Address{}
	key := explorer.GetAddressKey(address)
	bytes, err := explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port, false).GetDB().Get([]byte(key), nil)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[Explorer] Cannot get storage db instance")
		return make([]common.Hash, 0), nil
	}
	if err = rlp.DecodeBytes(bytes, &addressData); err != nil {
		utils.Logger().Error().Err(err).Msg("[Explorer] Cannot convert address data from DB")
		return nil, err
	}
	if order == "DESC" {
		sort.Slice(addressData.TXs[:], func(i, j int) bool {
			return addressData.TXs[i].Timestamp > addressData.TXs[j].Timestamp
		})
	} else {
		sort.Slice(addressData.TXs[:], func(i, j int) bool {
			return addressData.TXs[i].Timestamp < addressData.TXs[j].Timestamp
		})
	}
	hashes := make([]common.Hash, 0)
	for _, tx := range addressData.TXs {
		if txType == "" || txType == "ALL" || txType == tx.Type {
			hash := common.HexToHash(tx.ID)
			hashes = append(hashes, hash)
		}
	}
	return hashes, nil
}

// GetStakingTransactionsHistory returns list of staking transactions hashes of address.
func (node *Node) GetStakingTransactionsHistory(address, txType, order string) ([]common.Hash, error) {
	addressData := &explorer.Address{}
	key := explorer.GetAddressKey(address)
	bytes, err := explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port, false).GetDB().Get([]byte(key), nil)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[Explorer] Cannot get storage db instance")
		return make([]common.Hash, 0), nil
	}
	if err = rlp.DecodeBytes(bytes, &addressData); err != nil {
		utils.Logger().Error().Err(err).Msg("[Explorer] Cannot convert address data from DB")
		return nil, err
	}
	if order == "DESC" {
		sort.Slice(addressData.StakingTXs[:], func(i, j int) bool {
			return addressData.StakingTXs[i].Timestamp > addressData.StakingTXs[j].Timestamp
		})
	} else {
		sort.Slice(addressData.StakingTXs[:], func(i, j int) bool {
			return addressData.StakingTXs[i].Timestamp < addressData.StakingTXs[j].Timestamp
		})
	}
	hashes := make([]common.Hash, 0)
	for _, tx := range addressData.StakingTXs {
		if txType == "" || txType == "ALL" || txType == tx.Type {
			hash := common.HexToHash(tx.ID)
			hashes = append(hashes, hash)
		}
	}
	return hashes, nil
}

// GetTransactionsCount returns the number of regular transactions hashes of address for input type.
func (node *Node) GetTransactionsCount(address, txType string) (uint64, error) {
	addressData := &explorer.Address{}
	key := explorer.GetAddressKey(address)
	bytes, err := explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port, false).GetDB().Get([]byte(key), nil)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[Explorer] Cannot get storage db instance")
		return 0, nil
	}
	if err = rlp.DecodeBytes(bytes, &addressData); err != nil {
		utils.Logger().Error().Err(err).Msg("[Explorer] Cannot convert address data from DB")
		return 0, err
	}

	count := uint64(0)
	for _, tx := range addressData.TXs {
		if txType == "" || txType == "ALL" || txType == tx.Type {
			count++
		}
	}
	return count, nil
}

// GetStakingTransactionsCount returns the number of staking transactions hashes of address for input type.
func (node *Node) GetStakingTransactionsCount(address, txType string) (uint64, error) {
	addressData := &explorer.Address{}
	key := explorer.GetAddressKey(address)
	bytes, err := explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port, false).GetDB().Get([]byte(key), nil)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[Explorer] Cannot get storage db instance")
		return 0, nil
	}
	if err = rlp.DecodeBytes(bytes, &addressData); err != nil {
		utils.Logger().Error().Err(err).Msg("[Explorer] Cannot convert address data from DB")
		return 0, err
	}

	count := uint64(0)
	for _, tx := range addressData.StakingTXs {
		if txType == "" || txType == "ALL" || txType == tx.Type {
			count++
		}
	}
	return count, nil
}
