package consensus

import (
	"bytes"

	protobuf "github.com/golang/protobuf/proto"
	libbls "github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/pkg/errors"
)

// MaxBlockNumDiff limits the received block number to only 100 further from the current block number
const MaxBlockNumDiff = 100

// verifyMessageSig verify the signature of the message are valid from the signer's public key.
func verifyMessageSig(signerPubKey *libbls.PublicKey, message *msg_pb.Message) error {
	signature := message.Signature
	message.Signature = nil
	messageBytes, err := protobuf.Marshal(message)
	if err != nil {
		return err
	}

	msgSig := libbls.Sign{}
	err = msgSig.Deserialize(signature)
	if err != nil {
		return err
	}
	msgHash := hash.Keccak256(messageBytes)
	if !msgSig.VerifyHash(signerPubKey, msgHash[:]) {
		return errors.New("failed to verify the signature")
	}
	message.Signature = signature
	return nil
}

func (consensus *Consensus) senderKeySanityChecks(msg *msg_pb.Message, senderKey *bls.SerializedPublicKey) bool {
	pubkey, err := bls.BytesToBLSPublicKey(senderKey[:])
	if err != nil {
		return false
	}

	if err := verifyMessageSig(pubkey, msg); err != nil {
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
	if recvMsg.ViewID != consensus.GetCurViewID() || recvMsg.BlockNum != consensus.blockNum {
		consensus.getLogger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Interface("ValidatorPubKey", recvMsg.SenderPubkeys).
			Msg("BlockNum/viewID not match")
		return false
	}
	return true
}

func (consensus *Consensus) onAnnounceSanityChecks(recvMsg *FBFTMessage) bool {
	logMsgs := consensus.FBFTLog.GetMessagesByTypeSeqView(
		msg_pb.MessageType_ANNOUNCE, recvMsg.BlockNum, recvMsg.ViewID,
	)
	if len(logMsgs) > 0 {
		if len(logMsgs[0].SenderPubkeys) != 1 || len(recvMsg.SenderPubkeys) != 1 {
			consensus.getLogger().Debug().
				Interface("signers", recvMsg.SenderPubkeys).
				Msg("[OnAnnounce] Announce message have 0 or more than 1 signers")
			return false
		}
		if logMsgs[0].BlockHash != recvMsg.BlockHash &&
			bytes.Equal(logMsgs[0].SenderPubkeys[0].Bytes[:], recvMsg.SenderPubkeys[0].Bytes[:]) {
			consensus.getLogger().Debug().
				Str("logMsgSenderKey", logMsgs[0].SenderPubkeys[0].Bytes.Hex()).
				Str("logMsgBlockHash", logMsgs[0].BlockHash.Hex()).
				Str("recvMsg.SenderPubkeys", recvMsg.SenderPubkeys[0].Bytes.Hex()).
				Uint64("recvMsg.BlockNum", recvMsg.BlockNum).
				Uint64("recvMsg.ViewID", recvMsg.ViewID).
				Str("recvMsgBlockHash", recvMsg.BlockHash.Hex()).
				Str("LeaderKey", consensus.LeaderPubKey.Bytes.Hex()).
				Msg("[OnAnnounce] Leader is malicious")
			if consensus.IsViewChangingMode() {
				consensus.getLogger().Debug().Msg(
					"[OnAnnounce] Already in ViewChanging mode, conflicing announce, doing noop",
				)
			} else {
				consensus.startViewChange(consensus.GetCurViewID() + 1)
			}
		}
		consensus.getLogger().Debug().
			Str("leaderKey", consensus.LeaderPubKey.Bytes.Hex()).
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

// TODO: leo: move the sanity check to p2p message validation
func (consensus *Consensus) onViewChangeSanityCheck(recvMsg *FBFTMessage) bool {
	// TODO: if difference is only one, new leader can still propose the same committed block to avoid another view change
	// TODO: new leader catchup without ignore view change message

	consensus.getLogger().Info().
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Uint64("MyViewChangingID", consensus.GetViewChangingID()).
		Uint64("MsgViewChangingID", recvMsg.ViewID).
		Msg("onViewChange")

	if consensus.blockNum > recvMsg.BlockNum {
		consensus.getLogger().Debug().
			Msg("[onViewChange] Message BlockNum Is Low")
		return false
	}
	if consensus.blockNum < recvMsg.BlockNum {
		consensus.getLogger().Warn().
			Msg("[onViewChange] New Leader Has Lower Blocknum")
		return false
	}
	if consensus.IsViewChangingMode() &&
		consensus.GetViewChangingID() > recvMsg.ViewID {
		consensus.getLogger().Warn().
			Msg("[onViewChange] ViewChanging ID Is Low")
		return false
	}
	if recvMsg.ViewID-consensus.GetViewChangingID() > MaxViewIDDiff {
		consensus.getLogger().Debug().
			Msg("Received viewID that is MaxViewIDDiff (100) further from the current viewID!")
		return false
	}
	return true
}

// TODO: leo: move the sanity check to p2p message validation
func (consensus *Consensus) onNewViewSanityCheck(recvMsg *FBFTMessage) bool {
	if recvMsg.ViewID < consensus.GetCurViewID() {
		consensus.getLogger().Warn().
			Uint64("LastSuccessfulConsensusViewID", consensus.GetCurViewID()).
			Uint64("MsgViewChangingID", recvMsg.ViewID).
			Msg("[onNewView] ViewID should be larger than the viewID of the last successful consensus")
		return false
	}
	if !consensus.IsViewChangingMode() {
		consensus.getLogger().Warn().
			Msg("[onNewView] Not in ViewChanging mode, ignoring the new view message")
		return false
	}
	return true
}
