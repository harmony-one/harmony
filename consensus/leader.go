package consensus

import (
	"encoding/binary"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p/host"
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
	network, err := consensus.construct(msg_pb.MessageType_ANNOUNCE, nil)
	if err != nil {
		consensus.getLogger().Err(err).
			Str("message-type", msg_pb.MessageType_ANNOUNCE.String()).
			Msg("failed constructing message")
		return
	}
	msgToSend, FPBTMsg := network.Bytes, network.FBFTMsg

	// TODO(chao): review FPBT log data structure
	consensus.FBFTLog.AddMessage(FPBTMsg)
	consensus.getLogger().Debug().
		Str("MsgBlockHash", FPBTMsg.BlockHash.Hex()).
		Uint64("MsgViewID", FPBTMsg.ViewID).
		Uint64("MsgBlockNum", FPBTMsg.BlockNum).
		Msg("[Announce] Added Announce message in FPBT")
	consensus.FBFTLog.AddBlock(block)

	// Leader sign the block hash itself
	consensus.Decider.SubmitVote(
		quorum.Prepare,
		consensus.PubKey,
		consensus.priKey.SignHash(consensus.blockHash[:]),
		nil,
	)
	if err := consensus.prepareBitmap.SetKey(consensus.PubKey, true); err != nil {
		consensus.getLogger().Warn().Err(err).Msg(
			"[Announce] Leader prepareBitmap SetKey failed",
		)
		return
	}

	// Construct broadcast p2p message
	if err := consensus.msgSender.SendWithRetry(
		consensus.blockNum, msg_pb.MessageType_ANNOUNCE, []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
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

	if recvMsg.ViewID != consensus.viewID || recvMsg.BlockNum != consensus.blockNum {
		consensus.getLogger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Msg("[OnPrepare] Message ViewId or BlockNum not match")
		return
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
		return
	}

	if consensus.Decider.IsQuorumAchieved(quorum.Prepare) {
		// already have enough signatures
		logger.Debug().Msg("[OnPrepare] Received Additional Prepare Message")
		return
	}

	// Check BLS signature for the multi-sig
	var sign bls.Sign
	err = sign.Deserialize(prepareSig)
	if err != nil {
		consensus.getLogger().Error().Err(err).
			Msg("[OnPrepare] Failed to deserialize bls signature")
		return
	}
	if !sign.VerifyHash(recvMsg.SenderPubkey, consensus.blockHash[:]) {
		consensus.getLogger().Error().Msg("[OnPrepare] Received invalid BLS signature")
		return
	}

	logger = logger.With().
		Int64("NumReceivedSoFar", consensus.Decider.SignersCount(quorum.Prepare)).
		Int64("PublicKeys", consensus.Decider.ParticipantsCount()).Logger()
	logger.Info().Msg("[OnPrepare] Received New Prepare Signature")
	consensus.Decider.SubmitVote(
		quorum.Prepare, validatorPubKey, &sign, nil,
	)
	// Set the bitmap indicating that this validator signed.
	if err := prepareBitmap.SetKey(recvMsg.SenderPubkey, true); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[OnPrepare] prepareBitmap.SetKey failed")
		return
	}

	if consensus.Decider.IsQuorumAchieved(quorum.Prepare) {
		logger.Debug().Msg("[OnPrepare] Received Enough Prepare Signatures")
		// Construct and broadcast prepared message
		network, err := consensus.construct(msg_pb.MessageType_PREPARED, nil)
		if err != nil {
			consensus.getLogger().Err(err).
				Str("message-type", msg_pb.MessageType_PREPARED.String()).
				Msg("failed constructing message")
			return
		}
		msgToSend, FBFTMsg, aggSig :=
			network.Bytes,
			network.FBFTMsg,
			network.OptionalAggregateSignature

		consensus.aggregatedPrepareSig = aggSig
		consensus.FBFTLog.AddMessage(FBFTMsg)
		// Leader add commit phase signature
		blockNumHash := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumHash, consensus.blockNum)
		commitPayload := append(blockNumHash, consensus.blockHash[:]...)
		consensus.Decider.SubmitVote(
			quorum.Commit,
			consensus.PubKey,
			consensus.priKey.SignHash(commitPayload),
			consensus.block[:],
		)

		if err := consensus.commitBitmap.SetKey(consensus.PubKey, true); err != nil {
			consensus.getLogger().Debug().Msg("[OnPrepare] Leader commit bitmap set failed")
			return
		}

		if err := consensus.msgSender.SendWithRetry(
			consensus.blockNum,
			msg_pb.MessageType_PREPARED, []nodeconfig.GroupID{
				nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
			},
			host.ConstructP2pMessage(byte(17), msgToSend),
		); err != nil {
			consensus.getLogger().Warn().Msg("[OnPrepare] Cannot send prepared message")
		} else {
			consensus.getLogger().Debug().
				Hex("blockHash", consensus.blockHash[:]).
				Uint64("blockNum", consensus.blockNum).
				Msg("[OnPrepare] Sent Prepared Message!!")
		}
		consensus.msgSender.StopRetry(msg_pb.MessageType_ANNOUNCE)
		// Stop retry committed msg of last consensus
		consensus.msgSender.StopRetry(msg_pb.MessageType_COMMITTED)

		consensus.getLogger().Debug().
			Str("From", consensus.phase.String()).
			Str("To", FBFTCommit.String()).
			Msg("[OnPrepare] Switching phase")
		consensus.switchPhase(FBFTCommit, true)
	}
}

func (consensus *Consensus) onCommit(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)

	if err != nil {
		consensus.getLogger().Debug().Err(err).Msg("[OnCommit] Parse pbft message failed")
		return
	}

	// let it handle its own logs
	// TODO(Edgar)(TEMP DISABLE while testing double sign)
	if !consensus.onCommitSanityChecks(recvMsg) {
		return
	}

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	validatorPubKey, commitSig, commitBitmap :=
		recvMsg.SenderPubkey, recvMsg.Payload, consensus.commitBitmap
	logger := consensus.getLogger().With().
		Str("validatorPubKey", validatorPubKey.SerializeToHexStr()).Logger()

	if alreadyCastBallot := consensus.Decider.ReadBallot(
		quorum.Commit, validatorPubKey,
	); alreadyCastBallot != nil {
		logger.Debug().Msg("voter already cast commit message")
		return

		// TODO(Edgar) Still working out double sign
		// var signed, received *types.Block
		// if err := rlp.DecodeBytes(
		// 	alreadyCastBallot.OptSerializedBlock, &signed,
		// ); err != nil {
		// 	// TODO handle
		// }

		// areHeightsEqual := signed.Header().Number().Uint64() == recvMsg.BlockNum
		// areViewIDsEqual := signed.Header().ViewID().Uint64() == recvMsg.ViewID
		// areHeadersEqual := bytes.Compare(
		// 	signed.Hash().Bytes(), recvMsg.BlockHash.Bytes(),
		// ) == 0

		// // If signer already signed, and the block height is the same, then we
		// // need to verify the block hash, and if block hash is different, then
		// // that is a clear case of double signing
		// if areHeightsEqual && areViewIDsEqual && !areHeadersEqual {
		// 	// TODO
		// 	if err := rlp.DecodeBytes(recvMsg.Block, &received); err != nil {
		// 		// TODO Some log
		// 		return
		// 	}

		// 	var doubleSign bls.Sign
		// 	if err := doubleSign.Deserialize(recvMsg.Payload); err != nil {
		// 		return
		// 	}

		// 	go func() {
		// 		consensus.SlashChan <- slash.NewRecord(
		// 			*shard.FromLibBLSPublicKeyUnsafe(validatorPubKey),
		// 			signed.Header(), received.Header(),
		// 			alreadyCastBallot.Signature, &doubleSign,
		// 			consensus.SelfAddress,
		// 		)
		// 	}()
		// }
		// return
	}

	// has to be called before verifying signature
	quorumWasMet := consensus.Decider.IsQuorumAchieved(quorum.Commit)
	// Verify the signature on commitPayload is correct
	var sign bls.Sign
	if err := sign.Deserialize(commitSig); err != nil {
		logger.Debug().Msg("[OnCommit] Failed to deserialize bls signature")
		return
	}

	blockNumHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumHash, recvMsg.BlockNum)
	commitPayload := append(blockNumHash, recvMsg.BlockHash[:]...)
	logger = logger.With().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Logger()

	if !sign.VerifyHash(recvMsg.SenderPubkey, commitPayload) {
		logger.Error().Msg("[OnCommit] Cannot verify commit message")
		return
	}

	logger = logger.With().
		Int64("numReceivedSoFar", consensus.Decider.SignersCount(quorum.Commit)).
		Logger()
	logger.Info().Msg("[OnCommit] Received new commit message")

	consensus.Decider.SubmitVote(
		quorum.Commit, validatorPubKey, &sign, consensus.block,
	)
	// Set the bitmap indicating that this validator signed.
	if err := commitBitmap.SetKey(recvMsg.SenderPubkey, true); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[OnCommit] commitBitmap.SetKey failed")
		return
	}

	quorumIsMet := consensus.Decider.IsQuorumAchieved(quorum.Commit)
	if !quorumWasMet && quorumIsMet {
		logger.Info().Msg("[OnCommit] 2/3 Enough commits received")
		go func(viewID uint64) {
			time.Sleep(2 * time.Second)
			logger.Debug().Msg("[OnCommit] Commit Grace Period Ended")
			consensus.commitFinishChan <- viewID
		}(consensus.viewID)

		consensus.msgSender.StopRetry(msg_pb.MessageType_PREPARED)
	}

	if consensus.Decider.IsRewardThresholdAchieved() {
		go func(viewID uint64) {
			consensus.commitFinishChan <- viewID
			logger.Info().Msg("[OnCommit] 90% Enough commits received")
		}(consensus.viewID)
	}
}
