package consensus

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
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
		utils.GetLogInstance().Error("Failed to unmarshal message payload.", "err", err, "consensus", consensus)
		return
	}

	// stop receiving normal message when is in viewchanging
	if consensus.mode.Mode() == ViewChanging && msg.Type != msg_pb.MessageType_VIEWCHANGE && msg.Type != msg_pb.MessageType_NEWVIEW {
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
		utils.GetLogInstance().Debug("tryAnnounce blockNum not match", "blockNum", block.NumberU64(), "myBlockNum", consensus.blockNum)
		return
	}
	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		utils.GetLogInstance().Debug("tryAnnounce key not match", "myKey", consensus.PubKey, "leaderKey", consensus.LeaderPubKey)
		return
	}
	blockHash := block.Hash()
	copy(consensus.blockHash[:], blockHash[:])

	// prepare message and broadcast to validators
	encodedBlock, err := rlp.EncodeToBytes(block)
	if err != nil {
		utils.GetLogInstance().Debug("tryAnnounce Failed encoding block")
		return
	}
	consensus.block = encodedBlock
	msgToSend := consensus.constructAnnounceMessage()
	consensus.switchPhase(Prepare)

	// save announce message to pbftLog
	msgPayload, _ := proto.GetConsensusMessagePayload(msgToSend)
	msg := &msg_pb.Message{}
	_ = protobuf.Unmarshal(msgPayload, msg)
	pbftMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogInstance().Warn("tryAnnounce unable to parse pbft message", "error", err)
		return
	}

	consensus.pbftLog.AddMessage(pbftMsg)
	consensus.pbftLog.AddBlock(block)

	// Leader sign the block hash itself
	consensus.prepareSigs[consensus.SelfAddress] = consensus.priKey.SignHash(consensus.blockHash[:])

	// Construct broadcast p2p message
	utils.GetLogInstance().Warn("tryAnnounce", "sent announce message", len(msgToSend), "groupID", p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID)))
	consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))
}

func (consensus *Consensus) onAnnounce(msg *msg_pb.Message) {
	if consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		utils.GetLogInstance().Debug("onAnnounce verifySenderKey failed", "error", err)
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) {
		utils.GetLogInstance().Warn("onAnnounce senderKey not match leader PubKey", "senderKey", senderKey.GetHexString(), "leaderKey", consensus.LeaderPubKey.GetHexString())
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.GetLogInstance().Debug("onAnnounce Failed to verify leader signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogInstance().Debug("onAnnounce Unparseable leader message", "error", err)
		return
	}
	block := recvMsg.Payload

	// check block header is valid
	var blockObj types.Block
	err = rlp.DecodeBytes(block, &blockObj)
	if err != nil {
		utils.GetLogInstance().Warn("onAnnounce Unparseable block header data", "error", err)
		return
	}

	// check block data transactions
	if err := consensus.VerifyHeader(consensus.ChainReader, blockObj.Header(), false); err != nil {
		utils.GetLogInstance().Warn("onAnnounce block content is not verified successfully", "error", err)
		return
	}
	if consensus.BlockVerifier == nil {
		// do nothing
	} else if err := consensus.BlockVerifier(&blockObj); err != nil {
		// TODO ek – maybe we could do this in commit phase
		err := ctxerror.New("block verification failed",
			"blockHash", blockObj.Hash(),
		).WithCause(err)
		ctxerror.Log15(utils.GetLogInstance().Warn, err)
		return
	}

	logMsgs := consensus.pbftLog.GetMessagesByTypeSeqView(msg_pb.MessageType_ANNOUNCE, recvMsg.BlockNum, recvMsg.ViewID)
	if len(logMsgs) > 0 {
		if logMsgs[0].BlockHash != blockObj.Header().Hash() {
			utils.GetLogInstance().Debug("onAnnounce leader is malicious", "leaderKey", utils.GetBlsAddress(consensus.LeaderPubKey))
			consensus.startViewChange(consensus.viewID + 1)
		}
		return
	}

	blockPayload := make([]byte, len(block))
	copy(blockPayload[:], block[:])
	consensus.block = blockPayload
	consensus.blockHash = recvMsg.BlockHash
	consensus.pbftLog.AddMessage(recvMsg)
	consensus.pbftLog.AddBlock(&blockObj)
	consensus.tryPrepare(blockObj.Header().Hash())

	consensus.consensusTimeout["bootstrap"].Stop()
	consensus.consensusTimeout["announce"].Stop()
	if !consensus.consensusTimeout["prepare"].IsActive() {
		consensus.consensusTimeout["prepare"].Start()
	}
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

	if consensus.phase != Announce || consensus.blockNum != block.NumberU64() || !consensus.pbftLog.HasMatchingAnnounce(consensus.blockNum, consensus.viewID, hash) {
		return
	}

	consensus.switchPhase(Prepare)

	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) { //TODO(chao): check whether this is necessary when calling tryPrepare
		// Construct and send prepare message
		msgToSend := consensus.constructPrepareMessage()
		utils.GetLogInstance().Info("tryPrepare", "sent prepare message", len(msgToSend))
		consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))
	}
}

// TODO: move to consensus_leader.go later
func (consensus *Consensus) onPrepare(msg *msg_pb.Message) {
	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		utils.GetLogInstance().Debug("onPrepare verifySenderKey failed", "error", err)
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.GetLogInstance().Debug("onPrepare Failed to verify sender's signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogInstance().Debug("[Consensus] onPrepare Unparseable validator message", "error", err)
		return
	}

	if recvMsg.ViewID != consensus.viewID || recvMsg.BlockNum != consensus.blockNum || consensus.phase != Prepare {
		utils.GetLogInstance().Debug("onPrepare message not match", "myPhase", consensus.phase, "myViewID", consensus.viewID,
			"msgViewID", recvMsg.ViewID, "myBlockNum", consensus.blockNum, "msgBlockNum", recvMsg.BlockNum)
		return
	}

	if !consensus.pbftLog.HasMatchingAnnounce(consensus.blockNum, consensus.viewID, recvMsg.BlockHash) {
		utils.GetLogInstance().Debug("onPrepare no matching announce message", "blockNum", consensus.blockNum, "viewID", consensus.viewID, "blockHash", recvMsg.BlockHash)
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
	if len(prepareSigs) >= ((len(consensus.PublicKeys)*2)/3 + 1) {
		// already have enough signatures
		return
	}

	// proceed only when the message is not received before
	_, ok := prepareSigs[validatorAddress]
	if ok {
		utils.GetLogInstance().Debug("Already received prepare message from the validator", "validatorAddress", validatorAddress)
		return
	}

	// Check BLS signature for the multi-sig
	var sign bls.Sign
	err = sign.Deserialize(prepareSig)
	if err != nil {
		utils.GetLogInstance().Error("Failed to deserialize bls signature", "validatorAddress", validatorAddress)
		return
	}
	if !sign.VerifyHash(validatorPubKey, consensus.blockHash[:]) {
		utils.GetLogInstance().Error("Received invalid BLS signature", "validatorAddress", validatorAddress)
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

		utils.GetLogInstance().Warn("onPrepare", "sent prepared message", len(msgToSend))
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

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		utils.GetLogInstance().Debug("onPrepared verifySenderKey failed", "error", err)
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) {
		utils.GetLogInstance().Warn("onPrepared senderKey not match leader PubKey")
		return
	}
	if err := verifyMessageSig(senderKey, msg); err != nil {
		utils.GetLogInstance().Debug("onPrepared Failed to verify sender's signature", "error", err)
		return
	}

	utils.GetLogInstance().Info("onPrepared received prepared message", "ValidatorAddress", consensus.SelfAddress)

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogInstance().Debug("onPrepared Unparseable validator message", "error", err)
		return
	}
	if recvMsg.BlockNum < consensus.blockNum {
		return
	}

	blockHash := recvMsg.BlockHash
	pubKey := recvMsg.SenderPubkey
	if !pubKey.IsEqual(consensus.LeaderPubKey) {
		utils.GetLogInstance().Debug("onPrepared leader pubkey not match", "suppose", consensus.LeaderPubKey, "got", pubKey)
		return
	}
	addrBytes := pubKey.GetAddress()
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

	// Verify the multi-sig for prepare phase
	deserializedMultiSig := bls.Sign{}
	err = deserializedMultiSig.Deserialize(multiSig)
	if err != nil {
		utils.GetLogInstance().Warn("onPrepared failed to deserialize the multi signature for prepare phase", "Error", err, "leader Address", leaderAddress)
		return
	}
	mask, err := bls_cosi.NewMask(consensus.PublicKeys, nil)
	mask.SetMask(bitmap)
	// TODO: add 2f+1 signature checking
	if !deserializedMultiSig.VerifyHash(mask.AggregatePublic, blockHash[:]) || err != nil {
		myBlockHash := common.Hash{}
		myBlockHash.SetBytes(consensus.blockHash[:])
		utils.GetLogInstance().Warn("onPrepared failed to verify multi signature for prepare phase", "error", err, "blockHash", blockHash, "myBlockHash", myBlockHash)
		return
	}

	consensus.pbftLog.AddMessage(recvMsg)
	if recvMsg.BlockNum > consensus.blockNum || consensus.phase != Prepare || recvMsg.ViewID != consensus.viewID {
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

	consensus.consensusTimeout["prepare"].Stop()
	if !consensus.consensusTimeout["commit"].IsActive() {
		consensus.consensusTimeout["commit"].Start()
	}
	return
}

// TODO: move it to consensus_leader.go later
func (consensus *Consensus) onCommit(msg *msg_pb.Message) {
	if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		utils.GetLogInstance().Debug("onCommit verifySenderKey failed", "error", err)
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.GetLogInstance().Debug("onCommit Failed to verify sender's signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogInstance().Debug("onCommit parse pbft message failed", "error", err)
		return
	}

	if recvMsg.ViewID != consensus.viewID || recvMsg.BlockNum != consensus.blockNum || consensus.phase != Commit {
		return
	}

	if !consensus.pbftLog.HasMatchingAnnounce(consensus.blockNum, consensus.viewID, recvMsg.BlockHash) {
		return
	}

	validatorPubKey := recvMsg.SenderPubkey
	addrBytes := validatorPubKey.GetAddress()
	validatorAddress := common.BytesToAddress(addrBytes[:])

	commitSig := recvMsg.Payload

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	if !consensus.IsValidatorInCommittee(validatorAddress) {
		utils.GetLogInstance().Error("Invalid validator", "validatorAddress", validatorAddress)
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
		consensus.switchPhase(Announce)

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
		explorer.GetStorageInstance(consensus.leader.IP, consensus.leader.Port, true).Dump(&blockObj, consensus.viewID)

		// Reset state to Finished, and clear other data.
		consensus.ResetState()
		consensus.viewID++
		consensus.blockNum++

		consensus.OnConsensusDone(&blockObj)
		utils.GetLogInstance().Debug("HOORAY!!!!!!! CONSENSUS REACHED!!!!!!!", "viewID", consensus.viewID, "numOfSignatures", len(commitSigs))

		// Send signal to Node so the new block can be added and new round of consensus can be triggered
		consensus.ReadySignal <- struct{}{}
	}
	return
}

func (consensus *Consensus) onCommitted(msg *msg_pb.Message) {
	utils.GetLogInstance().Warn("Received Committed Message", "ValidatorAddress", consensus.SelfAddress)

	if consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
		return
	}

	senderKey, err := consensus.verifySenderKey(msg)
	if err != nil {
		utils.GetLogInstance().Debug("onCommitted verifySenderKey failed", "error", err)
		return
	}
	if !senderKey.IsEqual(consensus.LeaderPubKey) {
		utils.GetLogInstance().Warn("onCommitted senderKey not match leader PubKey")
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.GetLogInstance().Debug("onCommitted Failed to verify sender's signature", "error", err)
		return
	}

	recvMsg, err := ParsePbftMessage(msg)
	if err != nil {
		utils.GetLogInstance().Warn("onCommitted unable to parse msg", "error", err)
		return
	}
	if recvMsg.BlockNum < consensus.blockNum {
		return
	}

	validatorPubKey := recvMsg.SenderPubkey
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
	// TODO: add 2f+1 signature checking
	if !deserializedMultiSig.VerifyHash(mask.AggregatePublic, prepareMultiSigAndBitmap) || err != nil {
		utils.GetLogInstance().Warn("Failed to verify the multi signature for commit phase", "Error", err, "leader Address", leaderAddress)
		return
	}

	consensus.pbftLog.AddMessage(recvMsg)

	if recvMsg.BlockNum > consensus.blockNum || consensus.phase != Commit || recvMsg.ViewID != consensus.viewID {
		return
	}
	consensus.aggregatedCommitSig = &deserializedMultiSig
	consensus.commitBitmap = mask

	consensus.tryCatchup()
	consensus.consensusTimeout["commit"].Stop()
	if !consensus.consensusTimeout["announce"].IsActive() {
		consensus.consensusTimeout["announce"].Start()
	}
	return
}

// try to catch up if fall behind
func (consensus *Consensus) tryCatchup() {
	utils.GetLogInstance().Info("tryCatchup: commit new blocks", "blockNum", consensus.blockNum)
	if consensus.phase != Commit {
		return
	}
	consensus.switchPhase(Announce)
	for {
		msgs := consensus.pbftLog.GetMessagesByTypeSeq(msg_pb.MessageType_COMMITTED, consensus.blockNum)
		if len(msgs) == 0 {
			break
		}
		if len(msgs) > 1 {
			utils.GetLogInstance().Info("[PBFT] we should only get one committed message for a given blockNum", "blockNum", consensus.blockNum, "numMsgs", len(msgs))
		}

		block := consensus.pbftLog.GetBlockByHash(msgs[0].BlockHash)
		if block == nil {
			break
		}

		if block.ParentHash() != consensus.ChainReader.CurrentHeader().Hash() {
			utils.GetLogInstance().Debug("[PBFT] parent block hash not match", "blockNum", consensus.blockNum)
			break
		}

		preparedMsgs := consensus.pbftLog.GetMessagesByTypeSeqHash(msg_pb.MessageType_PREPARED, msgs[0].BlockNum, msgs[0].BlockHash)
		if len(preparedMsgs) > 1 {
			utils.GetLogInstance().Info("[PBFT] we should only get one prepared message for a given blockNum", "blockNum", consensus.blockNum, "numMsgs", len(preparedMsgs))
		}
		if len(preparedMsgs) == 0 {
			break
		}

		cnt := 0
		for _, msg := range preparedMsgs {
			if msg.BlockHash == msgs[0].BlockHash {
				cnt++
				consensus.blockHash = [32]byte{}
				consensus.blockNum = consensus.blockNum + 1
				consensus.viewID = consensus.viewID + 1

				//#### Read payload data from committed msg
				aggSig := make([]byte, 48)
				bitmap := make([]byte, len(msgs[0].Payload)-48)
				offset := 0
				copy(aggSig[:], msgs[0].Payload[offset:offset+48])
				offset += 48
				copy(bitmap[:], msgs[0].Payload[offset:])
				//#### END Read payload data from committed msg

				//#### Read payload data from prepared msg
				prepareSig := make([]byte, 48)
				prepareBitmap := make([]byte, len(msg.Payload)-48)
				offset = 0
				copy(prepareSig[:], msg.Payload[offset:offset+48])
				offset += 48
				copy(prepareBitmap[:], msg.Payload[offset:])
				//#### END Read payload data from committed msg

				// Put the signatures into the block
				block.SetPrepareSig(prepareSig, prepareBitmap)

				block.SetCommitSig(aggSig, bitmap)
				utils.GetLogInstance().Info("Adding block to chain", "numTx", len(block.Transactions()))
				consensus.OnConsensusDone(block)
				consensus.ResetState()

				select {
				case consensus.VerifiedNewBlock <- block:
				default:
					utils.GetLogInstance().Info("[SYNC] consensus verified block send to chan failed", "blockHash", block.Hash())
					continue
				}

				break
			}
		}
		if cnt == 0 { // didn't find match block for current blockNum, return
			break
		}
	}
}

// Start waits for the next new block and run consensus
func (consensus *Consensus) Start(blockChannel chan *types.Block, stopChan chan struct{}, stoppedChan chan struct{}, startChannel chan struct{}) {
	if nodeconfig.GetDefaultConfig().IsLeader() {
		<-startChannel
	}
	go func() {
		utils.GetLogInstance().Info("start consensus", "time", time.Now())
		defer close(stoppedChan)
		ticker := time.NewTicker(3 * time.Second)
		consensus.consensusTimeout["bootstrap"].Start()
		for {
			select {
			case <-ticker.C:
				if !consensus.PubKey.IsEqual(consensus.LeaderPubKey) {
					for k, v := range consensus.consensusTimeout {
						if !v.CheckExpire() {
							continue
						}
						if k != "viewchange" {
							utils.GetLogInstance().Debug("ops", "phase", k, "mode", consensus.mode.Mode())
							consensus.startViewChange(consensus.viewID + 1)
							break
						} else {
							utils.GetLogInstance().Debug("ops", "phase", k, "mode", consensus.mode.Mode())
							viewID := consensus.mode.ViewID()
							consensus.startViewChange(viewID + 1)
							break
						}
					}

				}

			case newBlock := <-blockChannel:
				utils.GetLogInstance().Info("receive newBlock", "blockNum", newBlock.NumberU64())
				if consensus.ShardID == 0 {
					// TODO ek/rj - re-enable this after fixing DRand
					//if core.IsEpochBlock(newBlock) { // Only beacon chain do randomness generation
					//	// Receive pRnd from DRG protocol
					//	utils.GetLogInstance().Debug("[DRG] Waiting for pRnd")
					//	pRndAndBitmap := <-consensus.PRndChannel
					//	utils.GetLogInstance().Debug("[DRG] Got pRnd", "pRnd", pRndAndBitmap)
					//	pRnd := [32]byte{}
					//	copy(pRnd[:], pRndAndBitmap[:32])
					//	bitmap := pRndAndBitmap[32:]
					//	vrfBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
					//	vrfBitmap.SetMask(bitmap)
					//
					//	// TODO: check validity of pRnd
					//	newBlock.AddRandPreimage(pRnd)
					//}

					rnd, blockHash, err := consensus.GetNextRnd()
					if err == nil {
						// Verify the randomness
						_ = blockHash
						utils.GetLogInstance().Info("Adding randomness into new block", "rnd", rnd)
						newBlock.AddRandSeed(rnd)
					} else {
						utils.GetLogInstance().Info("Failed to get randomness", "error", err)
					}
				}

				startTime = time.Now()
				utils.GetLogInstance().Debug("STARTING CONSENSUS", "numTxs", len(newBlock.Transactions()), "consensus", consensus, "startTime", startTime, "publicKeys", len(consensus.PublicKeys))
				consensus.tryAnnounce(newBlock)

			case msg := <-consensus.MsgChan:
				consensus.handleMessageUpdate(msg)
			case <-stopChan:
				return
			}
		}
	}()
}
