package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"
	vrf_bls "github.com/harmony-one/harmony/crypto/vrf/bls"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/vdf/src/vdf_go"
)

// handleMessageUpdate will update the consensus state according to received message
func (consensus *Consensus) handleMessageUpdate(payload []byte) {
	if len(payload) == 0 {
		return
	}
	msg := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, msg)
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("Failed to unmarshal message payload.")
		return
	}

	// when node is in ViewChanging mode, it still accepts normal messages into FBFTLog
	// in order to avoid possible trap forever but drop PREPARE and COMMIT
	// which are message types specifically for a node acting as leader
	switch {
	case (consensus.current.Mode() == ViewChanging) &&
		(msg.Type == msg_pb.MessageType_PREPARE || msg.Type == msg_pb.MessageType_COMMIT):
		return
	case consensus.current.Mode() == Listening:
		return
	}

	if msg.Type == msg_pb.MessageType_VIEWCHANGE || msg.Type == msg_pb.MessageType_NEWVIEW {
		if msg.GetViewchange() != nil && msg.GetViewchange().ShardId != consensus.ShardID {
			consensus.getLogger().Warn().
				Uint32("myShardId", consensus.ShardID).
				Uint32("receivedShardId", msg.GetViewchange().ShardId).
				Msg("Received view change message from different shard")
			return
		}
	} else {
		if msg.GetConsensus() != nil && msg.GetConsensus().ShardId != consensus.ShardID {
			consensus.getLogger().Warn().
				Uint32("myShardId", consensus.ShardID).
				Uint32("receivedShardId", msg.GetConsensus().ShardId).
				Msg("Received consensus message from different shard")
			return
		}
	}

	notMemberButStillCatchup := !consensus.Decider.AmIMemberOfCommitee() &&
		msg.Type == msg_pb.MessageType_COMMITTED

	if notMemberButStillCatchup {
		consensus.onCommitted(msg)
		return
	}

	switch msg.Type {
	case msg_pb.MessageType_ANNOUNCE:
		consensus.onAnnounce(msg)
	case msg_pb.MessageType_PREPARE:
		consensus.onPrepare(msg)
	case msg_pb.MessageType_PREPARED:
		consensus.onPrepared(msg)
	case msg_pb.MessageType_COMMIT:
		consensus.onCommit(msg)
	case msg_pb.MessageType_COMMITTED:
		consensus.onCommitted(msg)
	case msg_pb.MessageType_VIEWCHANGE:
		consensus.onViewChange(msg)
	case msg_pb.MessageType_NEWVIEW:
		consensus.onNewView(msg)
	}
}

// TODO: move to consensus_leader.go later
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
	msgToSend := consensus.constructAnnounceMessage()

	// save announce message to FBFTLog
	msgPayload, _ := proto.GetConsensusMessagePayload(msgToSend)
	// TODO(chao): don't unmarshall the message here and direclty pass the original object.
	msg := &msg_pb.Message{}
	_ = protobuf.Unmarshal(msgPayload, msg)
	FPBTMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[Announce] Unable to parse FPBT message")
		return
	}

	// TODO(chao): review FPBT log data structure
	consensus.FBFTLog.AddMessage(FPBTMsg)
	consensus.getLogger().Debug().
		Str("MsgBlockHash", FPBTMsg.BlockHash.Hex()).
		Uint64("MsgViewID", FPBTMsg.ViewID).
		Uint64("MsgBlockNum", FPBTMsg.BlockNum).
		Msg("[Announce] Added Announce message in FPBT")
	consensus.FBFTLog.AddBlock(block)

	// Leader sign the block hash itself
	consensus.Decider.AddSignature(
		quorum.Prepare, consensus.PubKey, consensus.priKey.SignHash(consensus.blockHash[:]),
	)
	if err := consensus.prepareBitmap.SetKey(consensus.PubKey, true); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[Announce] Leader prepareBitmap SetKey failed")
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

func (consensus *Consensus) onAnnounce(msg *msg_pb.Message) {
	consensus.getLogger().Debug().Msg("[OnAnnounce] Receive announce message")
	if consensus.IsLeader() && consensus.current.Mode() == Normal {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("[OnAnnounce] VerifySenderKey failed")
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) &&
		consensus.current.Mode() == Normal && !consensus.ignoreViewIDCheck {
		consensus.getLogger().Warn().
			Str("senderKey", senderKey.SerializeToHexStr()).
			Str("leaderKey", consensus.LeaderPubKey.SerializeToHexStr()).
			Msg("[OnAnnounce] SenderKey does not match leader PubKey")
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Error().Err(err).Msg("[OnAnnounce] Failed to verify leader signature")
		return
	}

	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		consensus.getLogger().Error().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnAnnounce] Unparseable leader message")
		return
	}

	// verify validity of block header object
	// TODO: think about just sending the block hash instead of the header.
	encodedHeader := recvMsg.Payload
	header := new(block.Header)
	err = rlp.DecodeBytes(encodedHeader, header)
	if err != nil {
		consensus.getLogger().Warn().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnAnnounce] Unparseable block header data")
		return
	}

	if recvMsg.BlockNum < consensus.blockNum || recvMsg.BlockNum != header.Number().Uint64() {
		consensus.getLogger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("hdrBlockNum", header.Number().Uint64()).
			Uint64("consensuBlockNum", consensus.blockNum).
			Msg("[OnAnnounce] BlockNum does not match")
		return
	}
	if consensus.current.Mode() == Normal {
		if err = chain.Engine.VerifyHeader(consensus.ChainReader, header, true); err != nil {
			consensus.getLogger().Warn().
				Err(err).
				Str("inChain", consensus.ChainReader.CurrentHeader().Number().String()).
				Str("MsgBlockNum", header.Number().String()).
				Msg("[OnAnnounce] Block content is not verified successfully")
			return
		}

		//VRF/VDF is only generated in the beach chain
		if consensus.NeedsRandomNumberGeneration(header.Epoch()) {
			//validate the VRF with proof if a non zero VRF is found in header
			if len(header.Vrf()) > 0 {
				if !consensus.ValidateVrfAndProof(header) {
					return
				}
			}

			//validate the VDF with proof if a non zero VDF is found in header
			if len(header.Vdf()) > 0 {
				if !consensus.ValidateVdfAndProof(header) {
					return
				}
			}
		}
	}

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
				viewID := consensus.current.ViewID()
				consensus.startViewChange(viewID + 1)
			} else {
				consensus.startViewChange(consensus.viewID + 1)
			}
		}
		consensus.getLogger().Debug().
			Str("leaderKey", consensus.LeaderPubKey.SerializeToHexStr()).
			Msg("[OnAnnounce] Announce message received again")
		//return
	}

	consensus.getLogger().Debug().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Msg("[OnAnnounce] Announce message Added")
	consensus.FBFTLog.AddMessage(recvMsg)

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	consensus.blockHash = recvMsg.BlockHash

	// we have already added message and block, skip check viewID and send prepare message if is in ViewChanging mode
	if consensus.current.Mode() == ViewChanging {
		consensus.getLogger().Debug().Msg("[OnAnnounce] Still in ViewChanging Mode, Exiting !!")
		return
	}

	if consensus.checkViewID(recvMsg) != nil {
		if consensus.current.Mode() == Normal {
			consensus.getLogger().Debug().
				Uint64("MsgViewID", recvMsg.ViewID).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg("[OnAnnounce] ViewID check failed")
		}
		return
	}
	consensus.prepare()

	return
}

// tryPrepare will try to send prepare message
func (consensus *Consensus) prepare() {
	// Construct and send prepare message
	msgToSend := consensus.constructPrepareMessage()
	// TODO: this will not return immediatey, may block

	if err := consensus.msgSender.SendWithoutRetry([]nodeconfig.GroupID{nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[OnAnnounce] Cannot send prepare message")
	} else {
		consensus.getLogger().Info().
			Str("blockHash", hex.EncodeToString(consensus.blockHash[:])).
			Msg("[OnAnnounce] Sent Prepare Message!!")
	}
	consensus.getLogger().Debug().
		Str("From", consensus.phase.String()).
		Str("To", FBFTPrepare.String()).
		Msg("[Announce] Switching Phase")
	consensus.switchPhase(FBFTPrepare, true)
}

// TODO: move to consensus_leader.go later
func (consensus *Consensus) onPrepare(msg *msg_pb.Message) {
	if !consensus.IsLeader() {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("[OnPrepare] VerifySenderKey failed")
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Error().Err(err).Msg("[OnPrepare] Failed to verify sender's signature")
		return
	}

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
	signed := consensus.Decider.ReadSignature(quorum.Prepare, validatorPubKey)
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
	consensus.Decider.AddSignature(quorum.Prepare, validatorPubKey, &sign)
	// Set the bitmap indicating that this validator signed.
	if err := prepareBitmap.SetKey(recvMsg.SenderPubkey, true); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[OnPrepare] prepareBitmap.SetKey failed")
		return
	}

	if consensus.Decider.IsQuorumAchieved(quorum.Prepare) {
		logger.Debug().Msg("[OnPrepare] Received Enough Prepare Signatures")
		// Construct and broadcast prepared message
		msgToSend, aggSig := consensus.constructPreparedMessage()
		consensus.aggregatedPrepareSig = aggSig

		//leader adds prepared message to log
		// TODO(chao): don't unmarshall the payload again
		msgPayload, _ := proto.GetConsensusMessagePayload(msgToSend)
		msg := &msg_pb.Message{}
		_ = protobuf.Unmarshal(msgPayload, msg)
		FBFTMsg, err := ParseFBFTMessage(msg)

		if err != nil {
			consensus.getLogger().Warn().Err(err).Msg("[OnPrepare] Unable to parse pbft message")
			return
		}

		consensus.FBFTLog.AddMessage(FBFTMsg)
		// Leader add commit phase signature
		blockNumHash := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumHash, consensus.blockNum)
		commitPayload := append(blockNumHash, consensus.blockHash[:]...)
		consensus.Decider.AddSignature(
			quorum.Commit, consensus.PubKey, consensus.priKey.SignHash(commitPayload),
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
	return
}

func (consensus *Consensus) onPrepared(msg *msg_pb.Message) {
	consensus.getLogger().Debug().Msg("[OnPrepared] Received Prepared message")
	if consensus.IsLeader() && consensus.current.Mode() == Normal {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		consensus.getLogger().Debug().Err(err).Msg("[OnPrepared] VerifySenderKey failed")
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) &&
		consensus.current.Mode() == Normal && !consensus.ignoreViewIDCheck {
		consensus.getLogger().Warn().Msg("[OnPrepared] SenderKey not match leader PubKey")
		return
	}
	if err := verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Debug().Err(err).Msg("[OnPrepared] Failed to verify sender's signature")
		return
	}

	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		consensus.getLogger().Debug().Err(err).Msg("[OnPrepared] Unparseable validator message")
		return
	}
	consensus.getLogger().Info().
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Uint64("MsgViewID", recvMsg.ViewID).
		Msg("[OnPrepared] Received prepared message")

	if recvMsg.BlockNum < consensus.blockNum {
		consensus.getLogger().Debug().Uint64("MsgBlockNum", recvMsg.BlockNum).Msg("Old Block Received, ignoring!!")
		return
	}

	// check validity of prepared signature
	blockHash := recvMsg.BlockHash
	aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("ReadSignatureBitmapPayload failed!!")
		return
	}

	if !consensus.Decider.IsQuorumAchievedByMask(mask, true) {
		consensus.getLogger().Warn().
			Msgf("[OnPrepared] Quorum Not achieved")
		return
	}

	if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
		myBlockHash := common.Hash{}
		myBlockHash.SetBytes(consensus.blockHash[:])
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[OnPrepared] failed to verify multi signature for prepare phase")
		return
	}

	// check validity of block
	block := recvMsg.Block
	var blockObj types.Block
	err = rlp.DecodeBytes(block, &blockObj)
	if err != nil {
		consensus.getLogger().Warn().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnPrepared] Unparseable block header data")
		return
	}
	if blockObj.NumberU64() != recvMsg.BlockNum || recvMsg.BlockNum < consensus.blockNum {
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", blockObj.NumberU64()).
			Msg("[OnPrepared] BlockNum not match")
		return
	}
	if blockObj.Header().Hash() != recvMsg.BlockHash {
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Hex("MsgBlockHash", recvMsg.BlockHash[:]).
			Str("blockObjHash", blockObj.Header().Hash().Hex()).
			Msg("[OnPrepared] BlockHash not match")
		return
	}
	if consensus.current.Mode() == Normal {
		err := chain.Engine.VerifyHeader(consensus.ChainReader, blockObj.Header(), true)
		if err != nil {
			consensus.getLogger().Error().
				Err(err).
				Str("inChain", consensus.ChainReader.CurrentHeader().Number().String()).
				Str("MsgBlockNum", blockObj.Header().Number().String()).
				Msg("[OnPrepared] Block header is not verified successfully")
			return
		}
		if consensus.BlockVerifier == nil {
			// do nothing
		} else if err := consensus.BlockVerifier(&blockObj); err != nil {
			consensus.getLogger().Error().Err(err).Msg("[OnPrepared] Block verification failed")
			return
		}
	}

	consensus.FBFTLog.AddBlock(&blockObj)
	recvMsg.Block = []byte{} // save memory space
	consensus.FBFTLog.AddMessage(recvMsg)
	consensus.getLogger().Debug().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Hex("blockHash", recvMsg.BlockHash[:]).
		Msg("[OnPrepared] Prepared message and block added")

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	consensus.tryCatchup()
	if consensus.current.Mode() == ViewChanging {
		consensus.getLogger().Debug().Msg("[OnPrepared] Still in ViewChanging mode, Exiting!!")
		return
	}

	if consensus.checkViewID(recvMsg) != nil {
		if consensus.current.Mode() == Normal {
			consensus.getLogger().Debug().
				Uint64("MsgViewID", recvMsg.ViewID).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg("[OnPrepared] ViewID check failed")
		}
		return
	}
	if recvMsg.BlockNum > consensus.blockNum {
		consensus.getLogger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Msg("[OnPrepared] Future Block Received, ignoring!!")
		return
	}

	// add block field
	blockPayload := make([]byte, len(block))
	copy(blockPayload[:], block[:])
	consensus.block = blockPayload

	// add preparedSig field
	consensus.aggregatedPrepareSig = aggSig
	consensus.prepareBitmap = mask

	// Optimistically add blockhash field of prepare message
	emptyHash := [32]byte{}
	if bytes.Compare(consensus.blockHash[:], emptyHash[:]) == 0 {
		copy(consensus.blockHash[:], blockHash[:])
	}

	// Construct and send the commit message
	// TODO: should only sign on block hash
	blockNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, consensus.blockNum)
	commitPayload := append(blockNumBytes, consensus.blockHash[:]...)
	msgToSend := consensus.constructCommitMessage(commitPayload)

	// TODO: genesis account node delay for 1 second, this is a temp fix for allows FN nodes to earning reward
	if consensus.delayCommit > 0 {
		time.Sleep(consensus.delayCommit)
	}

	if err := consensus.msgSender.SendWithoutRetry([]nodeconfig.GroupID{nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		consensus.getLogger().Warn().Msg("[OnPrepared] Cannot send commit message!!")
	} else {
		consensus.getLogger().Info().
			Uint64("blockNum", consensus.blockNum).
			Hex("blockHash", consensus.blockHash[:]).
			Msg("[OnPrepared] Sent Commit Message!!")
	}

	consensus.getLogger().Debug().
		Str("From", consensus.phase.String()).
		Str("To", FBFTCommit.String()).
		Msg("[OnPrepared] Switching phase")
	consensus.switchPhase(FBFTCommit, true)

	return
}

// TODO: move it to consensus_leader.go later
func (consensus *Consensus) onCommit(msg *msg_pb.Message) {
	if !consensus.IsLeader() {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		consensus.getLogger().Debug().Msgf("[OnCommit] VerifySenderKey Failed %s", err.Error())
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Debug().Err(err).Msg("[OnCommit] Failed to verify sender's signature")
		return
	}

	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		consensus.getLogger().Debug().Err(err).Msg("[OnCommit] Parse pbft message failed")
		return
	}

	if recvMsg.ViewID != consensus.viewID || recvMsg.BlockNum != consensus.blockNum {
		consensus.getLogger().Debug().
			Uint64("MsgViewID", recvMsg.ViewID).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Str("ValidatorPubKey", recvMsg.SenderPubkey.SerializeToHexStr()).
			Msg("[OnCommit] BlockNum/viewID not match")
		return
	}

	if !consensus.FBFTLog.HasMatchingAnnounce(consensus.blockNum, recvMsg.BlockHash) {
		consensus.getLogger().Debug().
			Hex("MsgBlockHash", recvMsg.BlockHash[:]).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Msg("[OnCommit] Cannot find matching blockhash")
		return
	}

	if !consensus.FBFTLog.HasMatchingPrepared(consensus.blockNum, recvMsg.BlockHash) {
		consensus.getLogger().Debug().
			Hex("blockHash", recvMsg.BlockHash[:]).
			Uint64("blockNum", consensus.blockNum).
			Msg("[OnCommit] Cannot find matching prepared message")
		return
	}

	validatorPubKey := recvMsg.SenderPubkey
	commitSig := recvMsg.Payload
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	logger := consensus.getLogger().With().
		Str("validatorPubKey", validatorPubKey.SerializeToHexStr()).Logger()
	if !consensus.IsValidatorInCommittee(recvMsg.SenderPubkey) {
		logger.Error().Msg("[OnCommit] Invalid validator")
		return
	}

	commitBitmap := consensus.commitBitmap
	// proceed only when the message is not received before
	signed := consensus.Decider.ReadSignature(quorum.Commit, validatorPubKey)

	if signed != nil {
		logger.Debug().
			Msg("[OnCommit] Already received commit message from the validator")
		return
	}

	// has to be called before verifying signature
	quorumWasMet := consensus.Decider.IsQuorumAchieved(quorum.Commit)
	// Verify the signature on commitPayload is correct
	var sign bls.Sign
	err = sign.Deserialize(commitSig)
	if err != nil {
		logger.Debug().Msg("[OnCommit] Failed to deserialize bls signature")
		return
	}
	blockNumHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumHash, recvMsg.BlockNum)
	commitPayload := append(blockNumHash, recvMsg.BlockHash[:]...)
	logger = logger.With().Uint64("MsgViewID", recvMsg.ViewID).Uint64("MsgBlockNum", recvMsg.BlockNum).Logger()
	if !sign.VerifyHash(recvMsg.SenderPubkey, commitPayload) {
		logger.Error().Msg("[OnCommit] Cannot verify commit message")
		return
	}

	logger = logger.With().
		Int64("numReceivedSoFar", consensus.Decider.SignersCount(quorum.Commit)).
		Logger()
	logger.Info().Msg("[OnCommit] Received new commit message")
	consensus.Decider.AddSignature(quorum.Commit, validatorPubKey, &sign)
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

func (consensus *Consensus) finalizeCommits() {
	consensus.getLogger().Info().
		Int64("NumCommits", consensus.Decider.SignersCount(quorum.Commit)).
		Msg("[Finalizing] Finalizing Block")

	beforeCatchupNum := consensus.blockNum
	// Construct committed message
	msgToSend, aggSig := consensus.constructCommittedMessage()
	consensus.aggregatedCommitSig = aggSig // this may not needed

	// leader adds committed message to log
	msgPayload, _ := proto.GetConsensusMessagePayload(msgToSend)
	msg := &msg_pb.Message{}
	_ = protobuf.Unmarshal(msgPayload, msg)
	pbftMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[FinalizeCommits] Unable to parse pbft message")
		return
	}
	consensus.FBFTLog.AddMessage(pbftMsg)

	// find correct block content
	curBlockHash := consensus.blockHash
	block := consensus.FBFTLog.GetBlockByHash(curBlockHash)
	if block == nil {
		consensus.getLogger().Warn().
			Str("blockHash", hex.EncodeToString(curBlockHash[:])).
			Msg("[FinalizeCommits] Cannot find block by hash")
		return
	}
	consensus.tryCatchup()
	if consensus.blockNum-beforeCatchupNum != 1 {
		consensus.getLogger().Warn().
			Uint64("beforeCatchupBlockNum", beforeCatchupNum).
			Msg("[FinalizeCommits] Leader cannot provide the correct block for committed message")
		return
	}

	consensus.ChainReader.WriteLastCommits(pbftMsg.Payload)

	// if leader success finalize the block, send committed message to validators
	if err := consensus.msgSender.SendWithRetry(
		block.NumberU64(),
		msg_pb.MessageType_COMMITTED, []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
		host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[Finalizing] Cannot send committed message")
	} else {
		consensus.getLogger().Info().
			Hex("blockHash", curBlockHash[:]).
			Uint64("blockNum", consensus.blockNum).
			Msg("[Finalizing] Sent Committed Message")
	}

	consensus.reportMetrics(*block)

	// Dump new block into level db
	// In current code, we add signatures in block in tryCatchup, the block dump to explorer does not contains signatures
	// but since explorer doesn't need signatures, it should be fine
	// in future, we will move signatures to next block
	//explorer.GetStorageInstance(consensus.leader.IP, consensus.leader.Port, true).Dump(block, beforeCatchupNum)

	if consensus.consensusTimeout[timeoutBootstrap].IsActive() {
		consensus.consensusTimeout[timeoutBootstrap].Stop()
		consensus.getLogger().Debug().Msg("[Finalizing] Start consensus timer; stop bootstrap timer only once")
	} else {
		consensus.getLogger().Debug().Msg("[Finalizing] Start consensus timer")
	}
	consensus.consensusTimeout[timeoutConsensus].Start()

	consensus.getLogger().Info().
		Uint64("blockNum", block.NumberU64()).
		Uint64("epochNum", block.Epoch().Uint64()).
		Uint64("ViewId", block.Header().ViewID().Uint64()).
		Str("blockHash", block.Hash().String()).
		Int("index", consensus.Decider.IndexOf(consensus.PubKey)).
		Int("numTxns", len(block.Transactions())).
		Int("numStakingTxns", len(block.StakingTransactions())).
		Msg("HOORAY!!!!!!! CONSENSUS REACHED!!!!!!!")
	// Send signal to Node so the new block can be added and new round of consensus can be triggered
	consensus.ReadySignal <- struct{}{}
}

func (consensus *Consensus) onCommitted(msg *msg_pb.Message) {
	consensus.getLogger().Debug().Msg("[OnCommitted] Receive committed message")

	if consensus.IsLeader() && consensus.current.Mode() == Normal {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[OnCommitted] verifySenderKey failed")
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) &&
		consensus.current.Mode() == Normal && !consensus.ignoreViewIDCheck {
		consensus.getLogger().Warn().Msg("[OnCommitted] senderKey not match leader PubKey")
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[OnCommitted] Failed to verify sender's signature")
		return
	}

	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		consensus.getLogger().Warn().Msg("[OnCommitted] unable to parse msg")
		return
	}

	if recvMsg.BlockNum < consensus.blockNum {
		consensus.getLogger().Info().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Msg("[OnCommitted] Received Old Blocks!!")
		return
	}

	aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("[OnCommitted] readSignatureBitmapPayload failed")
		return
	}

	if !consensus.Decider.IsQuorumAchievedByMask(mask, true) {
		consensus.getLogger().Warn().
			Msgf("[OnCommitted] Quorum Not achieved")
		return
	}

	blockNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, recvMsg.BlockNum)
	commitPayload := append(blockNumBytes, recvMsg.BlockHash[:]...)
	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		consensus.getLogger().Error().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnCommitted] Failed to verify the multi signature for commit phase")
		return
	}

	consensus.FBFTLog.AddMessage(recvMsg)
	consensus.ChainReader.WriteLastCommits(recvMsg.Payload)
	consensus.getLogger().Debug().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Msg("[OnCommitted] Committed message added")

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	consensus.aggregatedCommitSig = aggSig
	consensus.commitBitmap = mask

	if recvMsg.BlockNum-consensus.blockNum > consensusBlockNumBuffer {
		consensus.getLogger().Debug().Uint64("MsgBlockNum", recvMsg.BlockNum).Msg("[OnCommitted] out of sync")
		go func() {
			select {
			case consensus.blockNumLowChan <- struct{}{}:
				consensus.current.SetMode(Syncing)
				for _, v := range consensus.consensusTimeout {
					v.Stop()
				}
			case <-time.After(1 * time.Second):
			}
		}()
		return
	}

	//	if consensus.checkViewID(recvMsg) != nil {
	//		consensus.getLogger().Debug("viewID check failed", "viewID", recvMsg.ViewID, "myViewID", consensus.viewID)
	//		return
	//	}

	consensus.tryCatchup()
	if consensus.current.Mode() == ViewChanging {
		consensus.getLogger().Debug().Msg("[OnCommitted] Still in ViewChanging mode, Exiting!!")
		return
	}

	if consensus.consensusTimeout[timeoutBootstrap].IsActive() {
		consensus.consensusTimeout[timeoutBootstrap].Stop()
		consensus.getLogger().Debug().Msg("[OnCommitted] Start consensus timer; stop bootstrap timer only once")
	} else {
		consensus.getLogger().Debug().Msg("[OnCommitted] Start consensus timer")
	}
	consensus.consensusTimeout[timeoutConsensus].Start()
	return
}

// LastCommitSig returns the byte array of aggregated commit signature and bitmap of last block
func (consensus *Consensus) LastCommitSig() ([]byte, []byte, error) {
	if consensus.blockNum <= 1 {
		return nil, nil, nil
	}
	lastCommits, err := consensus.ChainReader.ReadLastCommits()
	if err != nil || len(lastCommits) < 96 {
		msgs := consensus.FBFTLog.GetMessagesByTypeSeq(msg_pb.MessageType_COMMITTED, consensus.blockNum-1)
		if len(msgs) != 1 {
			consensus.getLogger().Error().
				Int("numCommittedMsg", len(msgs)).
				Msg("GetLastCommitSig failed with wrong number of committed message")
			return nil, nil, ctxerror.New("GetLastCommitSig failed with wrong number of committed message", "numCommittedMsg", len(msgs))
		}
		lastCommits = msgs[0].Payload
	}
	//#### Read payload data from committed msg
	aggSig := make([]byte, 96)
	bitmap := make([]byte, len(lastCommits)-96)
	offset := 0
	copy(aggSig[:], lastCommits[offset:offset+96])
	offset += 96
	copy(bitmap[:], lastCommits[offset:])
	//#### END Read payload data from committed msg
	return aggSig, bitmap, nil
}

// try to catch up if fall behind
func (consensus *Consensus) tryCatchup() {
	consensus.getLogger().Info().Msg("[TryCatchup] commit new blocks")
	//	if consensus.phase != Commit && consensus.mode.Mode() == Normal {
	//		return
	//	}
	currentBlockNum := consensus.blockNum
	for {
		msgs := consensus.FBFTLog.GetMessagesByTypeSeq(msg_pb.MessageType_COMMITTED, consensus.blockNum)
		if len(msgs) == 0 {
			break
		}
		if len(msgs) > 1 {
			consensus.getLogger().Error().
				Int("numMsgs", len(msgs)).
				Msg("[TryCatchup] DANGER!!! we should only get one committed message for a given blockNum")
		}
		consensus.getLogger().Info().Msg("[TryCatchup] committed message found")

		block := consensus.FBFTLog.GetBlockByHash(msgs[0].BlockHash)
		if block == nil {
			break
		}

		if consensus.BlockVerifier == nil {
			// do nothing
		} else if err := consensus.BlockVerifier(block); err != nil {
			consensus.getLogger().Info().Msg("[TryCatchup] block verification failed")
			return
		}

		if block.ParentHash() != consensus.ChainReader.CurrentHeader().Hash() {
			consensus.getLogger().Debug().Msg("[TryCatchup] parent block hash not match")
			break
		}
		consensus.getLogger().Info().Msg("[TryCatchup] block found to commit")

		preparedMsgs := consensus.FBFTLog.GetMessagesByTypeSeqHash(msg_pb.MessageType_PREPARED, msgs[0].BlockNum, msgs[0].BlockHash)
		msg := consensus.FBFTLog.FindMessageByMaxViewID(preparedMsgs)
		if msg == nil {
			break
		}
		consensus.getLogger().Info().Msg("[TryCatchup] prepared message found to commit")

		// TODO(Chao): Explain the reasoning for these code
		consensus.blockHash = [32]byte{}
		consensus.blockNum = consensus.blockNum + 1
		consensus.viewID = msgs[0].ViewID + 1
		consensus.LeaderPubKey = msgs[0].SenderPubkey

		consensus.getLogger().Info().Msg("[TryCatchup] Adding block to chain")
		consensus.OnConsensusDone(block, msgs[0].Payload)
		consensus.ResetState()

		select {
		case consensus.VerifiedNewBlock <- block:
		default:
			consensus.getLogger().Info().
				Str("blockHash", block.Hash().String()).
				Msg("[TryCatchup] consensus verified block send to chan failed")
			continue
		}

		break
	}
	if currentBlockNum < consensus.blockNum {
		consensus.getLogger().Info().
			Uint64("From", currentBlockNum).
			Uint64("To", consensus.blockNum).
			Msg("[TryCatchup] Caught up!")
		consensus.switchPhase(FBFTAnnounce, true)
	}
	// catup up and skip from view change trap
	if currentBlockNum < consensus.blockNum &&
		consensus.current.Mode() == ViewChanging {
		consensus.current.SetMode(Normal)
		consensus.consensusTimeout[timeoutViewChange].Stop()
	}
	// clean up old log
	consensus.FBFTLog.DeleteBlocksLessThan(consensus.blockNum - 1)
	consensus.FBFTLog.DeleteMessagesLessThan(consensus.blockNum - 1)
}

// Start waits for the next new block and run consensus
func (consensus *Consensus) Start(blockChannel chan *types.Block, stopChan chan struct{}, stoppedChan chan struct{}, startChannel chan struct{}) {
	go func() {
		toStart := false
		isInitialLeader := consensus.IsLeader()
		if isInitialLeader {
			consensus.getLogger().Info().Time("time", time.Now()).Msg("[ConsensusMainLoop] Waiting for consensus start")

			// send a signal to indicate it's ready to run consensus
			// this signal is consumed by node object to create a new block and in turn trigger a new consensus on it
			go func() {
				<-startChannel
				toStart = true
				consensus.getLogger().Info().Time("time", time.Now()).Msg("[ConsensusMainLoop] Send ReadySignal")
				consensus.ReadySignal <- struct{}{}
			}()
		}
		consensus.getLogger().Info().Time("time", time.Now()).Msg("[ConsensusMainLoop] Consensus started")
		defer close(stoppedChan)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		consensus.consensusTimeout[timeoutBootstrap].Start()
		consensus.getLogger().Debug().
			Uint64("viewID", consensus.viewID).
			Uint64("blockNum", consensus.blockNum).
			Msg("[ConsensusMainLoop] Start bootstrap timeout (only once)")

		vdfInProgress := false
		for {
			select {
			case <-ticker.C:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] Ticker")
				if toStart == false && isInitialLeader {
					continue
				}
				for k, v := range consensus.consensusTimeout {
					if consensus.current.Mode() == Syncing ||
						consensus.current.Mode() == Listening {
						v.Stop()
					}
					if !v.CheckExpire() {
						continue
					}
					if k != timeoutViewChange {
						consensus.getLogger().Debug().Msg("[ConsensusMainLoop] Ops Consensus Timeout!!!")
						consensus.startViewChange(consensus.viewID + 1)
						break
					} else {
						consensus.getLogger().Debug().Msg("[ConsensusMainLoop] Ops View Change Timeout!!!")
						viewID := consensus.current.ViewID()
						consensus.startViewChange(viewID + 1)
						break
					}
				}
			case <-consensus.syncReadyChan:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] syncReadyChan")
				consensus.SetBlockNum(consensus.ChainReader.CurrentHeader().Number().Uint64() + 1)
				consensus.SetViewID(consensus.ChainReader.CurrentHeader().ViewID().Uint64() + 1)
				mode := consensus.UpdateConsensusInformation()
				consensus.current.SetMode(mode)
				consensus.getLogger().Info().Str("Mode", mode.String()).Msg("Node is in sync")

			case <-consensus.syncNotReadyChan:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] syncNotReadyChan")
				consensus.SetBlockNum(consensus.ChainReader.CurrentHeader().Number().Uint64() + 1)
				consensus.current.SetMode(Syncing)
				consensus.getLogger().Info().Msg("Node is out of sync")

			case newBlock := <-blockChannel:
				// Debug code to trigger leader change.
				//if consensus.ShardID == 0 && newBlock.NumberU64() == 2 && strings.Contains(consensus.PubKey.SerializeToHexStr(), "65f55eb") {
				//	continue
				//}

				consensus.getLogger().Info().
					Uint64("MsgBlockNum", newBlock.NumberU64()).
					Msg("[ConsensusMainLoop] Received Proposed New Block!")

				//VRF/VDF is only generated in the beacon chain
				if consensus.NeedsRandomNumberGeneration(newBlock.Header().Epoch()) {
					// generate VRF if the current block has a new leader
					if !consensus.ChainReader.IsSameLeaderAsPreviousBlock(newBlock) {
						vrfBlockNumbers, err := consensus.ChainReader.ReadEpochVrfBlockNums(newBlock.Header().Epoch())
						if err != nil {
							consensus.getLogger().Info().
								Uint64("MsgBlockNum", newBlock.NumberU64()).
								Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
								Msg("[ConsensusMainLoop] no VRF block number from local db")
						}

						//check if VRF is already generated for the current block
						vrfAlreadyGenerated := false
						for _, v := range vrfBlockNumbers {
							if v == newBlock.NumberU64() {
								consensus.getLogger().Info().
									Uint64("MsgBlockNum", newBlock.NumberU64()).
									Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
									Msg("[ConsensusMainLoop] VRF is already generated for this block")
								vrfAlreadyGenerated = true
								break
							}
						}

						if !vrfAlreadyGenerated {
							//generate a new VRF for the current block
							vrfBlockNumbers := consensus.GenerateVrfAndProof(newBlock, vrfBlockNumbers)

							//generate a new VDF for the current epoch if there are enough VRFs in the current epoch
							//note that  >= instead of == is used, because it is possible the current leader
							//can commit this block, go offline without finishing VDF
							if (!vdfInProgress) && len(vrfBlockNumbers) >= consensus.VdfSeedSize() {
								//check local database to see if there's a VDF generated for this epoch
								//generate a VDF if no blocknum is available
								_, err := consensus.ChainReader.ReadEpochVdfBlockNum(newBlock.Header().Epoch())
								if err != nil {
									consensus.GenerateVdfAndProof(newBlock, vrfBlockNumbers)
									vdfInProgress = true
								}
							}
						}
					}

					vdfOutput, seed, err := consensus.GetNextRnd()
					if err == nil {
						vdfInProgress = false
						// Verify the randomness
						vdfObject := vdf_go.New(shard.Schedule.VdfDifficulty(), seed)
						if !vdfObject.Verify(vdfOutput) {
							consensus.getLogger().Warn().
								Uint64("MsgBlockNum", newBlock.NumberU64()).
								Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
								Msg("[ConsensusMainLoop] failed to verify the VDF output")
						} else {
							//write the VDF only if VDF has not been generated
							_, err := consensus.ChainReader.ReadEpochVdfBlockNum(newBlock.Header().Epoch())
							if err == nil {
								consensus.getLogger().Info().
									Uint64("MsgBlockNum", newBlock.NumberU64()).
									Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
									Msg("[ConsensusMainLoop] VDF has already been generated previously")
							} else {
								consensus.getLogger().Info().
									Uint64("MsgBlockNum", newBlock.NumberU64()).
									Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
									Msg("[ConsensusMainLoop] Generated a new VDF")

								newBlock.AddVdf(vdfOutput[:])
							}
						}
					} else {
						//consensus.getLogger().Error().Err(err). Msg("[ConsensusMainLoop] Failed to get randomness")
					}
				}

				startTime = time.Now()
				consensus.msgSender.Reset(newBlock.NumberU64())

				consensus.getLogger().Debug().
					Int("numTxs", len(newBlock.Transactions())).
					Time("startTime", startTime).
					Int64("publicKeys", consensus.Decider.ParticipantsCount()).
					Msg("[ConsensusMainLoop] STARTING CONSENSUS")
				consensus.announce(newBlock)

			case msg := <-consensus.MsgChan:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] MsgChan")
				consensus.handleMessageUpdate(msg)

			case viewID := <-consensus.commitFinishChan:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] commitFinishChan")
				// Only Leader execute this condition
				func() {
					consensus.mutex.Lock()
					defer consensus.mutex.Unlock()
					if viewID == consensus.viewID {
						consensus.finalizeCommits()
					}
				}()

			case <-stopChan:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] stopChan")
				return
			}
		}
		consensus.getLogger().Debug().Msg("[ConsensusMainLoop] Ended.")
	}()
}

// GenerateVrfAndProof generates new VRF/Proof from hash of previous block
func (consensus *Consensus) GenerateVrfAndProof(newBlock *types.Block, vrfBlockNumbers []uint64) []uint64 {
	sk := vrf_bls.NewVRFSigner(consensus.priKey)
	blockHash := [32]byte{}
	previousHeader := consensus.ChainReader.GetHeaderByNumber(newBlock.NumberU64() - 1)
	previousHash := previousHeader.Hash()
	copy(blockHash[:], previousHash[:])

	vrf, proof := sk.Evaluate(blockHash[:])
	newBlock.AddVrf(append(vrf[:], proof...))

	consensus.getLogger().Info().
		Uint64("MsgBlockNum", newBlock.NumberU64()).
		Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
		Int("Num of VRF", len(vrfBlockNumbers)).
		Msg("[ConsensusMainLoop] Leader generated a VRF")

	return vrfBlockNumbers
}

// ValidateVrfAndProof validates a VRF/Proof from hash of previous block
func (consensus *Consensus) ValidateVrfAndProof(headerObj *block.Header) bool {
	vrfPk := vrf_bls.NewVRFVerifier(consensus.LeaderPubKey)
	var blockHash [32]byte
	previousHeader := consensus.ChainReader.GetHeaderByNumber(
		headerObj.Number().Uint64() - 1,
	)
	previousHash := previousHeader.Hash()
	copy(blockHash[:], previousHash[:])
	vrfProof := [96]byte{}
	copy(vrfProof[:], headerObj.Vrf()[32:])
	hash, err := vrfPk.ProofToHash(blockHash[:], vrfProof[:])

	if err != nil {
		consensus.getLogger().Warn().
			Err(err).
			Str("MsgBlockNum", headerObj.Number().String()).
			Msg("[OnAnnounce] VRF verification error")
		return false
	}

	if !bytes.Equal(hash[:], headerObj.Vrf()[:32]) {
		consensus.getLogger().Warn().
			Str("MsgBlockNum", headerObj.Number().String()).
			Msg("[OnAnnounce] VRF proof is not valid")
		return false
	}

	vrfBlockNumbers, _ := consensus.ChainReader.ReadEpochVrfBlockNums(
		headerObj.Epoch(),
	)
	consensus.getLogger().Info().
		Str("MsgBlockNum", headerObj.Number().String()).
		Int("Number of VRF", len(vrfBlockNumbers)).
		Msg("[OnAnnounce] validated a new VRF")

	return true
}

// GenerateVdfAndProof generates new VDF/Proof from VRFs in the current epoch
func (consensus *Consensus) GenerateVdfAndProof(newBlock *types.Block, vrfBlockNumbers []uint64) {
	//derive VDF seed from VRFs generated in the current epoch
	seed := [32]byte{}
	for i := 0; i < consensus.VdfSeedSize(); i++ {
		previousVrf := consensus.ChainReader.GetVrfByNumber(vrfBlockNumbers[i])
		for j := 0; j < len(seed); j++ {
			seed[j] = seed[j] ^ previousVrf[j]
		}
	}

	consensus.getLogger().Info().
		Uint64("MsgBlockNum", newBlock.NumberU64()).
		Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
		Int("Num of VRF", len(vrfBlockNumbers)).
		Msg("[ConsensusMainLoop] VDF computation started")

	// TODO ek – limit concurrency
	go func() {
		vdf := vdf_go.New(shard.Schedule.VdfDifficulty(), seed)
		outputChannel := vdf.GetOutputChannel()
		start := time.Now()
		vdf.Execute()
		duration := time.Now().Sub(start)
		consensus.getLogger().Info().
			Dur("duration", duration).
			Msg("[ConsensusMainLoop] VDF computation finished")
		output := <-outputChannel

		// The first 516 bytes are the VDF+proof and the last 32 bytes are XORed VRF as seed
		rndBytes := [548]byte{}
		copy(rndBytes[:516], output[:])
		copy(rndBytes[516:], seed[:])
		consensus.RndChannel <- rndBytes
	}()
}

// ValidateVdfAndProof validates the VDF/proof in the current epoch
func (consensus *Consensus) ValidateVdfAndProof(headerObj *block.Header) bool {
	vrfBlockNumbers, err := consensus.ChainReader.ReadEpochVrfBlockNums(headerObj.Epoch())
	if err != nil {
		consensus.getLogger().Error().Err(err).
			Str("MsgBlockNum", headerObj.Number().String()).
			Msg("[OnAnnounce] failed to read VRF block numbers for VDF computation")
	}

	//extra check to make sure there's no index out of range error
	//it can happen if epoch is messed up, i.e. VDF ouput is generated in the next epoch
	if consensus.VdfSeedSize() > len(vrfBlockNumbers) {
		return false
	}

	seed := [32]byte{}
	for i := 0; i < consensus.VdfSeedSize(); i++ {
		previousVrf := consensus.ChainReader.GetVrfByNumber(vrfBlockNumbers[i])
		for j := 0; j < len(seed); j++ {
			seed[j] = seed[j] ^ previousVrf[j]
		}
	}

	vdfObject := vdf_go.New(shard.Schedule.VdfDifficulty(), seed)
	vdfOutput := [516]byte{}
	copy(vdfOutput[:], headerObj.Vdf())
	if vdfObject.Verify(vdfOutput) {
		consensus.getLogger().Info().
			Str("MsgBlockNum", headerObj.Number().String()).
			Int("Num of VRF", consensus.VdfSeedSize()).
			Msg("[OnAnnounce] validated a new VDF")

	} else {
		consensus.getLogger().Warn().
			Str("MsgBlockNum", headerObj.Number().String()).
			Uint64("Epoch", headerObj.Epoch().Uint64()).
			Int("Num of VRF", consensus.VdfSeedSize()).
			Msg("[OnAnnounce] VDF proof is not valid")
		return false
	}

	return true
}
