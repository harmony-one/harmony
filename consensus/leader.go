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
)

// Announce ..
func (consensus *Consensus) Announce(block *types.Block) {
	blockHash := block.Hash()
	copy(consensus.blockHash[:], blockHash[:])
	// prepare message and broadcast to validators
	encodedBlock, err := rlp.EncodeToBytes(block)
	if err != nil {
		utils.Logger().Debug().Msg("[Announce] Failed encoding block")
		return
	}
	encodedBlockHeader, err := rlp.EncodeToBytes(block.Header())
	if err != nil {
		utils.Logger().Debug().Msg("[Announce] Failed encoding block header")
		return
	}

	consensus.block = encodedBlock
	consensus.blockHeader = encodedBlockHeader

	key, err := consensus.GetConsensusLeaderPrivateKey()
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("[Announce] Node not a leader")
		return
	}
	networkMessage, err := consensus.construct(
		msg_pb.MessageType_ANNOUNCE, nil, key.GetPublicKey(), key,
	)
	if err != nil {
		utils.Logger().Err(err).
			Str("message-type", msg_pb.MessageType_ANNOUNCE.String()).
			Msg("failed constructing message")
		return
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
	num := consensus.BlockNum()
	viewID := consensus.ViewID()
	// Leader sign the block hash itself
	for i, key := range consensus.PubKey.PublicKey {
		if _, err := consensus.Decider.SubmitVote(
			quorum.Prepare,
			key,
			consensus.priKey.PrivateKey[i].SignHash(consensus.blockHash[:]),
			common.BytesToHash(consensus.blockHash[:]),
			num,
			viewID,
		); err != nil {
			return
		}
		if err := consensus.prepareBitmap.SetKey(key, true); err != nil {
			utils.Logger().Warn().Err(err).Msg(
				"[Announce] Leader prepareBitmap SetKey failed",
			)
			return
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
	} else {
		utils.Logger().Info().
			Str("blockHash", block.Hash().Hex()).
			Uint64("blockNum", block.NumberU64()).
			Msg("[Announce] Sent Announce Message!!")
	}

	consensus.switchPhase(FBFTPrepare)
}

func (consensus *Consensus) onPrepare(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[OnPrepare] Unparseable validator message")
		return
	}

	num := consensus.BlockNum()
	viewID := consensus.ViewID()

	if recvMsg.ViewID != viewID || recvMsg.BlockNum != num {
		utils.Logger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnPrepare] Message ViewId or BlockNum not match")
		return
	}

	consensus.Locks.Leader.Lock()
	defer consensus.Locks.Leader.Unlock()

	if !consensus.FBFTLog.HasMatchingViewAnnounce(
		num, viewID, recvMsg.BlockHash,
	) {
		utils.Logger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnPrepare] No Matching Announce message")
		//return
	}

	validatorPubKey := recvMsg.SenderPubkey
	prepareSig := recvMsg.Payload
	prepareBitmap := consensus.prepareBitmap

	logger := utils.Logger().With().Logger()

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
		utils.Logger().Error().Err(err).
			Msg("[OnPrepare] Failed to deserialize bls signature")
		return
	}
	if !sign.VerifyHash(recvMsg.SenderPubkey, consensus.blockHash[:]) {
		utils.Logger().Error().Msg("[OnPrepare] Received invalid BLS signature")
		return
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
		return
	}
	// Set the bitmap indicating that this validator signed.
	if err := prepareBitmap.SetKey(recvMsg.SenderPubkey, true); err != nil {
		utils.Logger().Warn().Err(err).Msg("[OnPrepare] prepareBitmap.SetKey failed")
		return
	}

	if consensus.Decider.IsQuorumAchieved(quorum.Prepare) {
		// NOTE Let it handle its own logs
		if err := consensus.didReachPrepareQuorum(); err != nil {
			return
		}
		consensus.switchPhase(FBFTCommit)
	}
}

func (consensus *Consensus) onCommit(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		utils.Logger().Debug().Err(err).Msg("[OnCommit] Parse pbft message failed")
		return
	}

	consensus.Locks.Leader.Lock()
	defer consensus.Locks.Leader.Unlock()

	// NOTE let it handle its own log
	if !consensus.isRightBlockNumAndViewID(recvMsg) {
		return
	}

	// Check for potential double signing
	if consensus.checkDoubleSign(recvMsg) {
		return
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
		return
	}

	commitPayload := signature.ConstructCommitPayload(
		consensus.ChainReader,
		new(big.Int).SetUint64(consensus.Epoch()),
		recvMsg.BlockHash,
		recvMsg.BlockNum, consensus.ViewID(),
	)
	logger = logger.With().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Logger()

	if !sign.VerifyHash(recvMsg.SenderPubkey, commitPayload) {
		logger.Error().Msg("[OnCommit] Cannot verify commit message")
		return
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
		return
	}
	// Set the bitmap indicating that this validator signed.
	if err := commitBitmap.SetKey(recvMsg.SenderPubkey, true); err != nil {
		utils.Logger().Warn().Err(err).
			Msg("[OnCommit] commitBitmap.SetKey failed")
		return
	}

	myViewID := consensus.ViewID()

	// Reading from this commitfinish chan should be own thread

	// quorumIsMet := consensus.Decider.IsQuorumAchieved(quorum.Commit)

	// if !quorumWasMet && quorumIsMet {
	// 	logger.Info().Msg("[OnCommit] 2/3 Enough commits received")
	// 	nextDue := consensus.NextBlockDue
	// 	go func() {
	// 		utils.Logger().Debug().Msg("[OnCommit] Starting Grace Period")
	// 		// Always wait for 2 seconds as minimum grace period
	// 		time.Sleep(2 * time.Second)
	// 		if n := time.Now(); n.Before(nextDue) {
	// 			// Sleep to wait for the full block time
	// 			time.Sleep(nextDue.Sub(n))
	// 		}
	// 		logger.Debug().Msg("[OnCommit] Commit Grace Period Ended")
	// 		consensus.commitFinishChan <- myViewID
	// 	}()
	// }

	if consensus.Decider.IsAllSigsCollected() {
		consensus.CommitFinishChan <- myViewID
		logger.Info().Msg("[OnCommit] 100% Enough commits received")
	}
}
