package consensus

import (
	"bytes"

	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/attack"
	"github.com/harmony-one/harmony/internal/utils"
)

// ProcessMessageValidator dispatches validator's consensus message.
func (consensus *Consensus) ProcessMessageValidator(payload []byte) {
	message := consensus_proto.Message{}
	err := protobuf.Unmarshal(payload, &message)

	if err != nil {
		utils.GetLogInstance().Error("Failed to unmarshal message payload.", "err", err, "consensus", consensus)
	}
	switch message.Type {
	case consensus_proto.MessageType_ANNOUNCE:
		consensus.processAnnounceMessage(message)
	case consensus_proto.MessageType_PREPARED:
		consensus.processPreparedMessage(message)
	case consensus_proto.MessageType_COMMITTED:
		consensus.processCommittedMessage(message)
	default:
		utils.GetLogInstance().Error("Unexpected message type", "msgType", message.Type, "consensus", consensus)
	}
}

// Processes the announce message sent from the leader
func (consensus *Consensus) processAnnounceMessage(message consensus_proto.Message) {
	utils.GetLogInstance().Info("Received Announce Message", "nodeID", consensus.nodeID)

	consensusID := message.ConsensusId
	blockHash := message.BlockHash
	leaderID := message.SenderId
	block := message.Payload
	signature := message.Signature

	copy(consensus.blockHash[:], blockHash[:])

	// Verify block data
	// check leader Id
	myLeaderID := utils.GetUniqueIDFromPeer(consensus.leader)
	if leaderID != myLeaderID {
		utils.GetLogInstance().Warn("Received message from wrong leader", "myLeaderID", myLeaderID, "receivedLeaderId", leaderID, "consensus", consensus)
		return
	}

	// Verify signature
	message.Signature = nil
	messageBytes, err := protobuf.Marshal(&message)
	if err != nil {
		utils.GetLogInstance().Warn("Failed to marshal the announce message", "error", err)
	}
	_ = signature
	_ = messageBytes
	// TODO: verify message signature
	//if schnorr.Verify(crypto.Ed25519Curve, consensus.leader.PubKey, messageBytes, signature) != nil {
	//	consensus.Log.Warn("Received message with invalid signature", "leaderKey", consensus.leader.PubKey, "consensus", consensus)
	//	return
	//}

	// check block header is valid
	var blockObj types.Block
	err = rlp.DecodeBytes(block, &blockObj)
	if err != nil {
		utils.GetLogInstance().Warn("Unparseable block header data", "error", err)
		return
	}

	consensus.block = block

	// Add block to received block cache
	consensus.mutex.Lock()
	consensus.blocksReceived[consensusID] = &BlockConsensusStatus{block, consensus.state}
	consensus.mutex.Unlock()

	// Add attack model of IncorrectResponse
	if attack.GetInstance().IncorrectResponse() {
		utils.GetLogInstance().Warn("IncorrectResponse attacked")
		return
	}

	// check block hash
	hash := blockObj.Hash()
	if !bytes.Equal(blockHash[:], hash[:]) {
		utils.GetLogInstance().Warn("Block hash doesn't match", "consensus", consensus)
		return
	}

	// check block data (transactions
	if !consensus.BlockVerifier(&blockObj) {
		utils.GetLogInstance().Warn("Block content is not verified successfully", "consensus", consensus)
		return
	}

	// Construct and send prepare message
	msgToSend := consensus.constructPrepareMessage()

	consensus.SendMessage(consensus.leader, msgToSend)
	// utils.GetLogInstance().Warn("Sending Commit to leader", "state", targetState)

	// Set state to PrepareDone
	consensus.state = PrepareDone
}

// Processes the prepared message sent from the leader
func (consensus *Consensus) processPreparedMessage(message consensus_proto.Message) {
	utils.GetLogInstance().Info("Received Prepared Message", "nodeID", consensus.nodeID)

	consensusID := message.ConsensusId
	blockHash := message.BlockHash
	leaderID := message.SenderId
	messagePayload := message.Payload
	signature := message.Signature

	//#### Read payload data
	offset := 0
	// 48 byte of multi-sig
	multiSig := messagePayload[offset : offset+48]
	offset += 48

	// bitmap
	bitmap := messagePayload[offset:]

	// Update readyByConsensus for attack.
	attack.GetInstance().UpdateConsensusReady(consensusID)

	// Verify block data and the aggregated signatures
	// check leader Id
	myLeaderID := utils.GetUniqueIDFromPeer(consensus.leader)
	if uint32(leaderID) != myLeaderID {
		utils.GetLogInstance().Warn("Received message from wrong leader", "myLeaderID", myLeaderID, "receivedLeaderId", leaderID, "consensus", consensus)
		return
	}

	// Verify signature
	message.Signature = nil
	messageBytes, err := protobuf.Marshal(&message)
	if err != nil {
		utils.GetLogInstance().Warn("Failed to marshal the announce message", "error", err)
	}
	_ = signature
	_ = messageBytes
	// TODO: verify message signature
	//if schnorr.Verify(crypto.Ed25519Curve, consensus.leader.PubKey, messageBytes, signature) != nil {
	//	consensus.Log.Warn("Received message with invalid signature", "leaderKey", consensus.leader.PubKey, "consensus", consensus)
	//	return
	//}

	// Add attack model of IncorrectResponse.
	if attack.GetInstance().IncorrectResponse() {
		utils.GetLogInstance().Warn("IncorrectResponse attacked")
		return
	}

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	// check block hash
	if !bytes.Equal(blockHash[:], consensus.blockHash[:]) {
		utils.GetLogInstance().Warn("Block hash doesn't match", "consensus", consensus)
		return
	}

	_ = multiSig
	_ = bitmap
	// TODO: verify multi signature

	// Construct and send the commit message
	msgToSend := consensus.constructCommitMessage()

	consensus.SendMessage(consensus.leader, msgToSend)

	consensus.state = CommitDone
}

// Processes the committed message sent from the leader
func (consensus *Consensus) processCommittedMessage(message consensus_proto.Message) {
	utils.GetLogInstance().Warn("Received Prepared Message", "nodeID", consensus.nodeID)

	consensusID := message.ConsensusId
	blockHash := message.BlockHash
	leaderID := message.SenderId
	messagePayload := message.Payload
	signature := message.Signature

	//#### Read payload data
	offset := 0
	// 48 byte of multi-sig
	multiSig := messagePayload[offset : offset+48]
	offset += 48

	// bitmap
	bitmap := messagePayload[offset:]

	// Update readyByConsensus for attack.
	attack.GetInstance().UpdateConsensusReady(consensusID)

	// Verify block data and the aggregated signatures
	// check leader Id
	myLeaderID := utils.GetUniqueIDFromPeer(consensus.leader)
	if uint32(leaderID) != myLeaderID {
		utils.GetLogInstance().Warn("Received message from wrong leader", "myLeaderID", myLeaderID, "receivedLeaderId", leaderID, "consensus", consensus)
		return
	}

	// Verify signature
	message.Signature = nil
	messageBytes, err := protobuf.Marshal(&message)
	if err != nil {
		utils.GetLogInstance().Warn("Failed to marshal the announce message", "error", err)
	}
	_ = signature
	_ = messageBytes
	// TODO: verify message signature
	//if schnorr.Verify(crypto.Ed25519Curve, consensus.leader.PubKey, messageBytes, signature) != nil {
	//	consensus.Log.Warn("Received message with invalid signature", "leaderKey", consensus.leader.PubKey, "consensus", consensus)
	//	return
	//}

	// Add attack model of IncorrectResponse.
	if attack.GetInstance().IncorrectResponse() {
		utils.GetLogInstance().Warn("IncorrectResponse attacked")
		return
	}

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	// check consensus Id
	if consensusID != consensus.consensusID {
		// hack for new node state syncing
		utils.GetLogInstance().Warn("Received message with wrong consensus Id", "myConsensusId", consensus.consensusID, "theirConsensusId", consensusID, "consensus", consensus)
		consensus.consensusID = consensusID
		return
	}

	// check block hash
	if !bytes.Equal(blockHash[:], consensus.blockHash[:]) {
		utils.GetLogInstance().Warn("Block hash doesn't match", "consensus", consensus)
		return
	}

	_ = multiSig
	_ = bitmap
	// TODO: verify multi signature

	// Construct and send the prepare message
	msgToSend := consensus.constructPrepareMessage()

	consensus.SendMessage(consensus.leader, msgToSend)

	consensus.state = CommittedDone

	// TODO: the block catch up logic is a temporary workaround for full failure node catchup. Implement the full node catchup logic
	// The logic is to roll up to the latest blocks one by one to try catching up with the leader.
	for {
		val, ok := consensus.blocksReceived[consensus.consensusID]
		if ok {
			delete(consensus.blocksReceived, consensus.consensusID)

			consensus.blockHash = [32]byte{}
			consensus.consensusID = consensusID + 1 // roll up one by one, until the next block is not received yet.

			var blockObj types.Block
			err := rlp.DecodeBytes(val.block, &blockObj)
			if err != nil {
				utils.GetLogInstance().Warn("Unparseable block header data", "error", err)
				return
			}
			if err != nil {
				utils.GetLogInstance().Debug("failed to construct the new block after consensus")
			}
			// check block data (transactions
			if !consensus.BlockVerifier(&blockObj) {
				utils.GetLogInstance().Debug("[WARNING] Block content is not verified successfully", "consensusID", consensus.consensusID)
				return
			}
			utils.GetLogInstance().Info("Finished Response. Adding block to chain", "numTx", len(blockObj.Transactions()))
			consensus.OnConsensusDone(&blockObj)
			consensus.state = Finished

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
}
