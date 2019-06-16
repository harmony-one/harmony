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
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// handleMessageUpdate will update the consensus state according to received message
func (consensus *Consensus) handleMessageUpdate(payload []byte) {
	if len(payload) == 0 {
		return
	}
	msg := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, msg)
	if err != nil {
		utils.GetLogger().Error("Failed to unmarshal message payload.", "err", err, "consensus", consensus)
		return
	}

	// when node is in ViewChanging mode, it still accepts normal message into PbftLog to avoid possible trap forever
	// but drop PREPARE and COMMIT which are message types for leader
	if consensus.mode.Mode() == ViewChanging && (msg.Type == msg_pb.MessageType_PREPARE || msg.Type == msg_pb.MessageType_COMMIT) {
		return
	}

	if msg.Type == msg_pb.MessageType_VIEWCHANGE || msg.Type == msg_pb.MessageType_NEWVIEW {
		if msg.GetViewchange() != nil && msg.GetViewchange().ShardId != consensus.ShardID {
			consensus.getLogger().Warn("Received view change message from different shard",
				"myShardId", consensus.ShardID, "receivedShardId", msg.GetViewchange().ShardId)
			return
		}
	} else {
		if msg.GetConsensus() != nil && msg.GetConsensus().ShardId != consensus.ShardID {
			consensus.getLogger().Warn("Received consensus message from different shard",
				"myShardId", consensus.ShardID, "receivedShardId", msg.GetConsensus().ShardId)
			return
		}
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
		consensus.getLogger().Debug("[Announce] Failed encoding block")
		return
	}
	consensus.block = encodedBlock
	msgToSend := consensus.constructAnnounceMessage()
	consensus.getLogger().Debug("[Announce] Switching phase", "From", consensus.phase, "To", Prepare)
	consensus.switchPhase(Prepare, true)

	// save announce message to pbftLog
	msgPayload, _ := proto.GetConsensusMessagePayload(msgToSend)
	msg := &msg_pb.Message{}
	_ = protobuf.Unmarshal(msgPayload, msg)
	pbftMsg, err := ParsePbftMessage(msg)
	if err != nil {
		consensus.getLogger().Warn("[Announce] Unable to parse pbft message", "error", err)
		return
	}

	consensus.pbftLog.AddMessage(pbftMsg)
	consensus.pbftLog.AddBlock(block)

	// Leader sign the block hash itself
	consensus.prepareSigs[consensus.PubKey.SerializeToHexStr()] = consensus.priKey.SignHash(consensus.blockHash[:])

	// Construct broadcast p2p message
	if err := consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		consensus.getLogger().Warn("[Announce] Cannot send announce message", "groupID", p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID)))
	} else {
		consensus.getLogger().Debug("[Announce] Sent Announce Message!!", "BlockHash", block.Hash(), "BlockNum", block.NumberU64())
	}
}

func (consensus *Consensus) onAnnounce(msg *msg_pb.Message) {
	consensus.getLogger().Debug("[OnAnnounce] Receive announce message")
	if consensus.PubKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		consensus.getLogger().Debug("[OnAnnounce] VerifySenderKey failed", "error", err)
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal && !consensus.ignoreViewIDCheck {
		consensus.getLogger().Warn("[OnAnnounce] SenderKey not match leader PubKey", "senderKey", senderKey.SerializeToHexStr(), "leaderKey", consensus.LeaderPubKey.SerializeToHexStr())
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Debug("[OnAnnounce] Failed to verify leader signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		consensus.getLogger().Debug("[OnAnnounce] Unparseable leader message", "error", err, "MsgBlockNum", recvMsg.BlockNum)
		return
	}

	logMsgs := consensus.pbftLog.GetMessagesByTypeSeqView(msg_pb.MessageType_ANNOUNCE, recvMsg.BlockNum, recvMsg.ViewID)
	if len(logMsgs) > 0 {
		if logMsgs[0].BlockHash != recvMsg.BlockHash {
			consensus.getLogger().Debug("[OnAnnounce] Leader is malicious", "leaderKey", consensus.LeaderPubKey.SerializeToHexStr())
			consensus.startViewChange(consensus.viewID + 1)
		}
		return
	}

	consensus.getLogger().Debug("[OnAnnounce] Announce message Added", "MsgViewID", recvMsg.ViewID, "MsgBlockNum", recvMsg.BlockNum)
	consensus.pbftLog.AddMessage(recvMsg)

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	consensus.blockHash = recvMsg.BlockHash

	// we have already added message and block, skip check viewID and send prepare message if is in ViewChanging mode
	if consensus.mode.Mode() == ViewChanging {
		consensus.getLogger().Debug("[OnAnnounce] Still in ViewChanging Mode, Exiting !!")
		return
	}

	if consensus.checkViewID(recvMsg) != nil {
		consensus.getLogger().Debug("[OnAnnounce] ViewID check failed", "MsgViewID", recvMsg.ViewID, "msgBlockNum", recvMsg.BlockNum)
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
	if err := consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		consensus.getLogger().Warn("[OnAnnounce] Cannot send prepare message")
	} else {
		consensus.getLogger().Info("[OnAnnounce] Sent Prepare Message!!", "BlockHash", hex.EncodeToString(consensus.blockHash[:]))
	}
	consensus.getLogger().Debug("[Announce] Switching Phase", "From", consensus.phase, "To", Prepare)
	consensus.switchPhase(Prepare, true)
}

// TODO: move to consensus_leader.go later
func (consensus *Consensus) onPrepare(msg *msg_pb.Message) {
	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		consensus.getLogger().Debug("[OnPrepare] VerifySenderKey failed", "error", err)
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Debug("[OnPrepare] Failed to verify sender's signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		consensus.getLogger().Debug("[OnPrepare] Unparseable validator message", "error", err)
		return
	}

	if recvMsg.ViewID != consensus.viewID || recvMsg.BlockNum != consensus.blockNum {
		consensus.getLogger().Debug("[OnPrepare] Message ViewId or BlockNum not match",
			"MsgViewID", recvMsg.ViewID, "MsgBlockNum", recvMsg.BlockNum)
		return
	}

	if !consensus.pbftLog.HasMatchingViewAnnounce(consensus.blockNum, consensus.viewID, recvMsg.BlockHash) {
		consensus.getLogger().Debug("[OnPrepare] No Matching Announce message", "MsgblockHash", recvMsg.BlockHash, "MsgBlockNum", recvMsg.BlockNum)
		return
	}

	validatorPubKey := recvMsg.SenderPubkey.SerializeToHexStr()

	prepareSig := recvMsg.Payload
	prepareSigs := consensus.prepareSigs
	prepareBitmap := consensus.prepareBitmap

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	if len(prepareSigs) >= consensus.Quorum() {
		// already have enough signatures
		consensus.getLogger().Info("[OnPrepare] Received Additional Prepare Message", "ValidatorPubKey", validatorPubKey)
		return
	}
	// proceed only when the message is not received before
	_, ok := prepareSigs[validatorPubKey]
	if ok {
		consensus.getLogger().Debug("[OnPrepare] Already Received prepare message from the validator", "ValidatorPubKey", validatorPubKey)
		return
	}

	// Check BLS signature for the multi-sig
	var sign bls.Sign
	err = sign.Deserialize(prepareSig)
	if err != nil {
		consensus.getLogger().Error("[OnPrepare] Failed to deserialize bls signature", "ValidatorPubKey", validatorPubKey)
		return
	}
	if !sign.VerifyHash(recvMsg.SenderPubkey, consensus.blockHash[:]) {
		consensus.getLogger().Error("[OnPrepare] Received invalid BLS signature", "ValidatorPubKey", validatorPubKey)
		return
	}

	consensus.getLogger().Debug("[OnPrepare] Received New Prepare Signature", "NumReceivedSoFar", len(prepareSigs), "validatorPubKey", validatorPubKey, "PublicKeys", len(consensus.PublicKeys))
	prepareSigs[validatorPubKey] = &sign
	// Set the bitmap indicating that this validator signed.
	if err := prepareBitmap.SetKey(recvMsg.SenderPubkey, true); err != nil {
		ctxerror.Warn(consensus.getLogger(), err, "[OnPrepare] prepareBitmap.SetKey failed")
	}

	if len(prepareSigs) >= consensus.Quorum() {
		consensus.getLogger().Debug("[OnPrepare] Received Enough Prepare Signatures", "NumReceivedSoFar", len(prepareSigs), "PublicKeys", len(consensus.PublicKeys))
		// Construct and broadcast prepared message
		msgToSend, aggSig := consensus.constructPreparedMessage()
		consensus.aggregatedPrepareSig = aggSig

		// add prepared message to log
		msgPayload, _ := proto.GetConsensusMessagePayload(msgToSend)
		msg := &msg_pb.Message{}
		_ = protobuf.Unmarshal(msgPayload, msg)
		pbftMsg, err := ParsePbftMessage(msg)
		if err != nil {
			consensus.getLogger().Warn("[OnPrepare] Unable to parse pbft message", "error", err)
			return
		}
		consensus.pbftLog.AddMessage(pbftMsg)

		if err := consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
			consensus.getLogger().Warn("[OnPrepare] Cannot send prepared message")
		} else {
			consensus.getLogger().Debug("[OnPrepare] Sent Prepared Message!!", "BlockHash", consensus.blockHash, "BlockNum", consensus.blockNum)
		}

		consensus.getLogger().Debug("[OnPrepare] Switching phase", "From", consensus.phase, "To", Commit)
		consensus.switchPhase(Commit, true)
		// Leader add commit phase signature
		blockNumHash := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumHash, consensus.blockNum)
		commitPayload := append(blockNumHash, consensus.blockHash[:]...)
		consensus.commitSigs[consensus.PubKey.SerializeToHexStr()] = consensus.priKey.SignHash(commitPayload)
		if err := consensus.commitBitmap.SetKey(consensus.PubKey, true); err != nil {
			consensus.getLogger().Debug("[OnPrepare] Leader commit bitmap set failed")
		}
	}
	return
}

func (consensus *Consensus) onPrepared(msg *msg_pb.Message) {
	consensus.getLogger().Debug("[OnPrepared] Received Prepared message")
	if consensus.PubKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		consensus.getLogger().Debug("[OnPrepared] VerifySenderKey failed", "error", err)
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal && !consensus.ignoreViewIDCheck {
		consensus.getLogger().Warn("[OnPrepared] SenderKey not match leader PubKey")
		return
	}
	if err := verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Debug("[OnPrepared] Failed to verify sender's signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		consensus.getLogger().Debug("[OnPrepared] Unparseable validator message", "error", err)
		return
	}
	consensus.getLogger().Info("[OnPrepared] Received prepared message", "MsgBlockNum", recvMsg.BlockNum, "MsgViewID", recvMsg.ViewID)

	if recvMsg.BlockNum < consensus.blockNum {
		consensus.getLogger().Debug("Old Block Received, ignoring!!",
			"MsgBlockNum", recvMsg.BlockNum)
		return
	}

	// check validity of prepared signature
	blockHash := recvMsg.BlockHash
	aggSig, mask, err := consensus.readSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		consensus.getLogger().Error("ReadSignatureBitmapPayload failed!!", "error", err)
		return
	}
	if count := utils.CountOneBits(mask.Bitmap); count < consensus.Quorum() {
		consensus.getLogger().Debug("Not enough signatures in the Prepared msg", "Need", consensus.Quorum(), "Got", count)
		return
	}
	if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
		myBlockHash := common.Hash{}
		myBlockHash.SetBytes(consensus.blockHash[:])
		consensus.getLogger().Warn("[OnPrepared] failed to verify multi signature for prepare phase", "MsgBlockHash", recvMsg.BlockHash, "myBlockHash", myBlockHash)
		return
	}

	// check validity of block
	block := recvMsg.Block
	var blockObj types.Block
	err = rlp.DecodeBytes(block, &blockObj)
	if err != nil {
		consensus.getLogger().Warn("[OnPrepared] Unparseable block header data", "error", err, "MsgBlockNum", recvMsg.BlockNum, "MsgPayloadBlockNum", blockObj.NumberU64())
		return
	}
	if blockObj.NumberU64() != recvMsg.BlockNum || recvMsg.BlockNum < consensus.blockNum {
		consensus.getLogger().Warn("[OnPrepared] BlockNum not match", "MsgBlockNum", recvMsg.BlockNum, "blockNum", blockObj.NumberU64())
		return
	}
	if blockObj.Header().Hash() != recvMsg.BlockHash {
		consensus.getLogger().Warn("[OnPrepared] BlockHash not match", "MsgBlockNum", recvMsg.BlockNum, "MsgBlockHash", recvMsg.BlockHash, "blockObjHash", blockObj.Header().Hash())
		return
	}
	if consensus.mode.Mode() == Normal {
		if err := consensus.VerifyHeader(consensus.ChainReader, blockObj.Header(), false); err != nil {
			consensus.getLogger().Warn("[OnPrepared] Block content is not verified successfully", "error", err, "inChain", consensus.ChainReader.CurrentHeader().Number, "MsgBlockNum", blockObj.Header().Number)
			return
		}
	}

	consensus.getLogger().Debug("[OnPrepared] Prepared message and block added", "MsgViewID", recvMsg.ViewID, "MsgBlockNum", recvMsg.BlockNum, "blockHash", recvMsg.BlockHash)
	consensus.pbftLog.AddBlock(&blockObj)
	recvMsg.Block = []byte{} // save memory space
	consensus.pbftLog.AddMessage(recvMsg)

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	consensus.tryCatchup()
	if consensus.mode.Mode() == ViewChanging {
		consensus.getLogger().Debug("[OnPrepared] Still in ViewChanging mode, Exiting !!")
		return
	}

	if consensus.checkViewID(recvMsg) != nil {
		consensus.getLogger().Debug("[OnPrepared] ViewID check failed", "MsgViewID", recvMsg.ViewID, "MsgBlockNum", recvMsg.BlockNum)
		return
	}
	if recvMsg.BlockNum > consensus.blockNum {
		consensus.getLogger().Debug("[OnPrepared] Future Block Received, ignoring!！",
			"MsgBlockNum", recvMsg.BlockNum)
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
	blockNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, consensus.blockNum)
	commitPayload := append(blockNumBytes, consensus.blockHash[:]...)
	msgToSend := consensus.constructCommitMessage(commitPayload)

	// TODO: genesis account node delay for 1 second, this is a temp fix for allows FN nodes to earning reward
	if consensus.delayCommit > 0 {
		time.Sleep(consensus.delayCommit)
	}

	if err := consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		consensus.getLogger().Warn("[OnPrepared] Cannot send commit message!!")
	} else {
		consensus.getLogger().Debug("[OnPrepared] Sent Commit Message!!", "BlockHash", consensus.blockHash, "BlockNum", consensus.blockNum)
	}

	consensus.getLogger().Debug("[OnPrepared] Switching phase", "From", consensus.phase, "To", Commit)
	consensus.switchPhase(Commit, true)

	return
}

// TODO: move it to consensus_leader.go later
func (consensus *Consensus) onCommit(msg *msg_pb.Message) {
	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		consensus.getLogger().Debug("[OnCommit] VerifySenderKey Failed", "error", err)
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Debug("[OnCommit] Failed to verify sender's signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		consensus.getLogger().Debug("[OnCommit] Parse pbft message failed", "error", err)
		return
	}

	if recvMsg.ViewID != consensus.viewID || recvMsg.BlockNum != consensus.blockNum {
		consensus.getLogger().Debug("[OnCommit] BlockNum/viewID not match", "MsgViewID", recvMsg.ViewID, "MsgBlockNum", recvMsg.BlockNum, "ValidatorPubKey", recvMsg.SenderPubkey.SerializeToHexStr())
		return
	}

	if !consensus.pbftLog.HasMatchingAnnounce(consensus.blockNum, recvMsg.BlockHash) {
		consensus.getLogger().Debug("[OnCommit] Cannot find matching blockhash", "MsgBlockHash", recvMsg.BlockHash, "MsgBlockNum", recvMsg.BlockNum)
		return
	}

	if !consensus.pbftLog.HasMatchingPrepared(consensus.blockNum, recvMsg.BlockHash) {
		consensus.getLogger().Debug("[OnCommit] Cannot find matching prepared message", "blockHash", recvMsg.BlockHash)
		return
	}

	validatorPubKey := recvMsg.SenderPubkey.SerializeToHexStr()

	commitSig := recvMsg.Payload

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	if !consensus.IsValidatorInCommittee(recvMsg.SenderPubkey) {
		consensus.getLogger().Error("[OnCommit] Invalid validator", "validatorPubKey", validatorPubKey)
		return
	}

	commitSigs := consensus.commitSigs
	commitBitmap := consensus.commitBitmap

	// proceed only when the message is not received before
	_, ok := commitSigs[validatorPubKey]
	if ok {
		consensus.getLogger().Info("[OnCommit] Already received commit message from the validator", "validatorPubKey", validatorPubKey)
		return
	}

	quorumWasMet := len(commitSigs) >= consensus.Quorum()

	// Verify the signature on commitPayload is correct
	var sign bls.Sign
	err = sign.Deserialize(commitSig)
	if err != nil {
		consensus.getLogger().Debug("[OnCommit] Failed to deserialize bls signature", "validatorPubKey", validatorPubKey)
		return
	}
	blockNumHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumHash, recvMsg.BlockNum)
	commitPayload := append(blockNumHash, recvMsg.BlockHash[:]...)
	if !sign.VerifyHash(recvMsg.SenderPubkey, commitPayload) {
		consensus.getLogger().Error("[OnCommit] Cannot verify commit message", "MsgViewID", recvMsg.ViewID, "MsgBlockNum", recvMsg.BlockNum)
		return
	}

	consensus.getLogger().Debug("[OnCommit] Received new commit message", "numReceivedSoFar", len(commitSigs), "MsgViewID", recvMsg.ViewID, "MsgBlockNum", recvMsg.BlockNum, "validatorPubKey", validatorPubKey)
	commitSigs[validatorPubKey] = &sign
	// Set the bitmap indicating that this validator signed.
	if err := commitBitmap.SetKey(recvMsg.SenderPubkey, true); err != nil {
		ctxerror.Warn(consensus.getLogger(), err, "[OnCommit] commitBitmap.SetKey failed")
	}

	quorumIsMet := len(commitSigs) >= consensus.Quorum()
	rewardThresholdIsMet := len(commitSigs) >= consensus.RewardThreshold()

	if !quorumWasMet && quorumIsMet {
		consensus.getLogger().Info("[OnCommit] 2/3 Enough commits received", "NumCommits", len(commitSigs))
		go func(viewID uint32) {
			time.Sleep(2 * time.Second)
			consensus.getLogger().Debug("[OnCommit] Commit Grace Period Ended", "NumCommits", len(commitSigs))
			consensus.commitFinishChan <- viewID
		}(consensus.viewID)
	}

	if rewardThresholdIsMet {
		go func(viewID uint32) {
			consensus.commitFinishChan <- viewID
			consensus.getLogger().Debug("[OnCommit] 90% Enough commits received", "NumCommits", len(commitSigs))
		}(consensus.viewID)
	}
}

func (consensus *Consensus) finalizeCommits() {
	consensus.getLogger().Info("[Finalizing] Finalizing Block", "NumCommits", len(consensus.commitSigs))

	// Construct and broadcast committed message
	msgToSend, aggSig := consensus.constructCommittedMessage()
	consensus.aggregatedCommitSig = aggSig

	if err := consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		ctxerror.Warn(consensus.getLogger(), err, "[Finalizing] Cannot send committed message")
	} else {
		consensus.getLogger().Debug("[Finalizing] Sent Committed Message", "BlockHash", consensus.blockHash, "BlockNum", consensus.blockNum)
	}

	consensus.getLogger().Debug("[Finalizing] Switching phase", "From", consensus.phase, "To", Announce)
	consensus.switchPhase(Announce, true)
	var blockObj types.Block
	err := rlp.DecodeBytes(consensus.block, &blockObj)
	if err != nil {
		consensus.getLogger().Debug("[Finalizing] failed to construct the new block after consensus")
	}

	// Sign the block
	blockObj.SetPrepareSig(
		consensus.aggregatedPrepareSig.Serialize(),
		consensus.prepareBitmap.Bitmap)
	blockObj.SetCommitSig(
		consensus.aggregatedCommitSig.Serialize(),
		consensus.commitBitmap.Bitmap)

	select {
	case consensus.VerifiedNewBlock <- &blockObj:
	default:
		consensus.getLogger().Info("[SYNC] Failed to send consensus verified block for state sync", "blockHash", blockObj.Hash())
	}

	consensus.reportMetrics(blockObj)

	// Dump new block into level db.
	explorer.GetStorageInstance(consensus.leader.IP, consensus.leader.Port, true).Dump(&blockObj, consensus.viewID)

	// Reset state to Finished, and clear other data.
	consensus.ResetState()
	consensus.viewID++
	consensus.blockNum++

	if consensus.consensusTimeout[timeoutBootstrap].IsActive() {
		consensus.consensusTimeout[timeoutBootstrap].Stop()
		consensus.getLogger().Debug("[Finalizing] Start consensus timer; stop bootstrap timer only once")
	} else {
		consensus.getLogger().Debug("[Finalizing] Start consensus timer")
	}
	consensus.consensusTimeout[timeoutConsensus].Start()

	consensus.OnConsensusDone(&blockObj)
	consensus.getLogger().Info("HOORAY!!!!!!! CONSENSUS REACHED!!!!!!!", "numOfSignatures", len(consensus.commitSigs), "BlockNum", consensus.blockNum-1, "ViewId", consensus.viewID-1, "BlockHash", blockObj.Hash(), "index", consensus.getIndexOfPubKey(consensus.PubKey))

	// TODO: wait for validators receive committed message; remove this temporary delay
	time.Sleep(time.Second)

	// Send signal to Node so the new block can be added and new round of consensus can be triggered
	consensus.ReadySignal <- struct{}{}
}

func (consensus *Consensus) onCommitted(msg *msg_pb.Message) {
	consensus.getLogger().Debug("[OnCommitted] Receive committed message")

	if consensus.PubKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		consensus.getLogger().Warn("[OnCommitted] verifySenderKey failed", "error", err)
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal && !consensus.ignoreViewIDCheck {
		consensus.getLogger().Warn("[OnCommitted] senderKey not match leader PubKey")
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Warn("[OnCommitted] Failed to verify sender's signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		consensus.getLogger().Warn("[OnCommitted] unable to parse msg", "error", err)
		return
	}
	if recvMsg.BlockNum < consensus.blockNum {
		consensus.getLogger().Info("[OnCommitted] Received Old Blocks!！", "MsgBlockNum", recvMsg.BlockNum)
		return
	}

	aggSig, mask, err := consensus.readSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		consensus.getLogger().Error("[OnCommitted] readSignatureBitmapPayload failed", "error", err)
		return
	}

	// check has 2f+1 signatures
	if count := utils.CountOneBits(mask.Bitmap); count < consensus.Quorum() {
		consensus.getLogger().Warn("[OnCommitted] Not enough signature in committed msg", "need", consensus.Quorum(), "got", count)
		return
	}

	blockNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, recvMsg.BlockNum)
	commitPayload := append(blockNumBytes, recvMsg.BlockHash[:]...)
	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		consensus.getLogger().Error("[OnCommitted] Failed to verify the multi signature for commit phase", "MsgBlockNum", recvMsg.BlockNum)
		return
	}

	consensus.getLogger().Debug("[OnCommitted] Committed message added", "MsgViewID", recvMsg.ViewID, "MsgBlockNum", recvMsg.BlockNum)
	consensus.pbftLog.AddMessage(recvMsg)

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	consensus.aggregatedCommitSig = aggSig
	consensus.commitBitmap = mask

	if recvMsg.BlockNum-consensus.blockNum > consensusBlockNumBuffer {
		consensus.getLogger().Debug("[OnCommitted] out of sync", "MsgBlockNum", recvMsg.BlockNum)
		go func() {
			select {
			case consensus.blockNumLowChan <- struct{}{}:
				consensus.mode.SetMode(Syncing)
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
	if consensus.mode.Mode() == ViewChanging {
		consensus.getLogger().Debug("[OnCommitted] Still in ViewChanging mode, Exiting !!")
		return
	}

	if consensus.consensusTimeout[timeoutBootstrap].IsActive() {
		consensus.consensusTimeout[timeoutBootstrap].Stop()
		consensus.getLogger().Debug("[OnCommitted] Start consensus timer; stop bootstrap timer only once")
	} else {
		consensus.getLogger().Debug("[OnCommitted] Start consensus timer")
	}
	consensus.consensusTimeout[timeoutConsensus].Start()
	return
}

// try to catch up if fall behind
func (consensus *Consensus) tryCatchup() {
	consensus.getLogger().Info("[TryCatchup] commit new blocks")
	//	if consensus.phase != Commit && consensus.mode.Mode() == Normal {
	//		return
	//	}
	currentBlockNum := consensus.blockNum
	for {
		msgs := consensus.pbftLog.GetMessagesByTypeSeq(msg_pb.MessageType_COMMITTED, consensus.blockNum)
		if len(msgs) == 0 {
			break
		}
		if len(msgs) > 1 {
			consensus.getLogger().Error("[TryCatchup] DANGER!!! we should only get one committed message for a given blockNum", "numMsgs", len(msgs))
		}
		consensus.getLogger().Info("[TryCatchup] committed message found")

		block := consensus.pbftLog.GetBlockByHash(msgs[0].BlockHash)
		if block == nil {
			break
		}

		if consensus.BlockVerifier == nil {
			// do nothing
		} else if err := consensus.BlockVerifier(block); err != nil {
			consensus.getLogger().Info("[TryCatchup]block verification faied")
			return
		}

		if block.ParentHash() != consensus.ChainReader.CurrentHeader().Hash() {
			consensus.getLogger().Debug("[TryCatchup] parent block hash not match")
			break
		}
		consensus.getLogger().Info("[TryCatchup] block found to commit")

		preparedMsgs := consensus.pbftLog.GetMessagesByTypeSeqHash(msg_pb.MessageType_PREPARED, msgs[0].BlockNum, msgs[0].BlockHash)
		msg := consensus.pbftLog.FindMessageByMaxViewID(preparedMsgs)
		if msg == nil {
			break
		}
		consensus.getLogger().Info("[TryCatchup] prepared message found to commit")

		consensus.blockHash = [32]byte{}
		consensus.blockNum = consensus.blockNum + 1
		consensus.viewID = msgs[0].ViewID + 1
		consensus.LeaderPubKey = msgs[0].SenderPubkey

		//#### Read payload data from committed msg
		aggSig := make([]byte, 96)
		bitmap := make([]byte, len(msgs[0].Payload)-96)
		offset := 0
		copy(aggSig[:], msgs[0].Payload[offset:offset+96])
		offset += 96
		copy(bitmap[:], msgs[0].Payload[offset:])
		//#### END Read payload data from committed msg

		//#### Read payload data from prepared msg
		prepareSig := make([]byte, 96)
		prepareBitmap := make([]byte, len(msg.Payload)-96)
		offset = 0
		copy(prepareSig[:], msg.Payload[offset:offset+96])
		offset += 96
		copy(prepareBitmap[:], msg.Payload[offset:])
		//#### END Read payload data from committed msg

		// Put the signatures into the block
		block.SetPrepareSig(prepareSig, prepareBitmap)

		block.SetCommitSig(aggSig, bitmap)
		consensus.getLogger().Info("[TryCatchup] Adding block to chain")
		consensus.OnConsensusDone(block)
		consensus.ResetState()

		select {
		case consensus.VerifiedNewBlock <- block:
		default:
			consensus.getLogger().Info("[TryCatchup] consensus verified block send to chan failed", "blockHash", block.Hash())
			continue
		}

		break
	}
	if currentBlockNum < consensus.blockNum {
		consensus.getLogger().Info("[TryCatchup] Catched up!", "From", currentBlockNum, "To", consensus.blockNum)
		consensus.switchPhase(Announce, true)
	}
	// catup up and skip from view change trap
	if currentBlockNum < consensus.blockNum && consensus.mode.Mode() == ViewChanging {
		consensus.mode.SetMode(Normal)
		consensus.consensusTimeout[timeoutViewChange].Stop()
	}
	// clean up old log
	consensus.pbftLog.DeleteBlocksLessThan(consensus.blockNum)
	consensus.pbftLog.DeleteMessagesLessThan(consensus.blockNum)
}

// Start waits for the next new block and run consensus
func (consensus *Consensus) Start(blockChannel chan *types.Block, stopChan chan struct{}, stoppedChan chan struct{}, startChannel chan struct{}) {
	if nodeconfig.GetDefaultConfig().IsLeader() {
		<-startChannel
	}
	go func() {
		consensus.getLogger().Info("[ConsensusMainLoop] Start consensus", "time", time.Now())
		defer close(stoppedChan)
		ticker := time.NewTicker(3 * time.Second)
		consensus.consensusTimeout[timeoutBootstrap].Start()
		consensus.getLogger().Debug("[ConsensusMainLoop] Start bootstrap timeout (only once)", "viewID", consensus.viewID, "block", consensus.blockNum)
		for {
			select {
			case <-ticker.C:
				for k, v := range consensus.consensusTimeout {
					if consensus.mode.Mode() == Syncing {
						v.Stop()
					}
					if !v.CheckExpire() {
						continue
					}
					if k != timeoutViewChange {
						consensus.getLogger().Debug("[ConsensusMainLoop] Ops Consensus Timeout!!!")
						consensus.startViewChange(consensus.viewID + 1)
						break
					} else {
						consensus.getLogger().Debug("[ConsensusMainLoop] Ops View Change Timeout!!!")
						viewID := consensus.mode.ViewID()
						consensus.startViewChange(viewID + 1)
						break
					}
				}
			case <-consensus.syncReadyChan:
				consensus.SetBlockNum(consensus.ChainReader.CurrentHeader().Number.Uint64() + 1)
				consensus.getLogger().Info("Node is in sync")
				consensus.ignoreViewIDCheck = true

			case <-consensus.syncNotReadyChan:
				consensus.SetBlockNum(consensus.ChainReader.CurrentHeader().Number.Uint64() + 1)
				consensus.mode.SetMode(Syncing)
				consensus.getLogger().Info("Node is out of sync")

			case newBlock := <-blockChannel:
				consensus.getLogger().Info("[ConsensusMainLoop] Received Proposed New Block!", "MsgBlockNum", newBlock.NumberU64())
				if consensus.ShardID == 0 {
					// TODO ek/rj - re-enable this after fixing DRand
					//if core.IsEpochBlock(newBlock) { // Only beacon chain do randomness generation
					//	// Receive pRnd from DRG protocol
					//	consensus.getLogger().Debug("[DRG] Waiting for pRnd")
					//	pRndAndBitmap := <-consensus.PRndChannel
					//	consensus.getLogger().Debug("[DRG] Got pRnd", "pRnd", pRndAndBitmap)
					//	pRnd := [32]byte{}
					//	copy(pRnd[:], pRndAndBitmap[:32])
					//	bitmap := pRndAndBitmap[32:]
					//	vrfBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
					//	vrfBitmap.SetMask(bitmap)
					//
					//	// TODO: check validity of pRnd
					//	newBlock.AddVrf(pRnd)
					//}

					rnd, blockHash, err := consensus.GetNextRnd()
					if err == nil {
						// Verify the randomness
						_ = blockHash
						consensus.getLogger().Info("[ConsensusMainLoop] Adding randomness into new block", "rnd", rnd)
						newBlock.AddVdf([258]byte{}) // TODO(HB): add real vdf
					} else {
						//consensus.getLogger().Info("Failed to get randomness", "error", err)
					}
				}

				startTime = time.Now()
				consensus.getLogger().Debug("[ConsensusMainLoop] STARTING CONSENSUS", "numTxs", len(newBlock.Transactions()), "consensus", consensus, "startTime", startTime, "publicKeys", len(consensus.PublicKeys))
				consensus.announce(newBlock)

			case msg := <-consensus.MsgChan:
				consensus.handleMessageUpdate(msg)

			case viewID := <-consensus.commitFinishChan:
				func() {
					consensus.mutex.Lock()
					defer consensus.mutex.Unlock()
					if viewID == consensus.viewID {
						consensus.finalizeCommits()
					}
				}()

			case <-stopChan:
				return
			}
		}
	}()
}
