package consensus

import (
	"time"

	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"

	"github.com/ethereum/go-ethereum/rlp"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
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

	copy(consensus.blockHash[:], blockHash[:])
	consensus.block = encodedBlock // Must set block bytes before consensus.construct()

	key, err := consensus.getConsensusLeaderPrivateKey()
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[Announce] Node not a leader")
		return
	}

	networkMessage, err := consensus.construct(msg_pb.MessageType_ANNOUNCE, nil, []*bls.PrivateKeyWrapper{key})
	if err != nil {
		consensus.getLogger().Err(err).
			Str("message-type", msg_pb.MessageType_ANNOUNCE.String()).
			Msg("failed constructing message")
		return
	}
	msgToSend, FPBTMsg := networkMessage.Bytes, networkMessage.FBFTMsg

	consensus.FBFTLog.AddVerifiedMessage(FPBTMsg)
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
			[]*bls.PublicKeyWrapper{key.Pub},
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
		consensus.getBlockNum(), msg_pb.MessageType_ANNOUNCE, []nodeconfig.GroupID{
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

	consensus.switchPhase("Announce", FBFTPrepare)
}

func (consensus *Consensus) onPrepare(recvMsg *FBFTMessage) {
	// TODO(audit): make FBFT lookup using map instead of looping through all items.
	if !consensus.FBFTLog.HasMatchingViewAnnounce(
		consensus.getBlockNum(), consensus.getCurBlockViewID(), recvMsg.BlockHash,
	) {
		consensus.getLogger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnPrepare] No Matching Announce message")
	}
	if !consensus.isRightBlockNumAndViewID(recvMsg) {
		return
	}

	blockHash := consensus.blockHash[:]
	prepareBitmap := consensus.prepareBitmap
	// proceed only when the message is not received before
	for _, signer := range recvMsg.SenderPubkeys {
		signed := consensus.Decider.ReadBallot(quorum.Prepare, signer.Bytes)
		if signed != nil {
			consensus.getLogger().Debug().
				Str("validatorPubKey", signer.Bytes.Hex()).
				Msg("[OnPrepare] Already Received prepare message from the validator")
			return
		}
	}

	if consensus.Decider.IsQuorumAchieved(quorum.Prepare) {
		// already have enough signatures
		consensus.getLogger().Debug().
			Interface("validatorPubKeys", recvMsg.SenderPubkeys).
			Msg("[OnPrepare] Received Additional Prepare Message")
		return
	}
	signerCount := consensus.Decider.SignersCount(quorum.Prepare)
	//// Read - End

	// Check BLS signature for the multi-sig
	prepareSig := recvMsg.Payload
	var sign bls_core.Sign
	err := sign.Deserialize(prepareSig)
	if err != nil {
		consensus.getLogger().Error().Err(err).
			Msg("[OnPrepare] Failed to deserialize bls signature")
		return
	}
	signerPubKey := &bls_core.PublicKey{}
	if recvMsg.HasSingleSender() {
		signerPubKey = recvMsg.SenderPubkeys[0].Object
	} else {
		for _, pubKey := range recvMsg.SenderPubkeys {
			signerPubKey.Add(pubKey.Object)
		}
	}
	if !sign.VerifyHash(signerPubKey, blockHash) {
		consensus.getLogger().
			Error().
			Msgf(
				"[OnPrepare] Received invalid BLS signature: hash %s, key %s",
				common.BytesToHash(blockHash).Hex(),
				signerPubKey.SerializeToHexStr(),
			)
		return
	}

	consensus.getLogger().Debug().
		Int64("NumReceivedSoFar", signerCount).
		Int64("PublicKeys", consensus.Decider.ParticipantsCount()).
		Msg("[OnPrepare] Received New Prepare Signature")

	//// Write - Start
	if _, err := consensus.Decider.AddNewVote(
		quorum.Prepare, recvMsg.SenderPubkeys,
		&sign, recvMsg.BlockHash,
		recvMsg.BlockNum, recvMsg.ViewID,
	); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("submit vote prepare failed")
		return
	}
	// Set the bitmap indicating that this validator signed.
	if err := prepareBitmap.SetKeysAtomic(recvMsg.SenderPubkeys, true); err != nil {
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
		consensus.switchPhase("onPrepare", FBFTCommit)
	}
	//// Read - End
}

func (consensus *Consensus) onCommit(recvMsg *FBFTMessage) {
	//// Read - Start
	if !consensus.isRightBlockNumAndViewID(recvMsg) {
		return
	}
	// proceed only when the message is not received before
	for _, signer := range recvMsg.SenderPubkeys {
		signed := consensus.Decider.ReadBallot(quorum.Commit, signer.Bytes)
		if signed != nil {
			consensus.getLogger().Debug().
				Str("validatorPubKey", signer.Bytes.Hex()).
				Msg("[OnCommit] Already Received commit message from the validator")
			return
		}
	}

	commitBitmap := consensus.commitBitmap

	// has to be called before verifying signature
	quorumWasMet := consensus.Decider.IsQuorumAchieved(quorum.Commit)

	signerCount := consensus.Decider.SignersCount(quorum.Commit)
	//// Read - End

	// Verify the signature on commitPayload is correct
	logger := consensus.getLogger().With().
		Str("recvMsg", recvMsg.String()).
		Int64("numReceivedSoFar", signerCount).Logger()

	logger.Debug().Msg("[OnCommit] Received new commit message")
	var sign bls_core.Sign
	if err := sign.Deserialize(recvMsg.Payload); err != nil {
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
	commitPayload := signature.ConstructCommitPayload(consensus.Blockchain().Config(),
		blockObj.Epoch(), blockObj.Hash(), blockObj.NumberU64(), blockObj.Header().ViewID().Uint64())
	logger = logger.With().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Logger()

	signerPubKey := &bls_core.PublicKey{}
	if recvMsg.HasSingleSender() {
		signerPubKey = recvMsg.SenderPubkeys[0].Object
	} else {
		for _, pubKey := range recvMsg.SenderPubkeys {
			signerPubKey.Add(pubKey.Object)
		}
	}
	if !sign.VerifyHash(signerPubKey, commitPayload) {
		logger.Error().Msg("[OnCommit] Cannot verify commit message")
		return
	}

	//// Write - Start
	// Check for potential double signing

	// FIXME (leo): failed view change, will comeback later
	/*
		if consensus.checkDoubleSign(recvMsg) {
			return
		}
	*/
	if _, err := consensus.Decider.AddNewVote(
		quorum.Commit, recvMsg.SenderPubkeys,
		&sign, recvMsg.BlockHash,
		recvMsg.BlockNum, recvMsg.ViewID,
	); err != nil {
		return
	}
	// Set the bitmap indicating that this validator signed.
	if err := commitBitmap.SetKeysAtomic(recvMsg.SenderPubkeys, true); err != nil {
		consensus.getLogger().Warn().Err(err).
			Msg("[OnCommit] commitBitmap.SetKey failed")
		return
	}
	//// Write - End

	//// Read - Start
	viewID := consensus.getCurBlockViewID()

	if consensus.Decider.IsAllSigsCollected() {
		logger.Info().Msg("[OnCommit] 100% Enough commits received")
		consensus.finalCommit()

		consensus.msgSender.StopRetry(msg_pb.MessageType_PREPARED)
		return
	}

	quorumIsMet := consensus.Decider.IsQuorumAchieved(quorum.Commit)
	//// Read - End

	if !quorumWasMet && quorumIsMet {
		logger.Info().Msg("[OnCommit] 2/3 Enough commits received")
		consensus.FBFTLog.MarkBlockVerified(blockObj)

		if !blockObj.IsLastBlockInEpoch() {
			// only do early commit if it's not epoch block to avoid problems
			consensus.preCommitAndPropose(blockObj)
		}

		go func(viewID uint64) {
			waitTime := 1000 * time.Millisecond
			maxWaitTime := time.Until(consensus.NextBlockDue) - 200*time.Millisecond
			if maxWaitTime > waitTime {
				waitTime = maxWaitTime
			}
			consensus.getLogger().Info().Str("waitTime", waitTime.String()).
				Msg("[OnCommit] Starting Grace Period")
			time.Sleep(waitTime)
			logger.Info().Msg("[OnCommit] Commit Grace Period Ended")

			consensus.mutex.Lock()
			defer consensus.mutex.Unlock()
			if viewID == consensus.getCurBlockViewID() {
				consensus.finalCommit()
			}
		}(viewID)

		consensus.msgSender.StopRetry(msg_pb.MessageType_PREPARED)
	}
}
