package node

import (
	"context"
	"sort"
	"sync"

	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/chain"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/pkg/errors"
)

var once sync.Once

var (
	errBlockBeforeCommit = errors.New(
		"explorer hasnt received the block before the committed msg",
	)
	errFailVerifyMultiSign = errors.New(
		"explorer failed to verify the multi signature for commit phase",
	)
	errFailFindingValidCommit = errors.New(
		"explorer failed finding a valid committed message",
	)
)

// explorerMessageHandler passes received message in node_handler to explorer service
func (node *Node) explorerMessageHandler(ctx context.Context, msg *msg_pb.Message) error {
	if msg.Type == msg_pb.MessageType_COMMITTED {
		parsedMsg, err := node.Consensus.ParseFBFTMessage(msg)
		if err != nil {
			return errors.New("failed to parse FBFT message")
		}

		node.Consensus.Mutex.Lock()
		defer node.Consensus.Mutex.Unlock()

		if err := node.explorerHelper.verifyCommittedMsg(parsedMsg); err != nil {
			if err == errBlockNotReady {
				utils.Logger().Info().Uint64("block number", parsedMsg.BlockNum).
					Str("blockHash", parsedMsg.BlockHash.Hex()).
					Msg("committed message received before prepared")
				return nil
			}
			return errors.Wrap(err, "verify committed message for explorer")
		}
		if err := node.explorerHelper.tryCatchup(); err != nil {
			return errors.Wrap(err, "failed to catchup for explorer")
		}
	} else if msg.Type == msg_pb.MessageType_PREPARED {
		parsedMsg, err := node.Consensus.ParseFBFTMessage(msg)
		if err != nil {
			return errors.New("failed to parse FBFT message")
		}

		node.Consensus.Mutex.Lock()
		defer node.Consensus.Mutex.Unlock()

		if err := node.explorerHelper.verifyPreparedMsg(parsedMsg); err != nil {
			return errors.Wrap(err, "verify prepared message for explorer")
		}
		if err := node.explorerHelper.tryCatchup(); err != nil {
			return errors.Wrap(err, "failed to catchup for explorer")
		}
	}
	return nil
}

// AddNewBlockForExplorer add new block for explorer.
func (node *Node) AddNewBlockForExplorer(block *types.Block) {
	utils.Logger().Info().Uint64("blockHeight", block.NumberU64()).Msg("[Explorer] Adding new block for explorer node")
	if _, err := node.Blockchain().InsertChain([]*types.Block{block}, false); err == nil {
		if block.IsLastBlockInEpoch() {
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
					explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port).Dump(
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
	explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port).Dump(block, block.NumberU64())

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
	bytes, err := explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port).GetDB().Get([]byte(key), nil)
	if err != nil {
		utils.Logger().Debug().Err(err).
			Msgf("[Explorer] Error retrieving transaction history for address %s", address)
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
			hash := common.HexToHash(tx.Hash)
			hashes = append(hashes, hash)
		}
	}
	return hashes, nil
}

// GetStakingTransactionsHistory returns list of staking transactions hashes of address.
func (node *Node) GetStakingTransactionsHistory(address, txType, order string) ([]common.Hash, error) {
	addressData := &explorer.Address{}
	key := explorer.GetAddressKey(address)
	bytes, err := explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port).GetDB().Get([]byte(key), nil)
	if err != nil {
		utils.Logger().Debug().Err(err).
			Msgf("[Explorer] Staking transaction history for address %s not found", address)
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
			hash := common.HexToHash(tx.Hash)
			hashes = append(hashes, hash)
		}
	}
	return hashes, nil
}

// GetTransactionsCount returns the number of regular transactions hashes of address for input type.
func (node *Node) GetTransactionsCount(address, txType string) (uint64, error) {
	addressData := &explorer.Address{}
	key := explorer.GetAddressKey(address)
	bytes, err := explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port).GetDB().Get([]byte(key), nil)
	if err != nil {
		utils.Logger().Error().Err(err).Str("addr", address).Msg("[Explorer] Address not found")
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
	bytes, err := explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port).GetDB().Get([]byte(key), nil)
	if err != nil {
		utils.Logger().Error().Err(err).Str("addr", address).Msg("[Explorer] Address not found")
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

var errBlockNotReady = errors.New("block not ready for committed message")

type explorerHelper struct {
	bc     *core.BlockChain
	n      *Node
	engine engine.Engine
	c      *consensus.Consensus
}

func newExplorerHelper(n *Node) *explorerHelper {
	return &explorerHelper{
		bc:     n.Blockchain(),
		n:      n,
		engine: n.Blockchain().Engine(),
		c:      n.Consensus,
	}
}

// TODO: this is the copied code from c.OnCommitted. Refactor later.
func (eh *explorerHelper) verifyCommittedMsg(recvMsg *consensus.FBFTMessage) error {
	if recvMsg.BlockNum <= eh.bc.CurrentBlock().NumberU64() {
		return errors.New("stale committed message received from pub-sub")
	}
	if curNumber := eh.bc.CurrentBlock().NumberU64(); recvMsg.BlockNum > curNumber+1 {
		// TODO: also add trigger to stream downloader module
		utils.Logger().Info().Uint64("received number", recvMsg.BlockNum).
			Uint64("current number", curNumber).Msg("sync spin up on explorer committed")
		select {
		case eh.c.BlockNumLowChan <- struct{}{}:
		default:
		}
	}
	eh.c.FBFTLog.AddNotVerifiedMessage(recvMsg)

	blockObj := eh.c.FBFTLog.GetBlockByHash(recvMsg.BlockHash)
	if blockObj == nil {
		utils.Logger().Info().Uint64("blockNum", recvMsg.BlockNum).
			Str("blockHash", recvMsg.BlockHash.Hex()).
			Msg("block not ready when handling committed message")
		return errBlockNotReady
	}
	sigBytes, bitmap, err := chain.ParseCommitSigAndBitmap(recvMsg.Payload)
	if err != nil {
		return errors.Wrap(err, "unable to read signature bitmap payload")
	}
	if err := eh.c.Blockchain.Engine().VerifyHeaderSignature(eh.bc, blockObj.Header(),
		sigBytes, bitmap); err != nil {
		utils.Logger().Error().Uint64("blockNum", recvMsg.BlockNum).
			Str("blockHash", recvMsg.BlockHash.Hex()).
			Msg("Failed to verify the multi signature for commit phase")
		return err
	}
	eh.c.FBFTLog.AddVerifiedMessage(recvMsg)
	return nil
}

// TODO: this is the copied code from c.OnPrepared. Refactor later.
func (eh *explorerHelper) verifyPreparedMsg(recvMsg *consensus.FBFTMessage) error {
	var blockObj *types.Block
	if err := rlp.DecodeBytes(recvMsg.Block, &blockObj); err != nil {
		return err
	}
	if blockObj.NumberU64() <= eh.bc.CurrentBlock().NumberU64() {
		return errors.New("stale prepared message received from pub-sub")
	}
	if curNumber := eh.bc.CurrentBlock().NumberU64(); blockObj.NumberU64() > curNumber+1 {
		// TODO: also add trigger to stream downloader module
		utils.Logger().Info().Uint64("received number", recvMsg.BlockNum).
			Uint64("current number", curNumber).Msg("sync spin up on explorer prepared")
		select {
		case eh.c.BlockNumLowChan <- struct{}{}:
		default:
		}
	}

	eh.c.FBFTLog.AddBlock(blockObj)

	msgs := eh.c.FBFTLog.GetMessagesByTypeSeqHash(
		msg_pb.MessageType_COMMITTED,
		blockObj.NumberU64(),
		blockObj.Hash(),
	)
	// If found, then add the new block into blockchain db.
	if len(msgs) > 0 {
		var committedMsg *consensus.FBFTMessage
		for i := range msgs {
			if blockObj.Hash() != msgs[i].BlockHash {
				continue
			}
			committedMsg = msgs[i]
			break
		}
		if committedMsg == nil {
			return nil
		}
		if err := eh.verifyCommittedMsg(committedMsg); err != nil {
			return errors.Wrap(err, "failed to verify committed message as paired prepared msg")
		}
	}
	return nil
}

func (eh *explorerHelper) tryCatchup() error {
	initBN := eh.bc.CurrentBlock().NumberU64() + 1
	blks, msgs, err := eh.c.GetLastMileBlocksAndMsg(initBN)
	if err != nil {
		return errors.Wrapf(err, "Failed to get last mile blocks")
	}
	for i := range blks {
		blk, msg := blks[i], msgs[i]
		if blk == nil {
			return nil
		}
		if !msg.Verified {
			if err := eh.verifyCommittedMsg(msg); err != nil {
				return errors.New("failed to verify committed message")
			}
		}
		blk.SetCurrentCommitSig(msg.Payload)
		eh.n.AddNewBlockForExplorer(blk)
		eh.n.commitBlockForExplorer(blk)
	}
	return nil
}
