package consensus

import (
	"bytes"

	"github.com/dedis/kyber/sign/schnorr"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/internal/attack"
	"github.com/harmony-one/harmony/internal/utils"
)

// ProcessMessageValidator dispatches validator's consensus message.
func (consensus *Consensus) ProcessMessageValidator(payload []byte) {
	message := consensus_proto.Message{}
	err := message.XXX_Unmarshal(payload)

	if err != nil {
		consensus.Log.Error("Failed to unmarshal message payload.", "err", err, "consensus", consensus)
	}
	switch message.Type {
	case consensus_proto.MessageType_ANNOUNCE:
		consensus.processAnnounceMessage(message)
	case consensus_proto.MessageType_CHALLENGE:
		consensus.processChallengeMessage(message, ResponseDone)
	case consensus_proto.MessageType_FINAL_CHALLENGE:
		consensus.processChallengeMessage(message, FinalResponseDone)
	case consensus_proto.MessageType_COLLECTIVE_SIG:
		consensus.processCollectiveSigMessage(message)
	default:
		consensus.Log.Error("Unexpected message type", "msgType", message.Type, "consensus", consensus)
	}
}

// Processes the announce message sent from the leader
func (consensus *Consensus) processAnnounceMessage(message consensus_proto.Message) {
	consensus.Log.Info("Received Announce Message", "nodeID", consensus.nodeID)

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
		consensus.Log.Warn("Received message from wrong leader", "myLeaderID", myLeaderID, "receivedLeaderId", leaderID, "consensus", consensus)
		return
	}

	// Verify signature
	message.Signature = nil
	messageBytes, err := message.XXX_Marshal([]byte{}, true)
	if err != nil {
		consensus.Log.Warn("Failed to marshal the announce message", "error", err)
	}
	if schnorr.Verify(crypto.Ed25519Curve, consensus.leader.PubKey, messageBytes, signature) != nil {
		consensus.Log.Warn("Received message with invalid signature", "leaderKey", consensus.leader.PubKey, "consensus", consensus)
		return
	}

	// check block header is valid
	var blockObj types.Block
	err = rlp.DecodeBytes(block, &blockObj)
	if err != nil {
		consensus.Log.Warn("Unparseable block header data", "error", err)
		return
	}

	// send consensus block to state syncing
	// TODO (chao): avoid malicious leader to send wrong block
	select {
	case consensus.ConsensusBlock <- &blockObj:
		consensus.Log.Info("consensus block sent to state sync", "blockHash", blockObj.Hash())
	default:
		consensus.Log.Warn("consensus block unable to sent to state sync", "blockHash", blockObj.Hash())
	}

	consensus.block = block

	// Add block to received block cache
	consensus.mutex.Lock()
	consensus.blocksReceived[consensusID] = &BlockConsensusStatus{block, consensus.state}
	consensus.mutex.Unlock()

	// Add attack model of IncorrectResponse.
	if attack.GetInstance().IncorrectResponse() {
		consensus.Log.Warn("IncorrectResponse attacked")
		return
	}

	// check block hash
	hash := blockObj.Hash()
	if !bytes.Equal(blockHash[:], hash[:]) {
		consensus.Log.Warn("Block hash doesn't match", "consensus", consensus)
		return
	}

	// check block data (transactions
	if !consensus.BlockVerifier(&blockObj) {
		consensus.Log.Warn("Block content is not verified successfully", "consensus", consensus)
		return
	}

	// Commit and store the commit
	secret, msgToSend := consensus.constructCommitMessage(consensus_proto.MessageType_COMMIT)
	consensus.secret[consensusID] = secret

	consensus.SendMessage(consensus.leader, msgToSend)
	// consensus.Log.Warn("Sending Commit to leader", "state", targetState)

	// Set state to CommitDone
	consensus.state = CommitDone
}

// Processes the challenge message sent from the leader
func (consensus *Consensus) processChallengeMessage(message consensus_proto.Message, targetState State) {
	consensus.Log.Info("Received Challenge Message", "nodeID", consensus.nodeID)

	consensusID := message.ConsensusId
	blockHash := message.BlockHash
	leaderID := message.SenderId
	messagePayload := message.Payload
	signature := message.Signature

	//#### Read payload data
	// TODO: use BLS-based multi-sig
	offset := 0
	// 33 byte of aggregated commit
	aggreCommit := messagePayload[offset : offset+33]
	offset += 33

	// 33 byte of aggregated key
	aggreKey := messagePayload[offset : offset+33]
	offset += 33

	// 32 byte of challenge
	challenge := messagePayload[offset : offset+32]
	offset += 32

	// Update readyByConsensus for attack.
	attack.GetInstance().UpdateConsensusReady(consensusID)

	// Verify block data and the aggregated signatures
	// check leader Id
	myLeaderID := utils.GetUniqueIDFromPeer(consensus.leader)
	if uint32(leaderID) != myLeaderID {
		consensus.Log.Warn("Received message from wrong leader", "myLeaderID", myLeaderID, "receivedLeaderId", leaderID, "consensus", consensus)
		return
	}

	// Verify signature
	message.Signature = nil
	messageBytes, err := message.XXX_Marshal([]byte{}, true)
	if err != nil {
		consensus.Log.Warn("Failed to marshal the announce message", "error", err)
	}
	if schnorr.Verify(crypto.Ed25519Curve, consensus.leader.PubKey, messageBytes, signature) != nil {
		consensus.Log.Warn("Received message with invalid signature", "leaderKey", consensus.leader.PubKey, "consensus", consensus)
		return
	}

	// Add attack model of IncorrectResponse.
	if attack.GetInstance().IncorrectResponse() {
		consensus.Log.Warn("IncorrectResponse attacked")
		return
	}

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	// check block hash
	if !bytes.Equal(blockHash[:], consensus.blockHash[:]) {
		consensus.Log.Warn("Block hash doesn't match", "consensus", consensus)
		return
	}

	aggCommitment := crypto.Ed25519Curve.Point()
	aggCommitment.UnmarshalBinary(aggreCommit[:32])
	aggKey := crypto.Ed25519Curve.Point()
	aggKey.UnmarshalBinary(aggreKey[:32])

	reconstructedChallenge, err := crypto.Challenge(crypto.Ed25519Curve, aggCommitment, aggKey, blockHash)

	if err != nil {
		log.Error("Failed to reconstruct the challenge from commits and keys")
		return
	}

	// For now, simply return the private key of this node.
	receivedChallenge := crypto.Ed25519Curve.Scalar()
	err = receivedChallenge.UnmarshalBinary(challenge)
	if err != nil {
		log.Error("Failed to deserialize challenge", "err", err)
		return
	}

	if !reconstructedChallenge.Equal(receivedChallenge) {
		log.Error("The challenge doesn't match the commitments and keys")
		return
	}

	response, err := crypto.Response(crypto.Ed25519Curve, consensus.priKey, consensus.secret[consensusID], receivedChallenge)
	if err != nil {
		log.Warn("validator failed to generate response", "err", err, "priKey", consensus.priKey, "nodeID", consensus.nodeID, "secret", consensus.secret[consensusID])
		return
	}

	msgTypeToSend := consensus_proto.MessageType_RESPONSE
	if targetState == FinalResponseDone {
		msgTypeToSend = consensus_proto.MessageType_FINAL_RESPONSE
	}
	msgToSend := consensus.constructResponseMessage(msgTypeToSend, response)

	consensus.SendMessage(consensus.leader, msgToSend)
	// consensus.Log.Warn("Sending Response to leader", "state", targetState)
	// Set state to target state (ResponseDone, FinalResponseDone)
	consensus.state = targetState

	if consensus.state == FinalResponseDone {
		// TODO: the block catch up logic is a temporary workaround for full failure node catchup. Implement the full node catchup logic
		// The logic is to roll up to the latest blocks one by one to try catching up with the leader.
		for {
			val, ok := consensus.blocksReceived[consensus.consensusID]
			if ok {
				delete(consensus.blocksReceived, consensus.consensusID)

				consensus.blockHash = [32]byte{}
				delete(consensus.secret, consensusID)
				consensus.consensusID = consensusID + 1 // roll up one by one, until the next block is not received yet.

				var blockObj types.Block
				err := rlp.DecodeBytes(val.block, &blockObj)
				if err != nil {
					consensus.Log.Warn("Unparseable block header data", "error", err)
					return
				}
				if err != nil {
					consensus.Log.Debug("failed to construct the new block after consensus")
				}
				// check block data (transactions
				if !consensus.BlockVerifier(&blockObj) {
					consensus.Log.Debug("[WARNING] Block content is not verified successfully", "consensusID", consensus.consensusID)
					return
				}
				consensus.Log.Info("Finished Response. Adding block to chain", "numTx", len(blockObj.Transactions()))
				consensus.OnConsensusDone(&blockObj)

				select {
				case consensus.VerifiedNewBlock <- &blockObj:
				default:
					consensus.Log.Info("[sync] consensus verified block send to chan failed", "blockHash", blockObj.Hash())
					continue
				}

			} else {
				break
			}

		}
	}
}

// Processes the collective signature message sent from the leader
func (consensus *Consensus) processCollectiveSigMessage(message consensus_proto.Message) {
	consensusID := message.ConsensusId
	blockHash := message.BlockHash
	leaderID := message.SenderId
	messagePayload := message.Payload
	signature := message.Signature

	//#### Read payload data
	collectiveSig := messagePayload[0:64]
	bitmap := messagePayload[64:]
	//#### END: Read payload data

	// Verify block data
	// check leader Id
	myLeaderID := utils.GetUniqueIDFromPeer(consensus.leader)
	if uint32(leaderID) != myLeaderID {
		consensus.Log.Warn("Received message from wrong leader", "myLeaderID", myLeaderID, "receivedLeaderId", leaderID, "consensus", consensus)
		return
	}

	// Verify signature
	message.Signature = nil
	messageBytes, err := message.XXX_Marshal([]byte{}, true)
	if err != nil {
		consensus.Log.Warn("Failed to marshal the announce message", "error", err)
	}
	if schnorr.Verify(crypto.Ed25519Curve, consensus.leader.PubKey, messageBytes, signature) != nil {
		consensus.Log.Warn("Received message with invalid signature", "leaderKey", consensus.leader.PubKey, "consensus", consensus)
		return
	}

	// Verify collective signature
	err = crypto.Verify(crypto.Ed25519Curve, consensus.PublicKeys, blockHash, append(collectiveSig, bitmap...), crypto.NewThresholdPolicy((2*len(consensus.PublicKeys)/3)+1))
	if err != nil {
		consensus.Log.Warn("Failed to verify the collective sig message", "consensusID", consensusID, "err", err, "bitmap", bitmap, "NodeID", consensus.nodeID, "#PK", len(consensus.PublicKeys))
		return
	}

	// Add attack model of IncorrectResponse.
	if attack.GetInstance().IncorrectResponse() {
		consensus.Log.Warn("IncorrectResponse attacked")
		return
	}

	// check consensus Id
	if consensusID != consensus.consensusID {
		consensus.Log.Warn("Received message with wrong consensus Id", "myConsensusId", consensus.consensusID, "theirConsensusId", consensusID, "consensus", consensus)
		return
	}

	// check block hash
	if !bytes.Equal(blockHash[:], consensus.blockHash[:]) {
		consensus.Log.Warn("Block hash doesn't match", "consensus", consensus)
		return
	}

	secret, msgToSend := consensus.constructCommitMessage(consensus_proto.MessageType_FINAL_COMMIT)
	// Store the commitment secret
	consensus.secret[consensusID] = secret

	consensus.SendMessage(consensus.leader, msgToSend)

	// Set state to CommitDone
	consensus.state = FinalCommitDone
}
