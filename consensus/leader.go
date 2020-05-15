package consensus

import (
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/pkg/errors"
)

func (consensus *Consensus) announce(block *types.Block) {
	blockHash := block.Hash()
	copy(consensus.blockHash[:], blockHash[:])

	// prepare message and broadcast to validators
	encodedBlock, err := rlp.EncodeToBytes(block)
	if err != nil {
		consensus.getLogger().Debug().Msg("[Announce] Failed encoding block")
		return
	}
	encodedBlockHeader, err := rlp.EncodeToBytes(block.Header())
	if err != nil {
		consensus.getLogger().Debug().Msg("[Announce] Failed encoding block header")
		return
	}

	consensus.block = encodedBlock
	consensus.blockHeader = encodedBlockHeader

	key, err := consensus.GetConsensusLeaderPrivateKey()
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[Announce] Node not a leader")
		return
	}
	networkMessage, err := consensus.construct(msg_pb.MessageType_ANNOUNCE, nil, key.GetPublicKey(), key)
	if err != nil {
		consensus.getLogger().Err(err).
			Str("message-type", msg_pb.MessageType_ANNOUNCE.String()).
			Msg("failed constructing message")
		return
	}
	msgToSend, FPBTMsg := networkMessage.Bytes, networkMessage.FBFTMsg

	// TODO(chao): review FPBT log data structure
	consensus.FBFTLog.AddMessage(FPBTMsg)
	consensus.getLogger().Debug().
		Str("MsgBlockHash", FPBTMsg.BlockHash.Hex()).
		Uint64("MsgViewID", FPBTMsg.ViewID).
		Uint64("MsgBlockNum", FPBTMsg.BlockNum).
		Msg("[Announce] Added Announce message in FPBT")
	consensus.FBFTLog.AddBlock(block)

	// Leader sign the block hash itself
	for i, key := range consensus.PubKey.PublicKey {
		if _, err := consensus.Decider.SubmitVote(
			quorum.Prepare,
			key,
			consensus.priKey.PrivateKey[i].SignHash(consensus.blockHash[:]),
			block.Hash(),
			block.NumberU64(),
			block.Header().ViewID().Uint64(),
		); err != nil {
			return
		}
		if err := consensus.prepareBitmap.SetKey(key, true); err != nil {
			consensus.getLogger().Warn().Err(err).Msg(
				"[Announce] Leader prepareBitmap SetKey failed",
			)
			return
		}
	}
	// Construct broadcast p2p message
	if err := consensus.msgSender.SendWithRetry(
		consensus.blockNum, msg_pb.MessageType_ANNOUNCE, []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		}, p2p.ConstructMessage(msgToSend)); err != nil {
		consensus.getLogger().Warn().
			Str("groupID", string(nodeconfig.NewGroupIDByShardID(
				nodeconfig.ShardID(consensus.ShardID),
			))).
			Msg("[Announce] Cannot send announce message")
	} else {
		consensus.getLogger().Info().
			Str("blockHash", block.Hash().Hex()).
			Uint64("blockNum", block.NumberU64()).
			Msg("[Announce] Sent Announce Message!!")
	}

	consensus.getLogger().Debug().
		Str("From", consensus.phase.String()).
		Str("To", FBFTPrepare.String()).
		Msg("[Announce] Switching phase")
	consensus.switchPhase(FBFTPrepare, true)
}

var (
	errInvalidBLSSign         = errors.New("onPrepare received an invalid BLS signature")
	errFailOnCommitVerifyHash = errors.New("leader cannot verify commit message")
)

func (consensus *Consensus) onPrepare(msg *msg_pb.Message) error {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("[OnPrepare] Unparseable validator message")
		return err
	}

	if recvMsg.ViewID != consensus.viewID {
		return errViewIDCheckFail
	}

	if recvMsg.BlockNum != consensus.blockNum {
		return errWrongBlockNum
	}

	if !consensus.FBFTLog.HasMatchingViewAnnounce(
		consensus.blockNum, consensus.viewID, recvMsg.BlockHash,
	) {
		consensus.getLogger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Msg("[OnPrepare] No Matching Announce message")
		//return
	}

	validatorPubKey := recvMsg.SenderPubkey
	prepareSig := recvMsg.Payload
	prepareBitmap := consensus.prepareBitmap

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	logger := consensus.getLogger().With().
		Str("validatorPubKey", validatorPubKey.SerializeToHexStr()).Logger()

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
	err = sign.Deserialize(prepareSig)
	if err != nil {
		consensus.getLogger().Error().Err(err).
			Msg("[OnPrepare] Failed to deserialize bls signature")
		return err
	}
	if !sign.VerifyHash(recvMsg.SenderPubkey, consensus.blockHash[:]) {
		consensus.getLogger().Error().Msg("[OnPrepare] Received invalid BLS signature")
		return errInvalidBLSSign
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
		consensus.getLogger().Warn().Err(err).Msg("submit vote prepare failed")
		return err
	}
	// Set the bitmap indicating that this validator signed.
	if err := prepareBitmap.SetKey(recvMsg.SenderPubkey, true); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[OnPrepare] prepareBitmap.SetKey failed")
		return err
	}

	if consensus.Decider.IsQuorumAchieved(quorum.Prepare) {
		// NOTE Let it handle its own logs
		if err := consensus.didReachPrepareQuorum(); err != nil {
			return err
		}
		consensus.switchPhase(FBFTCommit, true)
	}
	return nil
}

func (consensus *Consensus) onCommit(msg *msg_pb.Message) error {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		consensus.getLogger().Debug().Err(err).Msg("[OnCommit] Parse pbft message failed")
		return err
	}

	// NOTE let it handle its own log
	if err := consensus.isRightBlockNumAndViewID(recvMsg); err != nil {
		return err
	}

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	// Check for potential double signing
	if consensus.checkDoubleSign(recvMsg) {
		return errors.New("could be a double signer")
	}

	validatorPubKey, commitSig, commitBitmap :=
		recvMsg.SenderPubkey, recvMsg.Payload, consensus.commitBitmap
	logger := consensus.getLogger().With().
		Str("validatorPubKey", validatorPubKey.SerializeToHexStr()).Logger()

	// has to be called before verifying signature
	quorumWasMet := consensus.Decider.IsQuorumAchieved(quorum.Commit)
	// Verify the signature on commitPayload is correct
	var sign bls.Sign
	if err := sign.Deserialize(commitSig); err != nil {
		logger.Debug().Msg("[OnCommit] Failed to deserialize bls signature")
		return err
	}
	// Must have the corresponding block to verify committed message.
	blockObj := consensus.FBFTLog.GetBlockByHash(recvMsg.BlockHash)
	if blockObj == nil {
		consensus.getLogger().Debug().
			Uint64("blockNum", recvMsg.BlockNum).
			Uint64("viewID", recvMsg.ViewID).
			Str("blockHash", recvMsg.BlockHash.Hex()).
			Msg("[OnCommit] Failed finding a matching block for committed message")
		return errFailFindMatchBlock
	}
	commitPayload := signature.ConstructCommitPayload(
		consensus.ChainReader,
		blockObj.Epoch(), blockObj.Hash(),
		blockObj.NumberU64(), blockObj.Header().ViewID().Uint64(),
	)
	logger = logger.With().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Logger()

	if !sign.VerifyHash(recvMsg.SenderPubkey, commitPayload) {
		logger.Error().Msg("[OnCommit] Cannot verify commit message")
		return errFailOnCommitVerifyHash
	}

	logger = logger.With().
		Int64("numReceivedSoFar", consensus.Decider.SignersCount(quorum.Commit)).
		Logger()
	logger.Info().Msg("[OnCommit] Received new commit message")

	if _, err := consensus.Decider.SubmitVote(
		quorum.Commit, validatorPubKey,
		&sign, recvMsg.BlockHash,
		recvMsg.BlockNum, recvMsg.ViewID,
	); err != nil {
		return err
	}
	// Set the bitmap indicating that this validator signed.
	if err := commitBitmap.SetKey(recvMsg.SenderPubkey, true); err != nil {
		consensus.getLogger().Warn().Err(err).
			Msg("[OnCommit] commitBitmap.SetKey failed")
		return err
	}

	viewID := consensus.viewID

	quorumIsMet := consensus.Decider.IsQuorumAchieved(quorum.Commit)
	if !quorumWasMet && quorumIsMet {
		logger.Info().Msg("[OnCommit] 2/3 Enough commits received")
		next := consensus.NextBlockDue
		time.AfterFunc(2*time.Second, func() {
			<-time.After(time.Until(next))
			consensus.commitFinishChan <- viewID
		})

		consensus.msgSender.StopRetry(msg_pb.MessageType_PREPARED)
	}

	if consensus.Decider.IsAllSigsCollected() {
		go func() {
			consensus.commitFinishChan <- viewID
			logger.Info().Msg("[OnCommit] 100% Enough commits received")
		}()
	}

	return nil
}
