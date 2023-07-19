package consensus

import (
	"bytes"
	"encoding/binary"

	protobuf "github.com/golang/protobuf/proto"
	libbls "github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
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

func (consensus *Consensus) isRightBlockNumAndViewID(recvMsg *FBFTMessage) bool {
	blockNum := consensus.getBlockNum()
	if recvMsg.ViewID != consensus.getCurBlockViewID() || recvMsg.BlockNum != blockNum {
		consensus.getLogger().Debug().
			Uint64("blockNum", blockNum).
			Str("recvMsg", recvMsg.String()).
			Msg("BlockNum/viewID not match")
		return false
	}
	return true
}

func (consensus *Consensus) onAnnounceSanityChecks(recvMsg *FBFTMessage) bool {
	logMsgs := consensus.fBFTLog.GetMessagesByTypeSeqView(
		msg_pb.MessageType_ANNOUNCE, recvMsg.BlockNum, recvMsg.ViewID,
	)
	if len(logMsgs) > 0 {
		if !logMsgs[0].HasSingleSender() || !recvMsg.HasSingleSender() {
			consensus.getLogger().Warn().
				Str("logMsgs[0]", logMsgs[0].String()).
				Str("recvMsg", recvMsg.String()).
				Msg("[OnAnnounce] Announce message have 0 or more than 1 signers")
			return false
		}
		if logMsgs[0].BlockHash != recvMsg.BlockHash &&
			bytes.Equal(logMsgs[0].SenderPubkeys[0].Bytes[:], recvMsg.SenderPubkeys[0].Bytes[:]) {
			consensus.getLogger().Debug().
				Str("logMsgSenderKey", logMsgs[0].SenderPubkeys[0].Bytes.Hex()).
				Str("logMsgBlockHash", logMsgs[0].BlockHash.Hex()).
				Str("recvMsg", recvMsg.String()).
				Str("LeaderKey", consensus.LeaderPubKey.Bytes.Hex()).
				Msg("[OnAnnounce] Leader is malicious")
			if consensus.isViewChangingMode() {
				consensus.getLogger().Debug().Msg(
					"[OnAnnounce] Already in ViewChanging mode, conflicing announce, doing noop",
				)
			} else {
				consensus.startViewChange()
			}
		}
		consensus.getLogger().Debug().
			Str("leaderKey", consensus.LeaderPubKey.Bytes.Hex()).
			Msg("[OnAnnounce] Announce message received again")
	}
	return consensus.isRightBlockNumCheck(recvMsg)
}

func (consensus *Consensus) isRightBlockNumCheck(recvMsg *FBFTMessage) bool {
	if recvMsg.BlockNum < consensus.BlockNum() {
		consensus.getLogger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("Wrong BlockNum Received, ignoring!")
		return false
	} else if recvMsg.BlockNum-consensus.BlockNum() > MaxBlockNumDiff {
		consensus.getLogger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("MaxBlockNumDiff", MaxBlockNumDiff).
			Msg("Received blockNum that is MaxBlockNumDiff further from the current blockNum!")
		return false
	}
	return true
}

func (consensus *Consensus) newBlockSanityChecks(
	blockObj *types.Block, recvMsg *FBFTMessage,
) bool {
	if blockObj.NumberU64() != recvMsg.BlockNum ||
		recvMsg.BlockNum < consensus.BlockNum() {
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", blockObj.NumberU64()).
			Msg("[newBlockSanityChecks] BlockNum not match")
		return false
	}
	if blockObj.Header().Hash() != recvMsg.BlockHash {
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Hex("MsgBlockHash", recvMsg.BlockHash[:]).
			Str("blockObjHash", blockObj.Header().Hash().Hex()).
			Msg("[newBlockSanityChecks] BlockHash not match")
		return false
	}
	return true
}

// TODO: leo: move the sanity check to p2p message validation
func (consensus *Consensus) onViewChangeSanityCheck(recvMsg *FBFTMessage) bool {
	// TODO: if difference is only one, new leader can still propose the same committed block to avoid another view change
	// TODO: new leader catchup without ignore view change message

	consensus.getLogger().Debug().
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Uint64("MyViewChangingID", consensus.getViewChangingID()).
		Uint64("MsgViewChangingID", recvMsg.ViewID).
		Interface("SendPubKeys", recvMsg.SenderPubkeys).
		Msg("[onViewChangeSanityCheck]")

	if consensus.getBlockNum() > recvMsg.BlockNum {
		consensus.getLogger().Debug().
			Msg("[onViewChange] Message BlockNum Is Low")
		return false
	}
	if consensus.BlockNum() < recvMsg.BlockNum {
		consensus.getLogger().Warn().
			Msg("[onViewChangeSanityCheck] MsgBlockNum is different from my BlockNumber")
		return false
	}
	if consensus.isViewChangingMode() &&
		consensus.getCurBlockViewID() > recvMsg.ViewID {
		consensus.getLogger().Debug().Uint64("curBlockViewID", consensus.getCurBlockViewID()).
			Uint64("msgViewID", recvMsg.ViewID).
			Msg("[onViewChangeSanityCheck] ViewChanging ID Is Low")
		return false
	}
	if recvMsg.ViewID > consensus.getViewChangingID() && recvMsg.ViewID-consensus.getViewChangingID() > MaxViewIDDiff {
		consensus.getLogger().Debug().
			Msg("[onViewChangeSanityCheck] Received viewID that is MaxViewIDDiff (249) further from the current viewID!")
		return false
	}

	if !recvMsg.HasSingleSender() {
		consensus.getLogger().Error().Msg("[onViewChangeSanityCheck] zero or multiple signers in view change message.")
		return false
	}
	senderKey := recvMsg.SenderPubkeys[0]

	viewIDHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDHash, recvMsg.ViewID)
	if !recvMsg.ViewidSig.VerifyHash(senderKey.Object, viewIDHash) {
		consensus.getLogger().Warn().
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[onViewChangeSanityCheck] Failed to Verify viewID Signature")
		return false
	}
	return true
}

// TODO: leo: move the sanity check to p2p message validation
func (consensus *Consensus) onNewViewSanityCheck(recvMsg *FBFTMessage) bool {
	if recvMsg.ViewID < consensus.getCurBlockViewID() {
		consensus.getLogger().Warn().
			Uint64("LastSuccessfulConsensusViewID", consensus.getCurBlockViewID()).
			Uint64("MsgViewChangingID", recvMsg.ViewID).
			Msg("[onNewView] ViewID should be larger than the viewID of the last successful consensus")
		return false
	}
	return true
}
