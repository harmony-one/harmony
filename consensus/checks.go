package consensus

import (
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

// MaxBlockNumDiff limits the received block number to only 100 further from the current block number
const MaxBlockNumDiff = 100

func (consensus *Consensus) validatorSanityChecks(msg *msg_pb.Message) bool {
	if msg.GetConsensus() == nil {
		utils.Logger().Warn().Msg("[validatorSanityChecks] malformed message")
		return false
	}
	utils.Logger().Debug().
		Uint64("blockNum", msg.GetConsensus().BlockNum).
		Uint64("viewID", msg.GetConsensus().ViewId).
		Str("msgType", msg.Type.String()).
		Msg("[validatorSanityChecks] Checking new message")
	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		if err == shard.ErrValidNotInCommittee {
			utils.Logger().Info().
				Msg("sender key not in this slot's subcommittee")
		} else {
			utils.Logger().Error().Err(err).Msg("VerifySenderKey failed")
		}
		return false
	}
	consensus.pubKeyLock.Lock()
	defer consensus.pubKeyLock.Unlock()

	if !senderKey.IsEqual(consensus.LeaderPubKey) &&
		consensus.current.Mode() == Normal && !consensus.ignoreViewIDCheck {
		utils.Logger().Warn().Msgf(
			"[%s] SenderKey not match leader PubKey",
			msg.GetType().String(),
		)
		return false
	}

	if err := verifyMessageSig(senderKey, msg); err != nil {
		utils.Logger().Error().Err(err).Msg(
			"Failed to verify sender's signature",
		)
		return false
	}

	return true
}

func (consensus *Consensus) leaderSanityChecks(msg *msg_pb.Message) bool {
	if msg.GetConsensus() == nil {
		utils.Logger().Warn().Msg("[leaderSanityChecks] malformed message")
		return false
	}
	utils.Logger().Debug().
		Uint64("blockNum", msg.GetConsensus().BlockNum).
		Uint64("viewID", msg.GetConsensus().ViewId).
		Str("msgType", msg.Type.String()).
		Msg("[leaderSanityChecks] Checking new message")
	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		if err == shard.ErrValidNotInCommittee {
			utils.Logger().Info().Msgf(
				"[%s] sender key not in this slot's subcommittee",
				msg.GetType().String(),
			)
		} else {
			utils.Logger().Error().Err(err).Msgf(
				"[%s] verifySenderKey failed",
				msg.GetType().String(),
			)
		}
		return false
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.Logger().Error().Err(err).Msgf(
			"[%s] Failed to verify sender's signature",
			msg.GetType().String(),
		)
		return false
	}

	return true
}

func (consensus *Consensus) isRightBlockNumAndViewID(recvMsg *FBFTMessage,
) bool {
	if num := consensus.BlockNum(); recvMsg.ViewID != consensus.ViewID() ||
		recvMsg.BlockNum != num {
		utils.Logger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", num).
			Msg("[OnCommit] BlockNum/viewID not match")
		return false
	}
	return true
}

func (consensus *Consensus) onAnnounceSanityChecks(recvMsg *FBFTMessage) bool {
	consensus.infoMutex.Lock()
	defer consensus.infoMutex.Unlock()

	logMsgs := consensus.FBFTLog.GetMessagesByTypeSeqView(
		msg_pb.MessageType_ANNOUNCE, recvMsg.BlockNum, recvMsg.ViewID,
	)
	if len(logMsgs) > 0 {
		if logMsgs[0].BlockHash != recvMsg.BlockHash &&
			logMsgs[0].SenderPubkey.IsEqual(recvMsg.SenderPubkey) {
			utils.Logger().Debug().
				Str("logMsgBlockHash", logMsgs[0].BlockHash.Hex()).
				Uint64("recvMsg.BlockNum", recvMsg.BlockNum).
				Uint64("recvMsg.ViewID", recvMsg.ViewID).
				Str("recvMsgBlockHash", recvMsg.BlockHash.Hex()).
				Msg("[OnAnnounce] Leader is malicious")
			if consensus.current.Mode() == ViewChanging {
				utils.Logger().Debug().Msg(
					"[OnAnnounce] Already in ViewChanging mode, conflicing announce, doing noop",
				)
			} else {
				consensus.startViewChange(consensus.ViewID() + 1)
			}
		}
		utils.Logger().Debug().Msg("[OnAnnounce] Announce message received again")
	}
	return consensus.isRightBlockNumCheck(recvMsg)
}

func (consensus *Consensus) isRightBlockNumCheck(recvMsg *FBFTMessage) bool {

	if num := consensus.BlockNum(); recvMsg.BlockNum < num {
		utils.Logger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("Wrong BlockNum Received, ignoring!")
		return false
	} else if recvMsg.BlockNum-num > MaxBlockNumDiff {
		utils.Logger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("MaxBlockNumDiff", MaxBlockNumDiff).
			Msg("Received blockNum that is MaxBlockNumDiff further from the current blockNum!")
		return false
	}
	return true
}

func (consensus *Consensus) onPreparedSanityChecks(
	blockObj *types.Block, recvMsg *FBFTMessage,
) bool {

	if blockObj.NumberU64() != recvMsg.BlockNum ||
		recvMsg.BlockNum < consensus.BlockNum() {
		utils.Logger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", blockObj.NumberU64()).
			Msg("[OnPrepared] BlockNum not match")
		return false
	}
	if blockObj.Header().Hash() != recvMsg.BlockHash {
		utils.Logger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Hex("MsgBlockHash", recvMsg.BlockHash[:]).
			Str("blockObjHash", blockObj.Header().Hash().Hex()).
			Msg("[OnPrepared] BlockHash not match")
		return false
	}
	if consensus.current.Mode() == Normal {
		err := chain.Engine.VerifyHeader(consensus.ChainReader, blockObj.Header(), true)
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Str("inChain", consensus.ChainReader.CurrentHeader().Number().String()).
				Str("MsgBlockNum", blockObj.Header().Number().String()).
				Msg("[OnPrepared] Block header is not verified successfully")
			return false
		}

		resp := make(chan error)
		consensus.Verify.Request <- blkComeback{blockObj, resp}

		if err := <-resp; err != nil {
			utils.Logger().Error().Err(err).
				Msg("block verification failed in onprepared sanitychecks")
			return false
		}

	}

	return true
}

func (consensus *Consensus) viewChangeSanityCheck(msg *msg_pb.Message) bool {
	if msg.GetViewchange() == nil {
		utils.Logger().Warn().Msg("[viewChangeSanityCheck] malformed message")
		return false
	}
	utils.Logger().Debug().
		Msg("[viewChangeSanityCheck] Checking new message")
	senderKey, err := consensus.verifyViewChangeSenderKey(msg)
	if err != nil {
		if err == shard.ErrValidNotInCommittee {
			utils.Logger().Info().Msgf(
				"[%s] sender key not in this slot's subcommittee",
				msg.GetType().String(),
			)
		} else {
			utils.Logger().Error().Err(err).Msgf(
				"[%s] VerifySenderKey Failed",
				msg.GetType().String(),
			)
		}
		return false
	}
	if err := verifyMessageSig(senderKey, msg); err != nil {
		utils.Logger().Error().Err(err).Msgf(
			"[%s] Failed To Verify Sender's Signature",
			msg.GetType().String(),
		)
		return false
	}
	return true
}

func (consensus *Consensus) onViewChangeSanityCheck(recvMsg *FBFTMessage) bool {
	// TODO: if difference is only one, new leader can still propose the same committed block to avoid another view change
	// TODO: new leader catchup without ignore view change message
	num := consensus.BlockNum()
	if num > recvMsg.BlockNum {
		utils.Logger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onViewChange] Message BlockNum Is Low")
		return false
	}

	if num < recvMsg.BlockNum {
		utils.Logger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onViewChange] New Leader Has Lower Blocknum")
		return false
	}
	if consensus.current.Mode() == ViewChanging &&
		consensus.current.ViewID() > recvMsg.ViewID {
		utils.Logger().Warn().
			Uint64("MyViewChangingID", consensus.current.ViewID()).
			Uint64("MsgViewChangingID", recvMsg.ViewID).
			Msg("[onViewChange] ViewChanging ID Is Low")
		return false
	}
	if recvMsg.ViewID-consensus.current.ViewID() > MaxViewIDDiff {
		utils.Logger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("CurrentViewID", consensus.current.ViewID()).
			Msg("Received viewID that is MaxViewIDDiff (100) further from the current viewID!")
		return false
	}
	return true
}

func (consensus *Consensus) onNewViewSanityCheck(recvMsg *FBFTMessage) bool {
	if viewID := consensus.ViewID(); recvMsg.ViewID <= viewID {
		utils.Logger().Warn().
			Uint64("LastSuccessfulConsensusViewID", viewID).
			Uint64("MsgViewChangingID", recvMsg.ViewID).
			Msg("[onNewView] ViewID should be larger than the viewID of the last successful consensus")
		return false
	}
	if consensus.current.Mode() != ViewChanging {
		utils.Logger().Warn().
			Msg("[onNewView] Not in ViewChanging mode, ignoring the new view message")
		return false
	}
	return true
}
