package consensus

import (
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
)

func (consensus *Consensus) validatorSanityChecks(msg *msg_pb.Message) bool {
	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		if err == errValidNotInCommittee {
			errMsg := "[SanityChecks] Sender key not in this slot's subcommittee"
			consensus.getLogger().Info().Msg(errMsg)
			addToConsensusLog(msg, errMsg)
		} else {
			errMsg := "[SanityChecks] VerifySenderKey failed"
			consensus.getLogger().Error().Err(err).Msg(errMsg)
			addToConsensusLog(msg, errMsg)
		}
		return false
	}

	if !senderKey.IsEqual(consensus.LeaderPubKey) &&
		consensus.current.Mode() == Normal && !consensus.ignoreViewIDCheck {
		errMsg := "[SanityChecks] SenderKey not match leader PubKey"
		consensus.getLogger().Warn().Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return false
	}

	if err := verifyMessageSig(senderKey, msg); err != nil {
		errMsg := "[SanityChecks] Failed to verify sender's signature"
		consensus.getLogger().Error().Err(err).Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return false
	}

	return true
}

func (consensus *Consensus) leaderSanityChecks(msg *msg_pb.Message) bool {
	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		if err == errValidNotInCommittee {
			errMsg := "[LeaderSanityChecks] sender key not in this slot's subcommittee"
			consensus.getLogger().Info().Msg(errMsg)
			addToConsensusLog(msg, errMsg)
		} else {
			errMsg := "[LeaderSanityChecks] VerifySenderKey failed"
			consensus.getLogger().Error().Err(err).Msg(errMsg)
			addToConsensusLog(msg, errMsg)
		}
		return false
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		errMsg := "[LeaderSanityChecks] failed to verify sender's signature"
		consensus.getLogger().Error().Err(err).Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return false
	}

	return true
}

func (consensus *Consensus) onCommitSanityChecks(msg *msg_pb.Message) bool {
	recvMsg, _ := ParseFBFTMessage(msg)
	if recvMsg.ViewID != consensus.viewID || recvMsg.BlockNum != consensus.blockNum {
		errMsg := "[OnCommit] BlockNum/ViewID not match"
		consensus.getLogger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Str("ValidatorPubKey", recvMsg.SenderPubkey.SerializeToHexStr()).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return false
	}

	if !consensus.FBFTLog.HasMatchingAnnounce(consensus.blockNum, recvMsg.BlockHash) {
		errMsg := "[OnCommit] Cannot find matching Announce message"
		consensus.getLogger().Debug().
			Hex("MsgBlockHash", recvMsg.BlockHash[:]).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return false
	}

	if !consensus.FBFTLog.HasMatchingPrepared(consensus.blockNum, recvMsg.BlockHash) {
		errMsg := "[OnCommit] Cannot find matching Prepared message"
		consensus.getLogger().Debug().
			Hex("blockHash", recvMsg.BlockHash[:]).
			Uint64("blockNum", consensus.blockNum).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return false
	}
	return true
}

func (consensus *Consensus) onPreparedSanityChecks(
	blockObj *types.Block, msg *msg_pb.Message,
) bool {
	recvMsg, _ := ParseFBFTMessage(msg)
	if blockObj.NumberU64() != recvMsg.BlockNum ||
		recvMsg.BlockNum < consensus.blockNum {
		errMsg := "[OnPrepared] BlockNum not match"
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", blockObj.NumberU64()).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return false
	}
	if blockObj.Header().Hash() != recvMsg.BlockHash {
		errMsg := "[OnPrepared] BlockHash not match"
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Hex("MsgBlockHash", recvMsg.BlockHash[:]).
			Str("blockObjHash", blockObj.Header().Hash().Hex()).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return false
	}
	if consensus.current.Mode() == Normal {
		err := chain.Engine.VerifyHeader(consensus.ChainReader, blockObj.Header(), true)
		if err != nil {
			errMsg := "[OnPrepared] Block header is not verified successfully"
			consensus.getLogger().Error().
				Err(err).
				Str("inChain", consensus.ChainReader.CurrentHeader().Number().String()).
				Str("MsgBlockNum", blockObj.Header().Number().String()).
				Msg(errMsg)
			addToConsensusLog(msg, errMsg)
			return false
		}
		if consensus.BlockVerifier == nil {
			// do nothing
		} else if err := consensus.BlockVerifier(blockObj); err != nil {
			errMsg := "[OnPrepared] Block verification failed"
			consensus.getLogger().Error().Err(err).Msg(errMsg)
			addToConsensusLog(msg, errMsg)
			return false
		}
	}

	return true
}
