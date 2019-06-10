package consensus

import (
	"encoding/binary"
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
func (consensus *Consensus) tryAnnounce(block *types.Block) {
	// here we assume the leader should always be update to date
	if block.NumberU64() != consensus.blockNum {
		utils.GetLogger().Debug("tryAnnounce blockNum not match", "blockNum", block.NumberU64(), "myBlockNum", consensus.blockNum)
		return
	}
	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		utils.GetLogger().Debug("tryAnnounce key not match", "myKey", consensus.PubKey, "leaderKey", consensus.LeaderPubKey)
		return
	}
	blockHash := block.Hash()
	copy(consensus.blockHash[:], blockHash[:])

	// prepare message and broadcast to validators
	encodedBlock, err := rlp.EncodeToBytes(block)
	if err != nil {
		utils.GetLogger().Debug("tryAnnounce Failed encoding block")
		return
	}
	consensus.block = encodedBlock
	msgToSend := consensus.constructAnnounceMessage()
	consensus.switchPhase(Prepare, false)

	// save announce message to pbftLog
	msgPayload, _ := proto.GetConsensusMessagePayload(msgToSend)
	msg := &msg_pb.Message{}
	_ = protobuf.Unmarshal(msgPayload, msg)
	pbftMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogger().Warn("tryAnnounce unable to parse pbft message", "error", err)
		return
	}

	consensus.pbftLog.AddMessage(pbftMsg)
	consensus.pbftLog.AddBlock(block)

	// Leader sign the block hash itself
	consensus.prepareSigs[consensus.SelfAddress] = consensus.priKey.SignHash(consensus.blockHash[:])

	// Construct broadcast p2p message
	logger := utils.GetLogger().New("viewID", consensus.viewID, "block", consensus.blockNum, "groupID", p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID)))
	if err := consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		ctxerror.Warn(logger, err, "cannot send announce message")
	} else {
		logger.Debug("sent announce message")
	}
}

func (consensus *Consensus) onAnnounce(msg *msg_pb.Message) {
	utils.GetLogger().Debug("receive announce message", "phase", consensus.phase, "viewID", consensus.viewID, "block", consensus.blockNum)
	if consensus.PubKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		utils.GetLogger().Debug("onAnnounce verifySenderKey failed", "error", err)
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal && !consensus.ignoreViewIDCheck {
		utils.GetLogger().Warn("onAnnounce senderKey not match leader PubKey", "senderKey", senderKey.GetHexString()[:10], "leaderKey", consensus.LeaderPubKey.GetHexString()[:10])
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.GetLogger().Debug("onAnnounce Failed to verify leader signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogger().Debug("onAnnounce Unparseable leader message", "error", err)
		return
	}
	block := recvMsg.Payload

	// check block header is valid
	var blockObj types.Block
	err = rlp.DecodeBytes(block, &blockObj)
	if err != nil {
		utils.GetLogger().Warn("onAnnounce Unparseable block header data", "error", err)
		return
	}

	if blockObj.NumberU64() != recvMsg.BlockNum || recvMsg.BlockNum < consensus.blockNum {
		utils.GetLogger().Warn("blockNum not match", "recvBlockNum", recvMsg.BlockNum, "blockObjNum", blockObj.NumberU64(), "myBlockNum", consensus.blockNum)
		return
	}

	if consensus.mode.Mode() == Normal {
		// skip verify header when node is in Syncing mode
		if err := consensus.VerifyHeader(consensus.ChainReader, blockObj.Header(), false); err != nil {
			utils.GetLogger().Warn("onAnnounce block content is not verified successfully", "error", err, "inChain", consensus.ChainReader.CurrentHeader().Number, "got", blockObj.Header().Number)
			return
		}
	}

	// skip verify block in Syncing mode
	if consensus.BlockVerifier == nil || consensus.mode.Mode() != Normal {
		// do nothing
	} else if err := consensus.BlockVerifier(&blockObj); err != nil {
		// TODO ek – maybe we could do this in commit phase
		err := ctxerror.New("block verification failed",
			"blockHash", blockObj.Hash(),
		).WithCause(err)
		ctxerror.Log15(utils.GetLogger().Warn, err)
		return
	}
	//blockObj.Logger(utils.GetLogger()).Debug("received announce", "viewID", recvMsg.ViewID, "msgBlockNum", recvMsg.BlockNum)
	logMsgs := consensus.pbftLog.GetMessagesByTypeSeqView(msg_pb.MessageType_ANNOUNCE, recvMsg.BlockNum, recvMsg.ViewID)
	if len(logMsgs) > 0 {
		if logMsgs[0].BlockHash != blockObj.Header().Hash() {
			utils.GetLogger().Debug("onAnnounce leader is malicious", "leaderKey", utils.GetBlsAddress(consensus.LeaderPubKey))
			consensus.startViewChange(consensus.viewID + 1)
		}
		return
	}
	blockPayload := make([]byte, len(block))
	copy(blockPayload[:], block[:])
	consensus.block = blockPayload
	consensus.blockHash = recvMsg.BlockHash
	utils.GetLogger().Debug("announce block added", "phase", consensus.phase, "myViewID", consensus.viewID, "myBlock", consensus.blockNum, "msgViewID", recvMsg.ViewID, "msgBlock", recvMsg.BlockNum)
	consensus.pbftLog.AddMessage(recvMsg)
	consensus.pbftLog.AddBlock(&blockObj)

	// we have already added message and block, skip check viewID and send prepare message if is in ViewChanging mode
	if consensus.mode.Mode() == ViewChanging {
		return
	}

	consensus.tryCatchup()

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	if consensus.checkViewID(recvMsg) != nil {
		utils.GetLogger().Debug("viewID check failed", "viewID", recvMsg.ViewID, "myViewID", consensus.viewID)
		return
	}
	consensus.tryPrepare(blockObj.Header().Hash())

	return
}

// tryPrepare will try to send prepare message
func (consensus *Consensus) tryPrepare(blockHash common.Hash) {
	var hash common.Hash
	copy(hash[:], blockHash[:])
	block := consensus.pbftLog.GetBlockByHash(hash)
	if block == nil {
		return
	}

	if consensus.phase != Announce || consensus.blockNum != block.NumberU64() || !consensus.pbftLog.HasMatchingViewAnnounce(consensus.blockNum, consensus.viewID, hash) {
		utils.GetLogger().Debug("not match", "myPhase", consensus.phase, "myBlock", consensus.blockNum, "viewID", consensus.viewID)
		return
	}

	consensus.switchPhase(Prepare, false)

	// Construct and send prepare message
	msgToSend := consensus.constructPrepareMessage()
	logger := utils.GetLogger().New("viewID", consensus.viewID, "block", consensus.blockNum)
	// TODO: this will not return immediatey, may block
	if err := consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		ctxerror.Warn(logger, err, "cannot send prepare message")
	} else {
		logger.Info("sent prepare message")
	}
}

// TODO: move to consensus_leader.go later
func (consensus *Consensus) onPrepare(msg *msg_pb.Message) {
	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		utils.GetLogger().Debug("onPrepare verifySenderKey failed", "error", err)
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.GetLogger().Debug("onPrepare Failed to verify sender's signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogger().Debug("[Consensus] onPrepare Unparseable validator message", "error", err)
		return
	}

	if recvMsg.ViewID != consensus.viewID || recvMsg.BlockNum != consensus.blockNum || consensus.phase != Prepare {
		utils.GetLogger().Debug("onPrepare message not match", "myPhase", consensus.phase, "myViewID", consensus.viewID,
			"msgViewID", recvMsg.ViewID, "myBlockNum", consensus.blockNum, "msgBlockNum", recvMsg.BlockNum)
		return
	}

	if !consensus.pbftLog.HasMatchingViewAnnounce(consensus.blockNum, consensus.viewID, recvMsg.BlockHash) {
		utils.GetLogger().Debug("onPrepare no matching announce message", "blockNum", consensus.blockNum, "viewID", consensus.viewID, "blockHash", recvMsg.BlockHash)
		return
	}

	validatorPubKey := recvMsg.SenderPubkey
	addrBytes := validatorPubKey.GetAddress()
	validatorAddress := common.BytesToAddress(addrBytes[:])

	prepareSig := recvMsg.Payload
	prepareSigs := consensus.prepareSigs
	prepareBitmap := consensus.prepareBitmap

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	if len(prepareSigs) >= consensus.Quorum() {
		// already have enough signatures
		return
	}

	// proceed only when the message is not received before
	_, ok := prepareSigs[validatorAddress]
	if ok {
		utils.GetLogger().Debug("Already received prepare message from the validator", "validatorAddress", validatorAddress)
		return
	}

	// Check BLS signature for the multi-sig
	var sign bls.Sign
	err = sign.Deserialize(prepareSig)
	if err != nil {
		utils.GetLogger().Error("Failed to deserialize bls signature", "validatorAddress", validatorAddress)
		return
	}
	if !sign.VerifyHash(validatorPubKey, consensus.blockHash[:]) {
		utils.GetLogger().Error("Received invalid BLS signature", "validatorAddress", validatorAddress)
		return
	}

	utils.GetLogger().Debug("Received new prepare signature", "numReceivedSoFar", len(prepareSigs), "validatorAddress", validatorAddress, "PublicKeys", len(consensus.PublicKeys))
	prepareSigs[validatorAddress] = &sign
	// Set the bitmap indicating that this validator signed.
	if err := prepareBitmap.SetKey(validatorPubKey, true); err != nil {
		ctxerror.Warn(utils.GetLogger(), err, "prepareBitmap.SetKey failed")
	}

	if len(prepareSigs) >= consensus.Quorum() {
		consensus.switchPhase(Commit, false)
		// Construct and broadcast prepared message
		msgToSend, aggSig := consensus.constructPreparedMessage()
		consensus.aggregatedPrepareSig = aggSig

		// add prepared message to log
		msgPayload, _ := proto.GetConsensusMessagePayload(msgToSend)
		msg := &msg_pb.Message{}
		_ = protobuf.Unmarshal(msgPayload, msg)
		pbftMsg, err := ParsePbftMessage(msg)
		if err != nil {
			utils.GetLogger().Warn("onPrepare unable to parse pbft message", "error", err)
			return
		}
		consensus.pbftLog.AddMessage(pbftMsg)

		logger := utils.GetLogger().New("viewID", consensus.viewID, "block", consensus.blockNum)
		if err := consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
			ctxerror.Warn(logger, err, "cannot send prepared message")
		} else {
			logger.Debug("sent prepared message")
		}

		// Leader add commit phase signature
		blockNumHash := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumHash, consensus.blockNum)
		commitPayload := append(blockNumHash, consensus.blockHash[:]...)
		consensus.commitSigs[consensus.SelfAddress] = consensus.priKey.SignHash(commitPayload)
	}
	return
}

func (consensus *Consensus) onPrepared(msg *msg_pb.Message) {
	utils.GetLogger().Debug("receive prepared message", "phase", consensus.phase, "viewID", consensus.viewID, "block", consensus.blockNum)
	if consensus.PubKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		utils.GetLogger().Debug("onPrepared verifySenderKey failed", "error", err)
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal && !consensus.ignoreViewIDCheck {
		utils.GetLogger().Warn("onPrepared senderKey not match leader PubKey")
		return
	}
	if err := verifyMessageSig(senderKey, msg); err != nil {
		utils.GetLogger().Debug("onPrepared Failed to verify sender's signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogger().Debug("onPrepared Unparseable validator message", "error", err)
		return
	}
	utils.GetLogger().Info("onPrepared received prepared message", "myBlock", consensus.blockNum, "myViewID", consensus.viewID, "recvBlock", recvMsg.BlockNum, "msgViewID", recvMsg.ViewID)

	if recvMsg.BlockNum < consensus.blockNum {
		utils.GetLogger().Debug("old block received, ignoring",
			"receivedNumber", recvMsg.BlockNum,
			"expectedNumber", consensus.blockNum)
		return
	}

	blockHash := recvMsg.BlockHash
	aggSig, mask, err := consensus.readSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		utils.GetLogger().Error("readSignatureBitmapPayload failed", "error", err)
		return
	}

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	// check has 2f+1 signatures
	if count := utils.CountOneBits(mask.Bitmap); count < consensus.Quorum() {
		utils.GetLogger().Debug("not have enough signature", "need", consensus.Quorum(), "have", count)
		return
	}

	if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
		myBlockHash := common.Hash{}
		myBlockHash.SetBytes(consensus.blockHash[:])
		utils.GetLogger().Warn("onPrepared failed to verify multi signature for prepare phase", "blockHash", blockHash, "myBlockHash", myBlockHash)
		return
	}

	utils.GetLogger().Debug("prepared message added", "phase", consensus.phase, "myViewID", consensus.viewID, "myBlock", consensus.blockNum, "msgViewID", recvMsg.ViewID, "msgBlock", recvMsg.BlockNum)
	consensus.pbftLog.AddMessage(recvMsg)

	if consensus.mode.Mode() == ViewChanging {
		utils.GetLogger().Debug("viewchanging mode just exist after viewchanging")
		return
	}

	consensus.tryCatchup()

	if consensus.checkViewID(recvMsg) != nil {
		utils.GetLogger().Debug("viewID check failed", "viewID", recvMsg.ViewID, "myViewID", consensus.viewID)
		return
	}
	if recvMsg.BlockNum > consensus.blockNum {
		utils.GetLogger().Debug("future block received, ignoring",
			"receivedNumber", recvMsg.BlockNum,
			"expectedNumber", consensus.blockNum)
		return
	}

	consensus.aggregatedPrepareSig = aggSig
	consensus.prepareBitmap = mask

	if consensus.phase != Prepare {
		utils.GetLogger().Debug("we are in a wrong phase",
			"actualPhase", consensus.phase,
			"expectedPhase", Prepare)
		return
	}

	// Construct and send the commit message
	blockNumHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumHash, consensus.blockNum)
	commitPayload := append(blockNumHash, consensus.blockHash[:]...)
	msgToSend := consensus.constructCommitMessage(commitPayload)
	logger := utils.GetLogger().New("viewID", consensus.viewID, "block", consensus.blockNum)
	if err := consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		ctxerror.Warn(logger, err, "cannot send commit message")
	} else {
		logger.Debug("sent commit message")
	}

	consensus.switchPhase(Commit, false)

	return
}

// TODO: move it to consensus_leader.go later
func (consensus *Consensus) onCommit(msg *msg_pb.Message) {
	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		utils.GetLogger().Debug("onCommit verifySenderKey failed", "error", err)
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.GetLogger().Debug("onCommit Failed to verify sender's signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogger().Debug("onCommit parse pbft message failed", "error", err)
		return
	}

	if recvMsg.ViewID != consensus.viewID || recvMsg.BlockNum != consensus.blockNum || consensus.phase != Commit {
		utils.GetLogger().Debug("not match", "myViewID", consensus.viewID, "viewID", recvMsg.ViewID, "myBlock", consensus.blockNum, "block", recvMsg.BlockNum, "myPhase", consensus.phase, "phase", Commit)
		return
	}

	if !consensus.pbftLog.HasMatchingAnnounce(consensus.blockNum, recvMsg.BlockHash) {
		utils.GetLogger().Debug("cannot find matching blockhash")
		return
	}

	if !consensus.pbftLog.HasMatchingViewPrepared(consensus.blockNum, consensus.viewID, recvMsg.BlockHash) {
		utils.GetLogger().Debug("cannot find matching prepared message")
		return
	}

	validatorPubKey := recvMsg.SenderPubkey
	addrBytes := validatorPubKey.GetAddress()
	validatorAddress := common.BytesToAddress(addrBytes[:])

	commitSig := recvMsg.Payload

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	if !consensus.IsValidatorInCommittee(validatorAddress) {
		utils.GetLogger().Error("Invalid validator", "validatorAddress", validatorAddress)
		return
	}

	commitSigs := consensus.commitSigs
	commitBitmap := consensus.commitBitmap

	// proceed only when the message is not received before
	_, ok := commitSigs[validatorAddress]
	if ok {
		utils.GetLogger().Debug("Already received commit message from the validator", "validatorAddress", validatorAddress)
		return
	}

	// already had enough signautres
	if len(commitSigs) >= consensus.Quorum() {
		return
	}

	// Verify the signature on commitPayload is correct
	var sign bls.Sign
	err = sign.Deserialize(commitSig)
	if err != nil {
		utils.GetLogger().Debug("Failed to deserialize bls signature", "validatorAddress", validatorAddress)
		return
	}
	blockNumHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumHash, recvMsg.BlockNum)
	commitPayload := append(blockNumHash, recvMsg.BlockHash[:]...)
	if !sign.VerifyHash(validatorPubKey, commitPayload) {
		utils.GetLogger().Error("cannot verify commit message", "viewID", recvMsg.ViewID, "block", recvMsg.BlockNum)
		return
	}

	utils.GetLogger().Debug("Received new commit message", "numReceivedSoFar", len(commitSigs), "viewID", recvMsg.ViewID, "block", recvMsg.BlockNum, "validatorAddress", validatorAddress)
	commitSigs[validatorAddress] = &sign
	// Set the bitmap indicating that this validator signed.
	if err := commitBitmap.SetKey(validatorPubKey, true); err != nil {
		ctxerror.Warn(utils.GetLogger(), err, "commitBitmap.SetKey failed")
	}

	if len(commitSigs) >= consensus.Quorum() {
		utils.GetLogger().Info("Enough commits received!", "num", len(commitSigs), "phase", consensus.phase)
		consensus.finalizeCommits()
	}
}

func (consensus *Consensus) finalizeCommits() {
	utils.GetLogger().Info("finalizing block", "num", len(consensus.commitSigs), "phase", consensus.phase)
	consensus.switchPhase(Announce, false)

	// Construct and broadcast committed message
	msgToSend, aggSig := consensus.constructCommittedMessage()
	consensus.aggregatedCommitSig = aggSig

	if err := consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		ctxerror.Warn(utils.GetLogger(), err, "cannot send committed message")
	} else {
		utils.GetLogger().Debug("sent committed message", "len", len(msgToSend))
	}

	var blockObj types.Block
	err := rlp.DecodeBytes(consensus.block, &blockObj)
	if err != nil {
		utils.GetLogger().Debug("failed to construct the new block after consensus")
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
		utils.GetLogger().Info("[SYNC] Failed to send consensus verified block for state sync", "blockHash", blockObj.Hash())
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
		utils.GetLogger().Debug("start consensus timeout; stop bootstrap timeout only once", "viewID", consensus.viewID, "block", consensus.blockNum)
	} else {
		utils.GetLogger().Debug("start consensus timeout", "viewID", consensus.viewID, "block", consensus.blockNum)
	}
	consensus.consensusTimeout[timeoutConsensus].Start()

	consensus.OnConsensusDone(&blockObj)
	utils.GetLogger().Debug("HOORAY!!!!!!! CONSENSUS REACHED!!!!!!!", "viewID", consensus.viewID, "numOfSignatures", len(consensus.commitSigs))

	// Send signal to Node so the new block can be added and new round of consensus can be triggered
	consensus.ReadySignal <- struct{}{}
}

func (consensus *Consensus) onCommitted(msg *msg_pb.Message) {
	utils.GetLogger().Debug("receive committed message", "phase", consensus.phase, "viewID", consensus.viewID, "block", consensus.blockNum)

	if consensus.PubKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		utils.GetLogger().Warn("onCommitted verifySenderKey failed", "error", err)
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) && consensus.mode.Mode() == Normal && !consensus.ignoreViewIDCheck {
		utils.GetLogger().Warn("onCommitted senderKey not match leader PubKey")
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.GetLogger().Warn("onCommitted Failed to verify sender's signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogger().Warn("onCommitted unable to parse msg", "error", err)
		return
	}
	if recvMsg.BlockNum < consensus.blockNum {
		return
	}

	aggSig, mask, err := consensus.readSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		utils.GetLogger().Error("readSignatureBitmapPayload failed", "error", err)
		return
	}

	// check has 2f+1 signatures
	if count := utils.CountOneBits(mask.Bitmap); count < consensus.Quorum() {
		utils.GetLogger().Debug("not have enough signature", "need", consensus.Quorum(), "have", count)
		return
	}

	blockNumHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumHash, recvMsg.BlockNum)
	commitPayload := append(blockNumHash, recvMsg.BlockHash[:]...)
	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		utils.GetLogger().Error("Failed to verify the multi signature for commit phase", "blockNum", recvMsg.BlockNum)
		return
	}
	consensus.aggregatedCommitSig = aggSig
	consensus.commitBitmap = mask
	utils.GetLogger().Debug("committed message added", "phase", consensus.phase, "myViewID", consensus.viewID, "myBlock", consensus.blockNum, "msgViewID", recvMsg.ViewID, "msgBlock", recvMsg.BlockNum)
	consensus.pbftLog.AddMessage(recvMsg)

	if recvMsg.BlockNum-consensus.blockNum > consensusBlockNumBuffer {
		utils.GetLogger().Debug("onCommitted out of sync", "myBlock", consensus.blockNum, "msgBlock", recvMsg.BlockNum)
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
	//		utils.GetLogger().Debug("viewID check failed", "viewID", recvMsg.ViewID, "myViewID", consensus.viewID)
	//		return
	//	}

	consensus.tryCatchup()
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	if consensus.consensusTimeout[timeoutBootstrap].IsActive() {
		consensus.consensusTimeout[timeoutBootstrap].Stop()
		utils.GetLogger().Debug("start consensus timeout; stop bootstrap timeout only once", "viewID", consensus.viewID, "block", consensus.blockNum)
	} else {
		utils.GetLogger().Debug("start consensus timeout", "viewID", consensus.viewID, "block", consensus.blockNum)
	}
	consensus.consensusTimeout[timeoutConsensus].Start()
	return
}

// try to catch up if fall behind
func (consensus *Consensus) tryCatchup() {
	utils.GetLogger().Info("tryCatchup: commit new blocks", "blockNum", consensus.blockNum)
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
			utils.GetLogger().Error("[PBFT] DANGER!!! we should only get one committed message for a given blockNum", "blockNum", consensus.blockNum, "numMsgs", len(msgs))
		}
		utils.GetLogger().Info("committed message found", "block", consensus.blockNum)

		block := consensus.pbftLog.GetBlockByHash(msgs[0].BlockHash)
		if block == nil {
			break
		}

		if block.ParentHash() != consensus.ChainReader.CurrentHeader().Hash() {
			utils.GetLogger().Debug("[PBFT] parent block hash not match", "blockNum", consensus.blockNum)
			break
		}
		utils.GetLogger().Info("block found to commit", "block", consensus.blockNum)

		preparedMsgs := consensus.pbftLog.GetMessagesByTypeSeqHash(msg_pb.MessageType_PREPARED, msgs[0].BlockNum, msgs[0].BlockHash)
		msg := consensus.pbftLog.FindMessageByMaxViewID(preparedMsgs)
		if msg == nil {
			break
		}
		utils.GetLogger().Info("prepared message found to commit", "block", consensus.blockNum)

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
		utils.GetLogger().Info("Adding block to chain", "viewID", consensus.viewID, "block", consensus.blockNum)
		consensus.OnConsensusDone(block)
		consensus.ResetState()

		select {
		case consensus.VerifiedNewBlock <- block:
		default:
			utils.GetLogger().Info("[SYNC] consensus verified block send to chan failed", "blockHash", block.Hash())
			continue
		}

		break
	}
	if currentBlockNum < consensus.blockNum {
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
		utils.GetLogger().Info("start consensus", "time", time.Now())
		defer close(stoppedChan)
		ticker := time.NewTicker(3 * time.Second)
		consensus.consensusTimeout[timeoutBootstrap].Start()
		utils.GetLogger().Debug("start bootstrap timeout only once", "viewID", consensus.viewID, "block", consensus.blockNum)
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
						utils.GetLogger().Debug("ops", "phase", k, "mode", consensus.mode.Mode(), "viewChangeID", consensus.viewID)
						consensus.startViewChange(consensus.viewID + 1)
						break
					} else {
						utils.GetLogger().Debug("ops", "phase", k, "mode", consensus.mode.Mode(), "viewChangeID", consensus.mode.ViewID())
						viewID := consensus.mode.ViewID()
						consensus.startViewChange(viewID + 1)
						break
					}
				}

			case <-consensus.syncReadyChan:
				consensus.mode.SetMode(Normal)
				consensus.SetBlockNum(consensus.ChainReader.CurrentHeader().Number.Uint64() + 1)
				consensus.ignoreViewIDCheck = true

			case newBlock := <-blockChannel:
				utils.GetLogger().Info("receive newBlock", "blockNum", newBlock.NumberU64())
				if consensus.ShardID == 0 {
					// TODO ek/rj - re-enable this after fixing DRand
					//if core.IsEpochBlock(newBlock) { // Only beacon chain do randomness generation
					//	// Receive pRnd from DRG protocol
					//	utils.GetLogger().Debug("[DRG] Waiting for pRnd")
					//	pRndAndBitmap := <-consensus.PRndChannel
					//	utils.GetLogger().Debug("[DRG] Got pRnd", "pRnd", pRndAndBitmap)
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
						utils.GetLogger().Info("Adding randomness into new block", "rnd", rnd)
						newBlock.AddVdf([258]byte{}) // TODO(HB): add real vdf
					} else {
						utils.GetLogger().Info("Failed to get randomness", "error", err)
					}
				}

				startTime = time.Now()
				utils.GetLogger().Debug("STARTING CONSENSUS", "numTxs", len(newBlock.Transactions()), "consensus", consensus, "startTime", startTime, "publicKeys", len(consensus.PublicKeys))
				consensus.tryAnnounce(newBlock)

			case msg := <-consensus.MsgChan:
				consensus.handleMessageUpdate(msg)

			case <-stopChan:
				return
			}
		}
	}()
}
