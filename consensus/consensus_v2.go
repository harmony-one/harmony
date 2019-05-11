package consensus

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// Start is the entry point and main loop for consensus
func (consensus *Consensus) Start(stopChan chan struct{}, stoppedChan chan struct{}) {
	defer close(stoppedChan)
	tick := time.NewTicker(blockDuration)
	consensus.idleTimeout.Start()
	for {
		select {
		default:
			msg := consensus.recvWithTimeout(receiveTimeout)
			consensus.handleMessageUpdate(msg)
			if consensus.idleTimeout.CheckExpire() {
				consensus.startViewChange(consensus.consensusID + 1)
			}
			if consensus.commitTimeout.CheckExpire() {
				consensus.startViewChange(consensus.consensusID + 1)
			}
			if consensus.viewChangeTimeout.CheckExpire() {
				if consensus.mode.Mode() == Normal {
					continue
				}
				consensusID := consensus.mode.ConsensusID()
				consensus.startViewChange(consensusID + 1)
			}
		case <-tick.C:
		case <-stopChan:
			return
		}
	}

}

// recvWithTimeout receives message before timeout
func (consensus *Consensus) recvWithTimeout(timeoutDuration time.Duration) []byte {
	var msg []byte
	select {
	case msg = <-consensus.MsgChan:
		utils.GetLogInstance().Debug("[Consensus] recvWithTimeout received", "msg", msg)
	case <-time.After(timeoutDuration):
		utils.GetLogInstance().Debug("[Consensus] recvWithTimeout timeout", "duration", timeoutDuration)
	}
	return msg
}

// handleMessageUpdate will update the consensus state according to received message
func (consensus *Consensus) handleMessageUpdate(payload []byte) {
	msg := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, msg)
	if err != nil {
		utils.GetLogInstance().Error("Failed to unmarshal message payload.", "err", err, "consensus", consensus)
	}

	consensusMsg := msg.GetConsensus()
	senderKey, err := bls_cosi.BytesToBlsPublicKey(consensusMsg.SenderPubkey)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to deserialize BLS public key", "error", err)
		return
	}
	addrBytes := senderKey.GetAddress()
	senderAddr := common.BytesToAddress(addrBytes[:])

	if !consensus.IsValidatorInCommittee(senderAddr) {
		utils.GetLogInstance().Error("Invalid validator", "senderAddress", senderAddr)
		return
	}

	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.GetLogInstance().Debug("Failed to verify sender's signature", "error", err)
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

func (consensus *Consensus) onAnnounce(msg *msg_pb.Message) {
	if consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}

	utils.GetLogInstance().Info("Received Announce Message", "ValidatorAddress", consensus.SelfAddress)
	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogInstance().Debug("[Consensus] onAnnounce Unparseable leader message", "error", err)
	}
	block := recvMsg.Payload

	if err := verifyMessageSig(consensus.LeaderPubKey, msg); err != nil {
		utils.GetLogInstance().Debug("[Consensus] Failed to verify leader signature", "error", err)
		return
	}

	// check block header is valid
	var blockObj types.Block
	err = rlp.DecodeBytes(block, &blockObj)
	if err != nil {
		utils.GetLogInstance().Warn("[Consensus] Unparseable block header data", "error", err)
		return
	}

	// check block data transactions
	if err := consensus.VerifyHeader(consensus.ChainReader, blockObj.Header(), false); err != nil {
		utils.GetLogInstance().Warn("Block content is not verified successfully", "error", err)
		return
	}

	logMsgs := consensus.pbftLog.GetMessagesByTypeSeqView(msg_pb.MessageType_PREPARED, recvMsg.SeqNum, recvMsg.ConsensusID)
	if len(logMsgs) > 0 {
		utils.GetLogInstance().Debug("[Consensus] leader is malicious", "leaderKey", utils.GetBlsAddress(consensus.LeaderPubKey))
		consensus.startViewChange(consensus.consensusID + 1)
		return
	}

	consensus.pbftLog.AddMessage(recvMsg)
	consensus.pbftLog.AddBlock(&blockObj)
	consensus.tryPreparing(blockObj.Hash())
	return
}

// tryPreparing will try to send prepare message
func (consensus *Consensus) tryPreparing(blockHash []byte) {
	var hash common.Hash
	copy(hash[:], blockHash)
	block := consensus.pbftLog.GetBlockByHash(hash)
	if block == nil {
		return
	}

	if consensus.phase != Announce || consensus.seqNum != block.NumberU64() || !consensus.pbftLog.HasMatchingAnnounce(consensus.seqNum, consensus.consensusID, hash) {
		return
	}

	consensus.switchPhase(Prepare)
	consensus.idleTimeout.Stop()
	consensus.commitTimeout.Start()

	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) { //TODO(chao): check whether this is necessary when calling tryPreparing
		// Construct and send prepare message
		msgToSend := consensus.constructPrepareMessage()
		utils.GetLogInstance().Warn("[Consensus]", "sent prepare message", len(msgToSend))
		consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))
	}
}

func (consensus *Consensus) onPrepare(msg *msg_pb.Message) {
	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}
	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogInstance().Debug("[Consensus] onPrepare Unparseable validator message", "error", err)
		return
	}

	if recvMsg.ConsensusID != consensus.consensusID || recvMsg.SeqNum != consensus.seqNum || recvMsg.phase != Prepare {
		return
	}

	if !consensus.pbftLog.HasMatchingAnnounce(consensus.seqNum, consensus.consensusID, recvMsg.BlockHash) {
		return
	}

	validatorPubKey, _ := bls_cosi.BytesToBlsPublicKey(recvMsg.SenderPubkey)
	addrBytes := validatorPubKey.GetAddress()
	validatorAddress := common.BytesToAddress(addrBytes[:])

	prepareSig := recvMsg.Payload
	prepareSigs := consensus.prepareSigs
	prepareBitmap := consensus.prepareBitmap

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	// proceed only when the message is not received before
	_, ok := prepareSigs[validatorAddress]
	if ok {
		utils.GetLogInstance().Debug("Already received prepare message from the validator", "validatorAddress", validatorAddress)
		return
	}

	if len(prepareSigs) >= ((len(consensus.PublicKeys)*2)/3 + 1) {
		// already have enough signatures
		return
	}

	// Check BLS signature for the multi-sig
	var sign bls.Sign
	err = sign.Deserialize(prepareSig)
	if err != nil {
		utils.GetLogInstance().Error("Failed to deserialize bls signature", "validatorAddress", validatorAddress)
		return
	}

	utils.GetLogInstance().Debug("Received new prepare signature", "numReceivedSoFar", len(prepareSigs), "validatorAddress", validatorAddress, "PublicKeys", len(consensus.PublicKeys))
	prepareSigs[validatorAddress] = &sign
	prepareBitmap.SetKey(validatorPubKey, true) // Set the bitmap indicating that this validator signed.

	if len(prepareSigs) >= ((len(consensus.PublicKeys)*2)/3 + 1) {
		consensus.switchPhase(Commit)

		// Construct and broadcast prepared message
		msgToSend, aggSig := consensus.constructPreparedMessage()
		consensus.aggregatedPrepareSig = aggSig

		utils.GetLogInstance().Warn("[Consensus]", "sent prepared message", len(msgToSend))
		consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))

		// Leader sign the multi-sig and bitmap (for commit phase)
		multiSigAndBitmap := append(aggSig.Serialize(), prepareBitmap.Bitmap...)
		consensus.commitSigs[consensus.SelfAddress] = consensus.priKey.SignHash(multiSigAndBitmap)
	}
	return
}

func (consensus *Consensus) onPrepared(msg *msg_pb.Message) {
	if consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}
	utils.GetLogInstance().Info("Received Prepared Message", "ValidatorAddress", consensus.SelfAddress)

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogInstance().Debug("[Consensus] onPrepare Unparseable validator message", "error", err)
		return
	}

	blockHash := consensusMsg.BlockHash
	pubKey, _ := bls_cosi.BytesToBlsPublicKey(recvMsg.SenderPubkey)
	if !pubKey.IsEqual(consensus.LeaderPubKey) {
		utils.GetLogInstance().Debug("onPrepared:leader pubkey not match", "suppose", consensus.LeaderPubKey, "got", pubKey)
		return
	}
	addrBytes := pubKey.GetAddress()
	leaderAddress := common.BytesToAddress(addrBytes[:]).Hex()

	messagePayload := consensusMsg.Payload

	//#### Read payload data
	offset := 0
	// 48 byte of multi-sig
	multiSig := messagePayload[offset : offset+48]
	offset += 48

	// bitmap
	bitmap := messagePayload[offset:]
	//#### END Read payload data

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	// Verify the multi-sig for prepare phase
	deserializedMultiSig := bls.Sign{}
	err = deserializedMultiSig.Deserialize(multiSig)
	if err != nil {
		utils.GetLogInstance().Warn("Failed to deserialize the multi signature for prepare phase", "Error", err, "leader Address", leaderAddress)
		return
	}
	mask, err := bls_cosi.NewMask(consensus.PublicKeys, nil)
	mask.SetMask(bitmap)
	if !deserializedMultiSig.VerifyHash(mask.AggregatePublic, blockHash) || err != nil {
		utils.GetLogInstance().Warn("Failed to verify the multi signature for prepare phase", "Error", err, "leader Address", leaderAddress, "PubKeys", len(consensus.PublicKeys))
		return
	}
	consensus.aggregatedPrepareSig = &deserializedMultiSig
	consensus.prepareBitmap = mask

	// Construct and send the commit message
	multiSigAndBitmap := append(multiSig, bitmap...)
	msgToSend := consensus.constructCommitMessage(multiSigAndBitmap)
	utils.GetLogInstance().Warn("[Consensus]", "sent commit message", len(msgToSend))
	consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))

	consensus.switchPhase(Commit)
	return
}
func (consensus *Consensus) onCommit(msg msg_pb.Message) {
	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}
	recvMsg, err := ParsePbftMessage(msg)

	if recvMsg.ConsensusID != consensus.consensusID || recvMsg.SeqNum != consensus.seqNum || recvMsg.phase != Commit {
		return
	}

	if !consensus.pbftLog.HasMatchingAnnounce(consensus.seqNum, consensus.consensusID, recvMsg.BlockHash) {
		return
	}

	validatorPubKey, _ := bls_cosi.BytesToBlsPublicKey(recvMsg.SenderPubkey)
	addrBytes := validatorPubKey.GetAddress()
	validatorAddress := common.BytesToAddress(addrBytes[:])

	commitSig := consensusMsg.Payload

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	if !consensus.IsValidatorInCommittee(validatorAddress) {
		utils.GetLogInstance().Error("Invalid validator", "validatorAddress", validatorAddress)
		return
	}

	if err := consensus.checkConsensusMessage(message, validatorPubKey); err != nil {
		utils.GetLogInstance().Debug("Failed to check the validator message", "validatorAddress", validatorAddress)
		return
	}

	commitSigs := consensus.commitSigs
	commitBitmap := consensus.commitBitmap

	// proceed only when the message is not received before
	_, ok := commitSigs[validatorAddress]
	if ok {
		utils.GetLogInstance().Debug("Already received commit message from the validator", "validatorAddress", validatorAddress)
		return
	}

	if len((commitSigs)) >= ((len(consensus.PublicKeys)*2)/3 + 1) {
		return
	}

	// Verify the signature on prepare multi-sig and bitmap is correct
	var sign bls.Sign
	err = sign.Deserialize(commitSig)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to deserialize bls signature", "validatorAddress", validatorAddress)
		return
	}
	aggSig := bls_cosi.AggregateSig(consensus.GetPrepareSigsArray())
	if !sign.VerifyHash(validatorPubKey, append(aggSig.Serialize(), consensus.prepareBitmap.Bitmap...)) {
		utils.GetLogInstance().Error("Received invalid BLS signature", "validatorAddress", validatorAddress)
		return
	}

	utils.GetLogInstance().Debug("Received new commit message", "numReceivedSoFar", len(commitSigs), "validatorAddress", validatorAddress)
	commitSigs[validatorAddress] = &sign
	// Set the bitmap indicating that this validator signed.
	commitBitmap.SetKey(validatorPubKey, true)

	if len(commitSigs) >= ((len(consensus.PublicKeys)*2)/3 + 1) {
		utils.GetLogInstance().Info("Enough commits received!", "num", len(commitSigs), "state", consensus.state)

		// Construct and broadcast committed message
		msgToSend, aggSig := consensus.constructCommittedMessage()
		consensus.aggregatedCommitSig = aggSig

		utils.GetLogInstance().Warn("[Consensus]", "sent committed message", len(msgToSend))
		consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))

		var blockObj types.Block
		err := rlp.DecodeBytes(consensus.block, &blockObj)
		if err != nil {
			utils.GetLogInstance().Debug("failed to construct the new block after consensus")
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
			utils.GetLogInstance().Info("[SYNC] Failed to send consensus verified block for state sync", "blockHash", blockObj.Hash())
		}

		consensus.reportMetrics(blockObj)

		// Dump new block into level db.
		explorer.GetStorageInstance(consensus.leader.IP, consensus.leader.Port, true).Dump(&blockObj, consensus.consensusID)

		// Reset state to Finished, and clear other data.
		consensus.ResetState()
		consensus.consensusID++

		consensus.OnConsensusDone(&blockObj)
		utils.GetLogInstance().Debug("HOORAY!!!!!!! CONSENSUS REACHED!!!!!!!", "consensusID", consensus.consensusID, "numOfSignatures", len(commitSigs))

		// TODO: remove this temporary delay
		time.Sleep(500 * time.Millisecond)
		// Send signal to Node so the new block can be added and new round of consensus can be triggered
		consensus.ReadySignal <- struct{}{}
	}
	return
}

func (consensus *Consensus) onCommitted(msg msg_pb.Message) {
	utils.GetLogInstance().Warn("Received Committed Message", "ValidatorAddress", consensus.SelfAddress)

	if consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}
	recvMsg, err := ParsePbftMessage(msg)

	if recvMsg.ConsensusID != consensus.consensusID || recvMsg.SeqNum != consensus.seqNum || recvMsg.phase != Commit {
		return
	}

	if !consensus.pbftLog.HasMatchingAnnounce(consensus.seqNum, consensus.consensusID, recvMsg.BlockHash) {
		return
	}

	validatorPubKey, _ := bls_cosi.BytesToBlsPublicKey(recvMsg.SenderPubkey)
	addrBytes := validatorPubKey.GetAddress()
	leaderAddress := common.BytesToAddress(addrBytes[:]).Hex()

	messagePayload := recvMsg.Payload

	//#### Read payload data
	offset := 0
	// 48 byte of multi-sig
	multiSig := messagePayload[offset : offset+48]
	offset += 48

	// bitmap
	bitmap := messagePayload[offset:]
	//#### END Read payload data

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	// Verify the multi-sig for commit phase
	deserializedMultiSig := bls.Sign{}
	err = deserializedMultiSig.Deserialize(multiSig)
	if err != nil {
		utils.GetLogInstance().Warn("Failed to deserialize the multi signature for commit phase", "Error", err, "leader Address", leaderAddress)
		return
	}
	mask, err := bls_cosi.NewMask(consensus.PublicKeys, nil)
	mask.SetMask(bitmap)
	prepareMultiSigAndBitmap := append(consensus.aggregatedPrepareSig.Serialize(), consensus.prepareBitmap.Bitmap...)
	if !deserializedMultiSig.VerifyHash(mask.AggregatePublic, prepareMultiSigAndBitmap) || err != nil {
		utils.GetLogInstance().Warn("Failed to verify the multi signature for commit phase", "Error", err, "leader Address", leaderAddress)
		return
	}
	consensus.aggregatedCommitSig = &deserializedMultiSig
	consensus.commitBitmap = mask

	// TODO: the block catch up logic is a temporary workaround for full failure node catchup. Implement the full node catchup logic
	// The logic is to roll up to the latest blocks one by one to try catching up with the leader.
	// but because of checkConsensusMessage, the catchup logic will never be used here
	for {
		val, ok := consensus.blocksReceived[consensus.consensusID]
		if ok {
			delete(consensus.blocksReceived, consensus.consensusID)

			consensus.blockHash = [32]byte{}
			consensus.consensusID = consensusID + 1 // roll up one by one, until the next block is not received yet.

			var blockObj types.Block
			err := rlp.DecodeBytes(val.block, &blockObj)
			if err != nil {
				utils.GetLogInstance().Debug("failed to construct the new block after consensus")
			}
			// check block data (transactions
			if err := consensus.VerifyHeader(consensus.ChainReader, blockObj.Header(), false); err != nil {
				utils.GetLogInstance().Debug("[WARNING] Block content is not verified successfully", "consensusID", consensus.consensusID)
				return
			}

			// Put the signatures into the block
			blockObj.SetPrepareSig(
				consensus.aggregatedPrepareSig.Serialize(),
				consensus.prepareBitmap.Bitmap)
			blockObj.SetCommitSig(
				consensus.aggregatedCommitSig.Serialize(),
				consensus.commitBitmap.Bitmap)
			utils.GetLogInstance().Info("Adding block to chain", "numTx", len(blockObj.Transactions()))
			consensus.OnConsensusDone(&blockObj)
			consensus.ResetState()

			select {
			case consensus.VerifiedNewBlock <- &blockObj:
			default:
				utils.GetLogInstance().Info("[SYNC] consensus verified block send to chan failed", "blockHash", blockObj.Hash())
				continue
			}
		} else {
			break
		}

	}
	return
}

func (consensus *Consensus) onViewChange(msg msb_pb.Message) {
	return
}
func (consensus *Consensus) onNewView(msg msg_pb.Message) {
	return
}
