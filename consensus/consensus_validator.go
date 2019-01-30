package consensus

import (
	"github.com/harmony-one/bls/ffi/go/bls"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"

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
	block := message.Payload

	copy(consensus.blockHash[:], blockHash[:])

	if !consensus.checkConsensusMessage(message, consensus.leader.PubKey) {
		utils.GetLogInstance().Debug("Failed to check the leader message")
		return
	}

	// check block header is valid
	var blockObj types.Block
	err := rlp.DecodeBytes(block, &blockObj)
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

	//#### Read payload data
	offset := 0
	// 48 byte of multi-sig
	multiSig := messagePayload[offset : offset+48]
	offset += 48

	// bitmap
	bitmap := messagePayload[offset:]

	// Update readyByConsensus for attack.
	attack.GetInstance().UpdateConsensusReady(consensusID)

	if !consensus.checkConsensusMessage(message, consensus.leader.PubKey) {
		utils.GetLogInstance().Debug("Failed to check the leader message")
		return
	}

	// Add attack model of IncorrectResponse.
	if attack.GetInstance().IncorrectResponse() {
		utils.GetLogInstance().Warn("IncorrectResponse attacked")
		return
	}

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	deserializedMultiSig := bls.Sign{}
	err := deserializedMultiSig.Deserialize(multiSig)
	if err != nil {
		utils.GetLogInstance().Warn("Failed to deserialize the multi signature for prepare phase", "Error", err, "leader ID", leaderID)
		return
	}
	mask, err := bls_cosi.NewMask(consensus.PublicKeys, nil)
	mask.SetMask(bitmap)
	if !deserializedMultiSig.VerifyHash(mask.AggregatePublic, blockHash) || err != nil {
		utils.GetLogInstance().Warn("Failed to verify the multi signature for prepare phase", "Error", err, "leader ID", leaderID)
		return
	}
	consensus.aggregatedPrepareSig = &deserializedMultiSig
	consensus.prepareBitmap = mask

	multiSigAndBitmap := append(multiSig, bitmap...)

	// Construct and send the commit message
	msgToSend := consensus.constructCommitMessage(multiSigAndBitmap)

	consensus.SendMessage(consensus.leader, msgToSend)

	consensus.state = CommitDone
}

// Processes the committed message sent from the leader
func (consensus *Consensus) processCommittedMessage(message consensus_proto.Message) {
	utils.GetLogInstance().Warn("Received Prepared Message", "nodeID", consensus.nodeID)

	consensusID := message.ConsensusId
	leaderID := message.SenderId
	messagePayload := message.Payload

	//#### Read payload data
	offset := 0
	// 48 byte of multi-sig
	multiSig := messagePayload[offset : offset+48]
	offset += 48

	// bitmap
	bitmap := messagePayload[offset:]

	// Update readyByConsensus for attack.
	attack.GetInstance().UpdateConsensusReady(consensusID)

	if !consensus.checkConsensusMessage(message, consensus.leader.PubKey) {
		utils.GetLogInstance().Debug("Failed to check the leader message")
		return
	}

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

	deserializedMultiSig := bls.Sign{}
	err := deserializedMultiSig.Deserialize(multiSig)
	if err != nil {
		utils.GetLogInstance().Warn("Failed to deserialize the multi signature for commit phase", "Error", err, "leader ID", leaderID)
		return
	}
	mask, err := bls_cosi.NewMask(consensus.PublicKeys, nil)
	mask.SetMask(bitmap)

	prepareMultiSigAndBitmap := append(consensus.aggregatedPrepareSig.Serialize(), consensus.prepareBitmap.Bitmap...)
	if !deserializedMultiSig.VerifyHash(mask.AggregatePublic, prepareMultiSigAndBitmap) || err != nil {
		utils.GetLogInstance().Warn("Failed to verify the multi signature for commit phase", "Error", err, "leader ID", leaderID)
		return
	}

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
}
