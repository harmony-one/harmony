package consensus

import (
	"time"

	"github.com/harmony-one/harmony/consensus/signature"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/p2p"
)

func (consensus *Consensus) announce(block *types.Block) {
	blockHash := block.Hash()

	// prepare message and broadcast to validators
	encodedBlock, err := rlp.EncodeToBytes(block)
	if err != nil {
		consensus.getLogger().Debug().Msg("[Announce] Failed encoding block")
		return
	}

	//// Lock Write - Start
	consensus.mutex.Lock()
	copy(consensus.blockHash[:], blockHash[:])
	consensus.block = encodedBlock
	consensus.mutex.Unlock()
	//// Lock Write - End

	key, err := consensus.GetConsensusLeaderPrivateKey()
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[Announce] Node not a leader")
		return
	}

	networkMessage, err := consensus.construct(msg_pb.MessageType_ANNOUNCE, nil, key)
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
	for i, key := range consensus.priKey {
		if err := consensus.prepareBitmap.SetKey(key.Pub.Bytes, true); err != nil {
			consensus.getLogger().Warn().Err(err).Msgf(
				"[Announce] Leader prepareBitmap SetKey failed for key at index %d", i,
			)
			continue
		}

		if _, err := consensus.Decider.AddNewVote(
			quorum.Prepare,
			key.Pub.Bytes,
			key.Pri.SignHash(consensus.blockHash[:]),
			block.Hash(),
			block.NumberU64(),
			block.Header().ViewID().Uint64(),
		); err != nil {
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

func (consensus *Consensus) onPrepare(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("[OnPrepare] Unparseable validator message")
		return
	}

	// TODO(audit): make FBFT lookup using map instead of looping through all items.
	if !consensus.FBFTLog.HasMatchingViewAnnounce(
		consensus.blockNum, consensus.viewID, recvMsg.BlockHash,
	) {
		consensus.getLogger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Msg("[OnPrepare] No Matching Announce message")
	}

	//// Read - Start
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	if !consensus.isRightBlockNumAndViewID(recvMsg) {
		return
	}

	blockHash := consensus.blockHash[:]
	prepareBitmap := consensus.prepareBitmap
	// proceed only when the message is not received before
	signed := consensus.Decider.ReadBallot(quorum.Prepare, recvMsg.SenderPubkey.Bytes)
	if signed != nil {
		consensus.getLogger().Debug().
			Str("validatorPubKey", recvMsg.SenderPubkey.Bytes.Hex()).
			Msg("[OnPrepare] Already Received prepare message from the validator")
		return
	}

	if consensus.Decider.IsQuorumAchieved(quorum.Prepare) {
		// already have enough signatures
		consensus.getLogger().Debug().
			Str("validatorPubKey", recvMsg.SenderPubkey.Bytes.Hex()).
			Msg("[OnPrepare] Received Additional Prepare Message")
		return
	}
	signerCount := consensus.Decider.SignersCount(quorum.Prepare)
	//// Read - End

	// Check BLS signature for the multi-sig
	prepareSig := recvMsg.Payload
	var sign bls.Sign
	err = sign.Deserialize(prepareSig)
	if err != nil {
		consensus.getLogger().Error().Err(err).
			Msg("[OnPrepare] Failed to deserialize bls signature")
		return
	}
	if !sign.VerifyHash(recvMsg.SenderPubkey.Object, blockHash) {
		consensus.getLogger().Error().Msg("[OnPrepare] Received invalid BLS signature")
		return
	}

	consensus.getLogger().Debug().
		Int64("NumReceivedSoFar", signerCount).
		Int64("PublicKeys", consensus.Decider.ParticipantsCount()).
		Msg("[OnPrepare] Received New Prepare Signature")

	//// Write - Start
	if _, err := consensus.Decider.AddNewVote(
		quorum.Prepare, recvMsg.SenderPubkey.Bytes,
		&sign, recvMsg.BlockHash,
		recvMsg.BlockNum, recvMsg.ViewID,
	); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("submit vote prepare failed")
		return
	}
	// Set the bitmap indicating that this validator signed.
	if err := prepareBitmap.SetKey(recvMsg.SenderPubkey.Bytes, true); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[OnPrepare] prepareBitmap.SetKey failed")
		return
	}
	//// Write - End

	//// Read - Start
	if consensus.Decider.IsQuorumAchieved(quorum.Prepare) {
		// NOTE Let it handle its own logs
		if err := consensus.didReachPrepareQuorum(); err != nil {
			return
		}
		consensus.switchPhase(FBFTCommit, true)
	}
	//// Read - End
}

func (consensus *Consensus) onCommit(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		consensus.getLogger().Debug().Err(err).Msg("[OnCommit] Parse pbft message failed")
		return
	}

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	//// Read - Start
	if !consensus.isRightBlockNumAndViewID(recvMsg) {
		return
	}
	commitBitmap := consensus.commitBitmap

	// has to be called before verifying signature
	quorumWasMet := consensus.Decider.IsQuorumAchieved(quorum.Commit)

	signerCount := consensus.Decider.SignersCount(quorum.Commit)
	//// Read - End

	// Verify the signature on commitPayload is correct
	validatorPubKey, commitSig := recvMsg.SenderPubkey, recvMsg.Payload
	logger := consensus.getLogger().With().
		Str("validatorPubKey", validatorPubKey.Bytes.Hex()).
		Int64("numReceivedSoFar", signerCount).Logger()

	logger.Debug().Msg("[OnCommit] Received new commit message")
	var sign bls.Sign
	if err := sign.Deserialize(commitSig); err != nil {
		logger.Debug().Msg("[OnCommit] Failed to deserialize bls signature")
		return
	}

	// Must have the corresponding block to verify commit signature.
	blockObj := consensus.FBFTLog.GetBlockByHash(recvMsg.BlockHash)
	if blockObj == nil {
		consensus.getLogger().Info().
			Uint64("blockNum", recvMsg.BlockNum).
			Uint64("viewID", recvMsg.ViewID).
			Str("blockHash", recvMsg.BlockHash.Hex()).
			Msg("[OnCommit] Failed finding a matching block for committed message")
		return
	}
	commitPayload := signature.ConstructCommitPayload(consensus.ChainReader,
		blockObj.Epoch(), blockObj.Hash(), blockObj.NumberU64(), blockObj.Header().ViewID().Uint64())
	logger = logger.With().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Logger()

	if !sign.VerifyHash(recvMsg.SenderPubkey.Object, commitPayload) {
		logger.Error().Msg("[OnCommit] Cannot verify commit message")
		return
	}

	//// Write - Start
	// Check for potential double signing
	if consensus.checkDoubleSign(recvMsg) {
		return
	}
	if _, err := consensus.Decider.AddNewVote(
		quorum.Commit, recvMsg.SenderPubkey.Bytes,
		&sign, recvMsg.BlockHash,
		recvMsg.BlockNum, recvMsg.ViewID,
	); err != nil {
		return
	}
	// Set the bitmap indicating that this validator signed.
	if err := commitBitmap.SetKey(recvMsg.SenderPubkey.Bytes, true); err != nil {
		consensus.getLogger().Warn().Err(err).
			Msg("[OnCommit] commitBitmap.SetKey failed")
		return
	}
	//// Write - End

	//// Read - Start
	viewID := consensus.viewID

	if consensus.Decider.IsAllSigsCollected() {
		go func(viewID uint64) {
			logger.Info().Msg("[OnCommit] 100% Enough commits received")
			consensus.commitFinishChan <- viewID
		}(viewID)

		consensus.msgSender.StopRetry(msg_pb.MessageType_PREPARED)
		return
	}

	quorumIsMet := consensus.Decider.IsQuorumAchieved(quorum.Commit)
	//// Read - End

	if !quorumWasMet && quorumIsMet {
		logger.Info().Msg("[OnCommit] 2/3 Enough commits received")

		consensus.getLogger().Info().Msg("[OnCommit] Starting Grace Period")
		go func(viewID uint64) {
			time.Sleep(2500 * time.Millisecond)
			logger.Info().Msg("[OnCommit] Commit Grace Period Ended")
			consensus.commitFinishChan <- viewID
		}(viewID)

		consensus.msgSender.StopRetry(msg_pb.MessageType_PREPARED)
	}
}
