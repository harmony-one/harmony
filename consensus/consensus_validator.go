package consensus

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/attack"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// sendBFTBlockToStateSyncing will send the latest BFT consensus block to state syncing checkingjjkkkkkkkkkkkkkkkjnjk
func (consensus *Consensus) sendBFTBlockToStateSyncing(consensusID uint32) {
	// validator send consensus block to state syncing
	if val, ok := consensus.blocksReceived[consensusID]; ok {
		consensus.mutex.Lock()
		delete(consensus.blocksReceived, consensusID)
		consensus.mutex.Unlock()

		var blockObj types.Block
		err := rlp.DecodeBytes(val.block, &blockObj)
		if err != nil {
			utils.GetLogInstance().Debug("failed to construct the cached block")
			return
		}
		blockInfo := &BFTBlockInfo{Block: &blockObj, ConsensusID: consensusID}
		select {
		case consensus.ConsensusBlock <- blockInfo:
		default:
			utils.GetLogInstance().Warn("consensus block unable to sent to state sync", "height", blockObj.NumberU64(), "blockHash", blockObj.Hash().Hex())
		}
	}
	return
}

// IsValidatorMessage checks if a message is to be sent to a validator.
func (consensus *Consensus) IsValidatorMessage(message *msg_pb.Message) bool {
	return message.ReceiverType == msg_pb.ReceiverType_VALIDATOR && message.ServiceType == msg_pb.ServiceType_CONSENSUS
}

// ProcessMessageValidator dispatches validator's consensus message.
func (consensus *Consensus) ProcessMessageValidator(payload []byte) {
	message := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to unmarshal message payload.", "err", err, "consensus", consensus)
	}

	if !consensus.IsValidatorMessage(message) {
		return
	}

	switch message.Type {
	case msg_pb.MessageType_ANNOUNCE:
		consensus.processAnnounceMessage(message)
	case msg_pb.MessageType_PREPARED:
		consensus.processPreparedMessage(message)
	case msg_pb.MessageType_COMMITTED:
		consensus.processCommittedMessage(message)
	case msg_pb.MessageType_PREPARE:
	case msg_pb.MessageType_COMMIT:
		// ignore consensus message that is only meant to sent to leader
		// since we use pubsub, the relay node will also receive those message
		// but we should just ignore them

	default:
		utils.GetLogInstance().Error("Unexpected message type", "msgType", message.Type, "consensus", consensus)
	}
}

// Processes the announce message sent from the leader
func (consensus *Consensus) processAnnounceMessage(message *msg_pb.Message) {
	utils.GetLogInstance().Info("Received Announce Message", "ValidatorAddress", consensus.SelfAddress)

	consensusMsg := message.GetConsensus()

	consensusID := consensusMsg.ConsensusId
	blockHash := consensusMsg.BlockHash
	block := consensusMsg.Payload

	// Add block to received block cache
	consensus.mutex.Lock()
	consensus.blocksReceived[consensusID] = &BlockConsensusStatus{block, consensus.state}
	consensus.mutex.Unlock()

	copy(consensus.blockHash[:], blockHash[:])
	consensus.block = block

	if err := consensus.checkConsensusMessage(message, consensus.leader.ConsensusPubKey); err != nil {
		utils.GetLogInstance().Debug("Failed to check the leader message")
		if err == consensus_engine.ErrConsensusIDNotMatch {
			utils.GetLogInstance().Debug("sending bft block to state syncing")
			consensus.sendBFTBlockToStateSyncing(consensusID)
		}
		return
	}

	// check block header is valid
	var blockObj types.Block
	err := rlp.DecodeBytes(block, &blockObj)
	if err != nil {
		utils.GetLogInstance().Warn("Unparseable block header data", "error", err)
		return
	}

	// Add attack model of IncorrectResponse
	if attack.GetInstance().IncorrectResponse() {
		utils.GetLogInstance().Warn("IncorrectResponse attacked")
		return
	}

	// check block data transactions
	if err := consensus.VerifyHeader(consensus.ChainReader, blockObj.Header(), false); err != nil {
		utils.GetLogInstance().Warn("Block content is not verified successfully", "error", err)
		return
	}

	// Construct and send prepare message
	msgToSend := consensus.constructPrepareMessage()
	utils.GetLogInstance().Warn("[Consensus]", "sent prepare message", len(msgToSend))
	consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, host.ConstructP2pMessage(byte(17), msgToSend))

	consensus.state = PrepareDone
}

// Processes the prepared message sent from the leader
func (consensus *Consensus) processPreparedMessage(message *msg_pb.Message) {
	utils.GetLogInstance().Info("Received Prepared Message", "ValidatorAddress", consensus.SelfAddress)

	consensusMsg := message.GetConsensus()

	consensusID := consensusMsg.ConsensusId
	blockHash := consensusMsg.BlockHash
	pubKey, err := bls_cosi.BytesToBlsPublicKey(consensusMsg.SenderPubkey)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to deserialize BLS public key", "error", err)
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

	// Update readyByConsensus for attack.
	attack.GetInstance().UpdateConsensusReady(consensusID)

	if err := consensus.checkConsensusMessage(message, consensus.leader.ConsensusPubKey); err != nil {
		utils.GetLogInstance().Debug("processPreparedMessage error", "error", err)
		return
	}

	// Add attack model of IncorrectResponse.
	if attack.GetInstance().IncorrectResponse() {
		utils.GetLogInstance().Warn("IncorrectResponse attacked")
		return
	}

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
	consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, host.ConstructP2pMessage(byte(17), msgToSend))

	consensus.state = CommitDone
}

// Processes the committed message sent from the leader
func (consensus *Consensus) processCommittedMessage(message *msg_pb.Message) {
	utils.GetLogInstance().Warn("Received Committed Message", "ValidatorAddress", consensus.SelfAddress)

	consensusMsg := message.GetConsensus()
	consensusID := consensusMsg.ConsensusId
	pubKey, err := bls_cosi.BytesToBlsPublicKey(consensusMsg.SenderPubkey)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to deserialize BLS public key", "error", err)
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

	// Update readyByConsensus for attack.
	attack.GetInstance().UpdateConsensusReady(consensusID)

	if err := consensus.checkConsensusMessage(message, consensus.leader.ConsensusPubKey); err != nil {
		utils.GetLogInstance().Debug("processCommittedMessage error", "error", err)
		return
	}

	// Add attack model of IncorrectResponse.
	if attack.GetInstance().IncorrectResponse() {
		utils.GetLogInstance().Warn("IncorrectResponse attacked")
		return
	}

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

	consensus.state = CommittedDone
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
}
