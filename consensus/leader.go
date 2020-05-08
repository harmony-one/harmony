package consensus

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/pkg/errors"
)

// Announce ..
func (consensus *Consensus) Announce(block *types.Block) error {

	consensus.SetBlockHash(block.Hash())
	encodedBlock, err := rlp.EncodeToBytes(block)

	if err != nil {
		utils.Logger().Debug().Msg("[Announce] Failed encoding block")
		return err
	}
	encodedBlockHeader, err := rlp.EncodeToBytes(block.Header())
	if err != nil {
		utils.Logger().Debug().Msg("[Announce] Failed encoding block header")
		return err
	}

	consensus.SetBlock(encodedBlock)
	consensus.SetBlockHeader(encodedBlockHeader)

	key, err := consensus.GetConsensusLeaderPrivateKey()
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("[Announce] Node not a leader")
		return err
	}

	networkMessage, err := consensus.construct(
		msg_pb.MessageType_ANNOUNCE, nil, key.GetPublicKey(), key,
	)
	if err != nil {
		utils.Logger().Err(err).
			Str("message-type", msg_pb.MessageType_ANNOUNCE.String()).
			Msg("failed constructing message")
		return err
	}
	msgToSend, FPBTMsg := networkMessage.Bytes, networkMessage.FBFTMsg

	// TODO(chao): review FPBT log data structure
	consensus.FBFTLog.AddMessage(FPBTMsg)
	utils.Logger().Debug().
		Str("MsgBlockHash", FPBTMsg.BlockHash.Hex()).
		Uint64("MsgViewID", FPBTMsg.ViewID).
		Uint64("MsgBlockNum", FPBTMsg.BlockNum).
		Msg("[Announce] Added Announce message in FPBT")
	consensus.FBFTLog.AddBlock(block)

	h := block.Hash()
	d := common.BytesToHash(h[:])

	// Leader sign the block hash itself
	for i := range consensus.PubKey.PublicKey {
		if _, err := consensus.Decider.SubmitVote(
			quorum.Prepare,
			consensus.PubKey.PublicKey[i],
			consensus.priKey.PrivateKey[i].SignHash(h[:]),
			d,
			consensus.BlockNum(),
			consensus.ViewID(),
		); err != nil {
			return err
		}
		// TODO race condition on the prepare bitmap
		if err := consensus.prepareBitmap.SetKey(
			consensus.PubKey.PublicKey[i], true,
		); err != nil {
			utils.Logger().Warn().Err(err).Msg(
				"[Announce] Leader prepareBitmap SetKey failed",
			)
			return err
		}
	}

	if err := consensus.host.SendMessageToGroups([]nodeconfig.GroupID{
		nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
	}, p2p.ConstructMessage(msgToSend)); err != nil {
		utils.Logger().Warn().
			Str("groupID", string(nodeconfig.NewGroupIDByShardID(
				nodeconfig.ShardID(consensus.ShardID),
			))).
			Msg("[Announce] Cannot send announce message")
		return err
	}
	utils.Logger().Info().
		Str("blockHash", block.Hash().Hex()).
		Uint64("blockNum", block.NumberU64()).
		Msg("[Announce] Sent Announce Message!!")

	consensus.switchPhase(FBFTPrepare)
	return nil
}

func (consensus *Consensus) onPrepare(msg *msg_pb.Message) error {

	if !consensus.IsLeader() {
		return nil
	}

	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[OnPrepare] Unparseable validator message")
		return err
	}

	if recvMsg.ViewID != consensus.ViewID() {
		utils.Logger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("OnPrepare message viewID not match")
		return errors.Errorf(
			"on prepare message viewID not match %d %d",
			recvMsg.ViewID,
			consensus.ViewID(),
		)
	}

	if recvMsg.BlockNum != consensus.BlockNum() {
		utils.Logger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnPrepare] blockNum not match")
		return errors.Errorf(
			"on prepare message blockNum not match %d %d",
			recvMsg.BlockNum,
			consensus.BlockNum(),
		)

	}

	if !consensus.FBFTLog.HasMatchingViewAnnounce(
		consensus.BlockNum(), consensus.ViewID(), recvMsg.BlockHash,
	) {
		utils.Logger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnPrepare] No Matching Announce message")
		fmt.Println("does this ever happen")
		//return
	}

	validatorPubKey := recvMsg.SenderPubkey
	prepareSig := recvMsg.Payload
	logger := utils.Logger().With().Logger()
	consensus.Locks.Global.Lock()
	defer consensus.Locks.Global.Unlock()

	// proceed only when the message is not received before
	signed := consensus.Decider.ReadBallot(quorum.Prepare, validatorPubKey)
	if signed != nil {
		logger.Debug().
			Msg("[OnPrepare] Already Received prepare message from the validator")
		return nil
	}

	if consensus.Decider.IsQuorumAchieved(quorum.Prepare) {
		// already have enough signatures
		logger.Debug().Msg("[OnPrepare] Received Additional Prepare Message")
		return nil
	}

	// Check BLS signature for the multi-sig
	var sign bls.Sign
	if err := sign.Deserialize(prepareSig); err != nil {
		utils.Logger().Error().Err(err).
			Msg("[OnPrepare] Failed to deserialize bls signature")
		return err
	}
	if h := consensus.BlockHash(); !sign.VerifyHash(recvMsg.SenderPubkey, h[:]) {
		utils.Logger().Error().Msg("[OnPrepare] Received invalid BLS signature")
		return errors.New("Received invalid BLS signature")
	}

	logger = logger.With().
		Int64("NumReceivedSoFar", consensus.Decider.SignersCount(quorum.Prepare)).
		Int64("PublicKeys", consensus.Decider.ParticipantsCount()).Logger()
	logger.Info().Msg("[OnPrepare] Received New Prepare Signature")
	if _, err := consensus.Decider.SubmitVote(
		quorum.Prepare, validatorPubKey,
		&sign, recvMsg.BlockHash,
		recvMsg.BlockNum, recvMsg.ViewID,
	); err != nil {
		utils.Logger().Warn().Err(err).Msg("submit vote prepare failed")
		return err
	}
	// Set the bitmap indicating that this validator signed.
	if err := consensus.prepareBitmap.SetKey(
		recvMsg.SenderPubkey, true,
	); err != nil {
		utils.Logger().Warn().Err(err).Msg("[OnPrepare] prepareBitmap.SetKey failed")
		return err
	}

	if consensus.Decider.IsQuorumAchieved(quorum.Prepare) {
		// NOTE Let it handle its own logs
		if err := consensus.didReachPrepareQuorum(); err != nil {
			return err
		}
		consensus.switchPhase(FBFTCommit)
	}
	return nil
}

func (consensus *Consensus) onCommit(msg *msg_pb.Message) error {

	if !consensus.IsLeader() {
		return nil
	}

	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		utils.Logger().Debug().Err(err).Msg("[OnCommit] Parse pbft message failed")
		return err
	}

	// NOTE let it handle its own log
	if !consensus.isRightBlockNumAndViewID(recvMsg) {
		return nil
	}

	consensus.Locks.Global.Lock()
	defer consensus.Locks.Global.Unlock()

	// Check for potential double signing
	if consensus.checkDoubleSign(recvMsg) {
		return nil
	}

	validatorPubKey, commitSig, commitBitmap :=
		recvMsg.SenderPubkey, recvMsg.Payload, consensus.commitBitmap
	logger := utils.Logger().With().Logger()

	// has to be called before verifying signature
	// quorumWasMet := consensus.Decider.IsQuorumAchieved(quorum.Commit)
	// Verify the signature on commitPayload is correct
	var sign bls.Sign
	if err := sign.Deserialize(commitSig); err != nil {
		logger.Debug().Msg("[OnCommit] Failed to deserialize bls signature")
		return err
	}

	commitPayload := signature.ConstructCommitPayload(
		consensus.ChainReader,
		new(big.Int).SetUint64(consensus.Epoch()),
		recvMsg.BlockHash[:],
		recvMsg.BlockNum,
		consensus.ViewID(),
	)
	logger = logger.With().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Logger()

	if !sign.VerifyHash(recvMsg.SenderPubkey, commitPayload) {
		logger.Error().Msg("[OnCommit] Cannot verify commit message")
		return errors.New("Cannot verify commit message")
	}

	utils.Logger().Info().
		Int64("numReceivedSoFar", consensus.Decider.SignersCount(quorum.Commit)).
		Msg("[OnCommit] Received new commit message")

	if _, err := consensus.Decider.SubmitVote(
		quorum.Commit, validatorPubKey,
		&sign, recvMsg.BlockHash,
		recvMsg.BlockNum, recvMsg.ViewID,
	); err != nil {
		fmt.Println("How can I be given an error?", err.Error())
		return err
	}
	// Set the bitmap indicating that this validator signed.
	if err := commitBitmap.SetKey(recvMsg.SenderPubkey, true); err != nil {
		utils.Logger().Warn().Err(err).
			Msg("[OnCommit] commitBitmap.SetKey failed")
		return err
	}

	if consensus.Decider.IsQuorumAchieved(quorum.Commit) {
		f := Finished{
			consensus.ViewID(),
			consensus.ShardID,
			consensus.BlockNum(),
			consensus.BlockHash(),
			consensus.NextBlockDue(),
		}
		go func() {
			consensus.CommitFinishChan <- f
		}()
	}

	return nil
}
