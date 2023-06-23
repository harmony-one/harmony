package node

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/harmony-one/harmony/internal/tikv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core"
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
		recvMsg, err := node.Consensus.ParseFBFTMessage(msg)
		if err != nil {
			utils.Logger().Error().Err(err).
				Msg("[Explorer] onCommitted unable to parse msg")
			return err
		}

		aggSig, mask, err := node.Consensus.ReadSignatureBitmapPayload(
			recvMsg.Payload, 0,
		)
		if err != nil {
			utils.Logger().Error().Err(err).
				Msg("[Explorer] readSignatureBitmapPayload failed")
			return err
		}

		if !node.Consensus.Decider.IsQuorumAchievedByMask(mask) {
			utils.Logger().Error().Msg("[Explorer] not have enough signature power")
			return nil
		}

		block := node.Consensus.FBFTLog.GetBlockByHash(recvMsg.BlockHash)

		if block == nil {
			utils.Logger().Info().
				Uint64("msgBlock", recvMsg.BlockNum).
				Msg("[Explorer] Haven't received the block before the committed msg")
			node.Consensus.FBFTLog.AddVerifiedMessage(recvMsg)
			return errBlockBeforeCommit
		}

		commitPayload := signature.ConstructCommitPayload(node.Blockchain().Config(),
			block.Epoch(), block.Hash(), block.Number().Uint64(), block.Header().ViewID().Uint64())
		if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
			utils.Logger().
				Error().Err(err).
				Uint64("msgBlock", recvMsg.BlockNum).
				Msg("[Explorer] Failed to verify the multi signature for commit phase")
			return errFailVerifyMultiSign
		}

		block.SetCurrentCommitSig(recvMsg.Payload)
		node.AddNewBlockForExplorer(block)
		node.commitBlockForExplorer(block)
	} else if msg.Type == msg_pb.MessageType_PREPARED {

		recvMsg, err := node.Consensus.ParseFBFTMessage(msg)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("[Explorer] Unable to parse Prepared msg")
			return err
		}
		block, blockObj := recvMsg.Block, &types.Block{}
		if err := rlp.DecodeBytes(block, blockObj); err != nil {
			utils.Logger().Error().Err(err).Msg("explorer could not rlp decode block")
			return err
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
			var committedMsg *consensus.FBFTMessage
			for i := range msgs {
				if blockObj.Hash() != msgs[i].BlockHash {
					continue
				}
				committedMsg = msgs[i]
				break
			}
			if committedMsg == nil {
				utils.Logger().Error().Err(err).Msg("[Explorer] Failed finding a valid committed message.")
				return errFailFindingValidCommit
			}
			blockObj.SetCurrentCommitSig(committedMsg.Payload)
			node.AddNewBlockForExplorer(blockObj)
			node.commitBlockForExplorer(blockObj)
		}
	}
	return nil
}

func (node *Node) TraceLoopForExplorer() {
	if !node.HarmonyConfig.General.TraceEnable {
		return
	}
	ch := make(chan core.TraceEvent)
	subscribe := node.Blockchain().SubscribeTraceEvent(ch)
	go func() {
	loop:
		select {
		case ev := <-ch:
			if exp, err := node.getExplorerService(); err == nil {
				storage := ev.Tracer.GetStorage()
				exp.DumpTraceResult(storage)
			}
			goto loop
		case <-subscribe.Err():
			//subscribe.Unsubscribe()
			break
		}
	}()
}

// AddNewBlockForExplorer add new block for explorer.
func (node *Node) AddNewBlockForExplorer(block *types.Block) {
	if node.HarmonyConfig.General.RunElasticMode && node.HarmonyConfig.TiKV.Role == tikv.RoleReader {
		node.Consensus.DeleteBlockByNumber(block.NumberU64())
		return
	}

	utils.Logger().Info().Uint64("blockHeight", block.NumberU64()).Msg("[Explorer] Adding new block for explorer node")

	if _, err := node.Blockchain().InsertChain([]*types.Block{block}, false); err == nil {
		if block.IsLastBlockInEpoch() {
			node.Consensus.UpdateConsensusInformation()
		}
		// Clean up the blocks to avoid OOM.
		node.Consensus.DeleteBlockByNumber(block.NumberU64())

		// if in tikv mode, only master writer node need dump all explorer block
		if !node.HarmonyConfig.General.RunElasticMode || node.Blockchain().IsTikvWriterMaster() {
			// Do dump all blocks from state syncing for explorer one time
			// TODO: some blocks can be dumped before state syncing finished.
			// And they would be dumped again here. Please fix it.
			once.Do(func() {
				utils.Logger().Info().Int64("starting height", int64(block.NumberU64())-1).
					Msg("[Explorer] Populating explorer data from state synced blocks")
				go func() {
					exp, err := node.getExplorerService()
					if err != nil {
						// shall be unreachable
						utils.Logger().Fatal().Err(err).Msg("critical error in explorer node")
					}

					if block.NumberU64() == 0 {
						return
					}

					// get checkpoint bitmap and flip all bit
					bitmap := exp.GetCheckpointBitmap()
					bitmap.Flip(0, block.NumberU64())

					// find all not processed block and dump it
					iterator := bitmap.ReverseIterator()
					for iterator.HasNext() {
						exp.DumpCatchupBlock(node.Blockchain().GetBlockByNumber(iterator.Next()))
					}
				}()
			})
		}
	} else {
		utils.Logger().Error().Err(err).Msg("[Explorer] Error when adding new block for explorer node")
	}
}

// ExplorerMessageHandler passes received message in node_handler to explorer service.
func (node *Node) commitBlockForExplorer(block *types.Block) {
	// if in tikv mode, only master writer node need dump explorer block
	if !node.HarmonyConfig.General.RunElasticMode || (node.HarmonyConfig.TiKV.Role == tikv.RoleWriter && node.Blockchain().IsTikvWriterMaster()) {
		if block.ShardID() != node.NodeConfig.ShardID {
			return
		}
		// Dump new block into level db.
		utils.Logger().Info().Uint64("blockNum", block.NumberU64()).Msg("[Explorer] Committing block into explorer DB")
		exp, err := node.getExplorerService()
		if err != nil {
			// shall be unreachable
			utils.Logger().Fatal().Err(err).Msg("critical error in explorer node")
		}
		exp.DumpNewBlock(block)
	}

	curNum := block.NumberU64()
	if curNum-100 > 0 {
		node.Consensus.DeleteBlocksLessThan(curNum - 100)
		node.Consensus.DeleteMessagesLessThan(curNum - 100)
	}
}

// GetTransactionsHistory returns list of transactions hashes of address.
func (node *Node) GetTransactionsHistory(address, txType, order string) ([]common.Hash, error) {
	exp, err := node.getExplorerService()
	if err != nil {
		return nil, err
	}
	allTxs, tts, err := exp.GetNormalTxHashesByAccount(address)
	if err != nil {
		return nil, err
	}
	txs := getTargetTxHashes(allTxs, tts, txType)

	if order == "DESC" {
		reverseTxs(txs)
	}
	return txs, nil
}

// GetStakingTransactionsHistory returns list of staking transactions hashes of address.
func (node *Node) GetStakingTransactionsHistory(address, txType, order string) ([]common.Hash, error) {
	exp, err := node.getExplorerService()
	if err != nil {
		return nil, err
	}
	allTxs, tts, err := exp.GetStakingTxHashesByAccount(address)
	if err != nil {
		return nil, err
	}
	txs := getTargetTxHashes(allTxs, tts, txType)

	if order == "DESC" {
		reverseTxs(txs)
	}
	return txs, nil
}

// GetTransactionsCount returns the number of regular transactions hashes of address for input type.
func (node *Node) GetTransactionsCount(address, txType string) (uint64, error) {
	exp, err := node.getExplorerService()
	if err != nil {
		return 0, err
	}
	_, tts, err := exp.GetNormalTxHashesByAccount(address)
	if err != nil {
		return 0, err
	}
	count := uint64(0)
	for _, tt := range tts {
		if isTargetTxType(tt, txType) {
			count++
		}
	}
	return count, nil
}

// GetStakingTransactionsCount returns the number of staking transactions hashes of address for input type.
func (node *Node) GetStakingTransactionsCount(address, txType string) (uint64, error) {
	exp, err := node.getExplorerService()
	if err != nil {
		return 0, err
	}
	_, tts, err := exp.GetStakingTxHashesByAccount(address)
	if err != nil {
		return 0, err
	}
	count := uint64(0)
	for _, tt := range tts {
		if isTargetTxType(tt, txType) {
			count++
		}
	}
	return count, nil
}

// GetStakingTransactionsCount returns the number of staking transactions hashes of address for input type.
func (node *Node) GetTraceResultByHash(hash common.Hash) (json.RawMessage, error) {
	exp, err := node.getExplorerService()
	if err != nil {
		return nil, err
	}
	return exp.GetTraceResultByHash(hash)
}

func (node *Node) getExplorerService() (*explorer.Service, error) {
	rawService := node.serviceManager.GetService(service.SupportExplorer)
	if rawService == nil {
		return nil, errors.New("explorer service not started")
	}
	return rawService.(*explorer.Service), nil
}

func isTargetTxType(tt explorer.TxType, target string) bool {
	return target == "" || target == "ALL" || target == tt.String()
}

func getTargetTxHashes(txs []common.Hash, tts []explorer.TxType, target string) []common.Hash {
	var res []common.Hash
	for i, tx := range txs {
		if isTargetTxType(tts[i], target) {
			res = append(res, tx)
		}
	}
	return res
}

func reverseTxs(txs []common.Hash) {
	for i := 0; i < len(txs)/2; i++ {
		j := len(txs) - i - 1
		txs[i], txs[j] = txs[j], txs[i]
	}
}
