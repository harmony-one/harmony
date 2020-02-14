package consensus

import (
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/shard"
)

func (consensus *Consensus) validatorSanityChecks(msg *msg_pb.Message) bool {
	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		if err == shard.ErrValidNotInCommittee {
			consensus.getLogger().Info().
				Msg("sender key not in this slot's subcommittee")
		} else {
			consensus.getLogger().Error().Err(err).Msg("VerifySenderKey failed")
		}
		return false
	}

	if !senderKey.IsEqual(consensus.LeaderPubKey) &&
		consensus.current.Mode() == Normal && !consensus.ignoreViewIDCheck {
		consensus.getLogger().Warn().Msg("[OnPrepared] SenderKey not match leader PubKey")
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
	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		if err == shard.ErrValidNotInCommittee {
			consensus.getLogger().Info().Msg(
				"[OnAnnounce] sender key not in this slot's subcommittee",
			)
		} else {
			consensus.getLogger().Error().Err(err).Msg("[OnAnnounce] erifySenderKey failed")
		}
		return false
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Error().Err(err).Msg(
			"[OnPrepare] Failed to verify sender's signature",
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

func (consensus *Consensus) couldThisBeADoubleSigner(
	recvMsg *FBFTMessage,
) bool {
	num, hash, now := consensus.blockNum, recvMsg.BlockHash, consensus.blockNum
	suspicious := !consensus.FBFTLog.HasMatchingAnnounce(num, hash) ||
		!consensus.FBFTLog.HasMatchingPrepared(num, hash)
	if suspicious {
		consensus.getLogger().Debug().
			Str("message", recvMsg.String()).
			Uint64("block-on-consensus", now).
			Msg("possible double signer")
		return true
	}
	return false
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
				viewID := consensus.current.ViewID()
				consensus.startViewChange(viewID + 1)
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
