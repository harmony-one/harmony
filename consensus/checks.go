package consensus

import (
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/pkg/errors"
)

// MaxBlockNumDiff limits the received block number to only 100 further from the current block number
const MaxBlockNumDiff = 100

func (consensus *Consensus) validatorSanityChecks(msg *msg_pb.Message) bool {
	consensus.getLogger().Debug().
		Uint64("blockNum", msg.GetConsensus().BlockNum).
		Uint64("viewID", msg.GetConsensus().ViewId).
		Str("msgType", msg.Type.String()).
		Msg("[validatorSanityChecks] Checking new message")

	// TODO can remove this as already verified but later below is some logic
	// NOTE can assume this won't explode, already past validation
	senderKey, err := bls_cosi.BytesToBLSPublicKey(
		msg.GetConsensus().GetSenderPubkey(),
	)

	if err != nil {
		return false
	}

	if !senderKey.IsEqual(consensus.LeaderPubKey) &&
		consensus.current.Mode() == Normal && !consensus.ignoreViewIDCheck {
		consensus.getLogger().Warn().Msgf(
			"[%s] SenderKey not match leader PubKey",
			msg.GetType().String(),
		)
		return false
	}

	return true
}

func (consensus *Consensus) isRightBlockNumAndViewID(
	recvMsg *FBFTMessage,
) error {
	if recvMsg.ViewID != consensus.viewID {
		return errViewIDCheckFail
	}
	if recvMsg.BlockNum != consensus.blockNum {
		return errWrongBlockNum
	}
	return nil
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

var (
	errLowBlockNum          = errors.New("message blockNum is low")
	errNewLeaderLowBlockNum = errors.New("new leader has lower blocknum")
	errViewChangeIDLow      = errors.New("viewChanging ID Is Low")
	errVCIDDiffToLarge      = errors.New(
		"received viewID that is MaxViewIDDiff (100) further from the current viewID",
	)
)

func (consensus *Consensus) onViewChangeSanityCheck(
	recvMsg *FBFTMessage,
) error {
	// TODO: if difference is only one, new leader can still propose the same committed block to avoid another view change
	// TODO: new leader catchup without ignore view change message
	if consensus.blockNum > recvMsg.BlockNum {
		consensus.getLogger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onViewChange] Message BlockNum Is Low")
		return errLowBlockNum
	}
	if consensus.blockNum < recvMsg.BlockNum {
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onViewChange] New Leader Has Lower Blocknum")
		return errNewLeaderLowBlockNum
	}
	if consensus.current.Mode() == ViewChanging &&
		consensus.current.ViewID() > recvMsg.ViewID {
		consensus.getLogger().Warn().
			Uint64("MyViewChangingID", consensus.current.ViewID()).
			Uint64("MsgViewChangingID", recvMsg.ViewID).
			Msg("[onViewChange] ViewChanging ID Is Low")
		return errViewChangeIDLow
	}
	if recvMsg.ViewID-consensus.current.ViewID() > MaxViewIDDiff {
		consensus.getLogger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("CurrentViewID", consensus.current.ViewID()).
			Msg("Received viewID that is MaxViewIDDiff (100) further from the current viewID!")
		return errVCIDDiffToLarge
	}
	return nil
}

var (
	errNewViewSanityCheck = errors.New(
		"viewID should be larger than the viewID of the last successful consensus",
	)
	errNotInVCMode = errors.New("not in ViewChanging mode, ignoring the new view message")
)

func (consensus *Consensus) onNewViewSanityCheck(
	recvMsg *FBFTMessage,
) error {
	if recvMsg.ViewID <= consensus.viewID {
		consensus.getLogger().Warn().
			Uint64("LastSuccessfulConsensusViewID", consensus.viewID).
			Uint64("MsgViewChangingID", recvMsg.ViewID).
			Msg("[onNewView] ViewID should be larger than the viewID of the last successful consensus")
		return errNewViewSanityCheck
	}
	if consensus.current.Mode() != ViewChanging {
		consensus.getLogger().Warn().
			Msg("[onNewView] Not in ViewChanging mode, ignoring the new view message")
		return errNotInVCMode
	}
	return nil
}
