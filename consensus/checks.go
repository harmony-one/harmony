package consensus

import (
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

// MaxBlockNumDiff limits the received block number to only 100 further from the current block number
const MaxBlockNumDiff = 100

func (consensus *Consensus) validatorSanityChecks(msg *msg_pb.Message) bool {
	if msg.GetConsensus() == nil {
		consensus.getLogger().Warn().Msg("[validatorSanityChecks] malformed message")
		return false
	}
	consensus.getLogger().Debug().
		Uint64("blockNum", msg.GetConsensus().BlockNum).
		Uint64("viewID", msg.GetConsensus().ViewId).
		Str("msgType", msg.Type.String()).
		Msg("[validatorSanityChecks] Checking new message")
	err := consensus.verifySenderKey(msg)
	if err != nil {
		if err == shard.ErrValidNotInCommittee {
			utils.SampledLogger().Info().
				Hex("senderKey", msg.GetConsensus().SenderPubkey).
				Msg("sender key not in this slot's subcommittee")
		} else {
			consensus.getLogger().Error().Err(err).Msg("VerifySenderKey failed")
		}
		return false
	}
	senderKey, err := bls.BytesToBLSPublicKey(msg.GetConsensus().SenderPubkey)
	if err != nil {
		return false
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) &&
		consensus.current.Mode() == Normal && !consensus.IgnoreViewIDCheck.IsSet() {
		consensus.getLogger().Warn().Msgf(
			"[%s] SenderKey not match leader PubKey",
			msg.GetType().String(),
		)
		return false
	}

	if err := verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Error().Err(err).Msg(
			"Failed to verify sender's signature",
		)
		return false
	}

	return true
}

func (consensus *Consensus) leaderSanityChecks(msg *msg_pb.Message) bool {
	if msg.GetConsensus() == nil {
		consensus.getLogger().Warn().Msg("[leaderSanityChecks] malformed message")
		return false
	}
	consensus.getLogger().Debug().
		Uint64("blockNum", msg.GetConsensus().BlockNum).
		Uint64("viewID", msg.GetConsensus().ViewId).
		Str("msgType", msg.Type.String()).
		Msg("[leaderSanityChecks] Checking new message")
	err := consensus.verifySenderKey(msg)
	if err != nil {
		if err == shard.ErrValidNotInCommittee {
			consensus.getLogger().Info().
				Hex("senderKey", msg.GetConsensus().SenderPubkey).Msgf(
				"[%s] sender key not in this slot's subcommittee",
				msg.GetType().String(),
			)
		} else {
			consensus.getLogger().Error().Err(err).Msgf(
				"[%s] verifySenderKey failed",
				msg.GetType().String(),
			)
		}
		return false
	}
	senderKey, err := bls.BytesToBLSPublicKey(msg.GetConsensus().SenderPubkey)
	if err != nil {
		return false
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Error().Err(err).Msgf(
			"[%s] Failed to verify sender's signature",
			msg.GetType().String(),
		)
		return false
	}

	return true
}

func (consensus *Consensus) isRightBlockNumAndViewID(recvMsg *FBFTMessage,
) bool {
	if recvMsg.ViewID != consensus.viewID || recvMsg.BlockNum != consensus.blockNum {
		consensus.getLogger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Str("ValidatorPubKey", recvMsg.SenderPubkey.SerializeToHexStr()).
			Msg("[OnCommit] BlockNum/viewID not match")
		return false
	}
	return true
}

func (consensus *Consensus) onAnnounceSanityChecks(recvMsg *FBFTMessage) bool {
	logMsgs := consensus.FBFTLog.GetMessagesByTypeSeqView(
		msg_pb.MessageType_ANNOUNCE, recvMsg.BlockNum, recvMsg.ViewID,
	)
	if len(logMsgs) > 0 {
		if logMsgs[0].BlockHash != recvMsg.BlockHash &&
			logMsgs[0].SenderPubkey.IsEqual(recvMsg.SenderPubkey) {
			consensus.getLogger().Debug().
				Str("logMsgSenderKey", logMsgs[0].SenderPubkey.SerializeToHexStr()).
				Str("logMsgBlockHash", logMsgs[0].BlockHash.Hex()).
				Str("recvMsg.SenderPubkey", recvMsg.SenderPubkey.SerializeToHexStr()).
				Uint64("recvMsg.BlockNum", recvMsg.BlockNum).
				Uint64("recvMsg.ViewID", recvMsg.ViewID).
				Str("recvMsgBlockHash", recvMsg.BlockHash.Hex()).
				Str("LeaderKey", consensus.LeaderPubKey.SerializeToHexStr()).
				Msg("[OnAnnounce] Leader is malicious")
			if consensus.current.Mode() == ViewChanging {
				consensus.getLogger().Debug().Msg(
					"[OnAnnounce] Already in ViewChanging mode, conflicing announce, doing noop",
				)
			} else {
				consensus.startViewChange(consensus.viewID + 1)
			}
		}
		consensus.getLogger().Debug().
			Str("leaderKey", consensus.LeaderPubKey.SerializeToHexStr()).
			Msg("[OnAnnounce] Announce message received again")
	}
	return consensus.isRightBlockNumCheck(recvMsg)
}

func (consensus *Consensus) isRightBlockNumCheck(recvMsg *FBFTMessage) bool {
	if recvMsg.BlockNum < consensus.blockNum {
		consensus.getLogger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("Wrong BlockNum Received, ignoring!")
		return false
	} else if recvMsg.BlockNum-consensus.blockNum > MaxBlockNumDiff {
		consensus.getLogger().Debug().
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
		recvMsg.BlockNum < consensus.blockNum {
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", blockObj.NumberU64()).
			Msg("[OnPrepared] BlockNum not match")
		return false
	}
	if blockObj.Header().Hash() != recvMsg.BlockHash {
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Hex("MsgBlockHash", recvMsg.BlockHash[:]).
			Str("blockObjHash", blockObj.Header().Hash().Hex()).
			Msg("[OnPrepared] BlockHash not match")
		return false
	}
	if consensus.current.Mode() == Normal {
		err := chain.Engine.VerifyHeader(consensus.ChainReader, blockObj.Header(), true)
		if err != nil {
			consensus.getLogger().Error().
				Err(err).
				Str("inChain", consensus.ChainReader.CurrentHeader().Number().String()).
				Str("MsgBlockNum", blockObj.Header().Number().String()).
				Msg("[OnPrepared] Block header is not verified successfully")
			return false
		}
		if consensus.BlockVerifier == nil {
			// do nothing
		} else if err := consensus.BlockVerifier(blockObj); err != nil {
			consensus.getLogger().Error().Err(err).Msg("[OnPrepared] Block verification failed")
			return false
		}
	}

	return true
}

func (consensus *Consensus) viewChangeSanityCheck(msg *msg_pb.Message) bool {
	if msg.GetViewchange() == nil {
		consensus.getLogger().Warn().Msg("[viewChangeSanityCheck] malformed message")
		return false
	}
	consensus.getLogger().Debug().
		Msg("[viewChangeSanityCheck] Checking new message")
	senderKey, err := consensus.verifyViewChangeSenderKey(msg)
	if err != nil {
		if err == shard.ErrValidNotInCommittee {
			consensus.getLogger().Info().
				Hex("senderKey", msg.GetViewchange().SenderPubkey).Msgf(
				"[%s] sender key not in this slot's subcommittee",
				msg.GetType().String(),
			)
		} else {
			consensus.getLogger().Error().Err(err).Msgf(
				"[%s] VerifySenderKey Failed",
				msg.GetType().String(),
			)
		}
		return false
	}
	if err := verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Error().Err(err).Msgf(
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
	if consensus.blockNum > recvMsg.BlockNum {
		consensus.getLogger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onViewChange] Message BlockNum Is Low")
		return false
	}
	if consensus.blockNum < recvMsg.BlockNum {
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onViewChange] New Leader Has Lower Blocknum")
		return false
	}
	if consensus.current.Mode() == ViewChanging &&
		consensus.current.ViewID() > recvMsg.ViewID {
		consensus.getLogger().Warn().
			Uint64("MyViewChangingID", consensus.current.ViewID()).
			Uint64("MsgViewChangingID", recvMsg.ViewID).
			Msg("[onViewChange] ViewChanging ID Is Low")
		return false
	}
	if recvMsg.ViewID-consensus.current.ViewID() > MaxViewIDDiff {
		consensus.getLogger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("CurrentViewID", consensus.current.ViewID()).
			Msg("Received viewID that is MaxViewIDDiff (100) further from the current viewID!")
		return false
	}
	return true
}

func (consensus *Consensus) onNewViewSanityCheck(recvMsg *FBFTMessage) bool {
	if recvMsg.ViewID <= consensus.viewID {
		consensus.getLogger().Warn().
			Uint64("LastSuccessfulConsensusViewID", consensus.viewID).
			Uint64("MsgViewChangingID", recvMsg.ViewID).
			Msg("[onNewView] ViewID should be larger than the viewID of the last successful consensus")
		return false
	}
	if consensus.current.Mode() != ViewChanging {
		consensus.getLogger().Warn().
			Msg("[onNewView] Not in ViewChanging mode, ignoring the new view message")
		return false
	}
	return true
}
