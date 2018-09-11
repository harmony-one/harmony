package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"

	"github.com/dedis/kyber/sign/schnorr"
	"github.com/simple-rules/harmony-benchmark/attack"
	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/crypto"
	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/p2p"
	proto_consensus "github.com/simple-rules/harmony-benchmark/proto/consensus"
	"github.com/simple-rules/harmony-benchmark/utils"
)

// Validator's consensus message dispatcher
func (consensus *Consensus) ProcessMessageValidator(message []byte) {
	msgType, err := proto_consensus.GetConsensusMessageType(message)
	if err != nil {
		consensus.Log.Error("Failed to get consensus message type", "err", err, "consensus", consensus)
	}

	payload, err := proto_consensus.GetConsensusMessagePayload(message)
	if err != nil {
		consensus.Log.Error("Failed to get consensus message payload", "err", err, "consensus", consensus)
	}

	switch msgType {
	case proto_consensus.ANNOUNCE:
		consensus.processAnnounceMessage(payload)
	case proto_consensus.CHALLENGE:
		consensus.processChallengeMessage(payload, RESPONSE_DONE)
	case proto_consensus.FINAL_CHALLENGE:
		consensus.processChallengeMessage(payload, FINAL_RESPONSE_DONE)
	case proto_consensus.COLLECTIVE_SIG:
		consensus.processCollectiveSigMessage(payload)
	default:
		consensus.Log.Error("Unexpected message type", "msgType", msgType, "consensus", consensus)
	}
}

// Processes the announce message sent from the leader
func (consensus *Consensus) processAnnounceMessage(payload []byte) {
	consensus.Log.Info("Received Announce Message")
	//#### Read payload data
	offset := 0
	// 4 byte consensus id
	consensusId := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4

	// 32 byte block hash
	blockHash := payload[offset : offset+32]
	offset += 32

	// 2 byte leader id
	leaderId := binary.BigEndian.Uint16(payload[offset : offset+2])
	offset += 2

	// n byte of message to cosign
	n := len(payload) - offset - 64 // the number means 64 signature
	blockHeader := payload[offset : offset+n]
	offset += n

	// 64 byte of signature on previous data
	signature := payload[offset : offset+64]
	offset += 64
	//#### END: Read payload data

	consensus.Log.Info("Received Announce Message", "LeaderId", leaderId)

	copy(consensus.blockHash[:], blockHash[:])

	// Verify block data
	// check leader Id
	myLeaderId := utils.GetUniqueIdFromPeer(consensus.leader)
	if leaderId != myLeaderId {
		consensus.Log.Warn("Received message from wrong leader", "myLeaderId", myLeaderId, "receivedLeaderId", leaderId, "consensus", consensus)
		return
	}

	// Verify signature
	if schnorr.Verify(crypto.Ed25519Curve, consensus.leader.PubKey, payload[:offset-64], signature) != nil {
		consensus.Log.Warn("Received message with invalid signature", "leaderKey", consensus.leader.PubKey, "consensus", consensus)
		return
	}

	// check block header is valid
	txDecoder := gob.NewDecoder(bytes.NewReader(blockHeader))
	var blockHeaderObj blockchain.Block // TODO: separate header from block. Right now, this blockHeader data is actually the whole block
	err := txDecoder.Decode(&blockHeaderObj)
	if err != nil {
		consensus.Log.Warn("Unparseable block header data", "consensus", consensus)
		return
	}
	consensus.blockHeader = blockHeader // TODO: think about remove this field and use blocksReceived instead
	consensus.mutex.Lock()
	consensus.blocksReceived[consensusId] = &BlockConsensusStatus{blockHeader, consensus.state}
	consensus.mutex.Unlock()

	// Add attack model of IncorrectResponse.
	if attack.GetInstance().IncorrectResponse() {
		consensus.Log.Warn("IncorrectResponse attacked")
		return
	}

	// check consensus Id
	if consensusId != consensus.consensusId {
		consensus.Log.Warn("Received message with wrong consensus Id", "myConsensusId", consensus.consensusId, "theirConsensusId", consensusId, "consensus", consensus)
		return
	}

	// check block hash
	if bytes.Compare(blockHash[:], blockHeaderObj.CalculateBlockHash()[:]) != 0 || bytes.Compare(blockHeaderObj.Hash[:], blockHeaderObj.CalculateBlockHash()[:]) != 0 {
		consensus.Log.Warn("Block hash doesn't match", "consensus", consensus)
		return
	}

	// check block data (transactions
	if !consensus.BlockVerifier(&blockHeaderObj) {
		consensus.Log.Warn("Block content is not verified successfully", "consensus", consensus)
		return
	}

	secret, msgToSend := consensus.constructCommitMessage(proto_consensus.COMMIT)
	// Store the commitment secret
	consensus.secret = secret

	p2p.SendMessage(consensus.leader, msgToSend)

	// Set state to COMMIT_DONE
	consensus.state = COMMIT_DONE
}

// Processes the challenge message sent from the leader
func (consensus *Consensus) processChallengeMessage(payload []byte, targetState ConsensusState) {
	//#### Read payload data
	offset := 0
	// 4 byte consensus id
	consensusId := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4

	// 32 byte block hash
	blockHash := payload[offset : offset+32]
	offset += 32

	// 2 byte leader id
	leaderId := binary.BigEndian.Uint16(payload[offset : offset+2])
	offset += 2

	// 33 byte of aggregated commit
	aggreCommit := payload[offset : offset+33]
	offset += 33

	// 33 byte of aggregated key
	aggreKey := payload[offset : offset+33]
	offset += 33

	// 32 byte of challenge
	challenge := payload[offset : offset+32]
	offset += 32

	// 64 byte of signature on previous data
	signature := payload[offset : offset+64]
	offset += 64
	//#### END: Read payload data

	// Update readyByConsensus for attack.
	attack.GetInstance().UpdateConsensusReady(consensusId)

	// Verify block data and the aggregated signatures
	// check leader Id
	myLeaderId := utils.GetUniqueIdFromPeer(consensus.leader)
	if leaderId != myLeaderId {
		consensus.Log.Warn("Received message from wrong leader", "myLeaderId", myLeaderId, "receivedLeaderId", leaderId, "consensus", consensus)
		return
	}

	// Verify signature
	if schnorr.Verify(crypto.Ed25519Curve, consensus.leader.PubKey, payload[:offset-64], signature) != nil {
		consensus.Log.Warn("Received message with invalid signature", "leaderKey", consensus.leader.PubKey, "consensus", consensus)
		return
	}

	consensus.mutex.Lock()

	// Add attack model of IncorrectResponse.
	if attack.GetInstance().IncorrectResponse() {
		consensus.Log.Warn("IncorrectResponse attacked")
		consensus.mutex.Unlock()
		return
	}

	// check block hash
	if bytes.Compare(blockHash[:], consensus.blockHash[:]) != 0 {
		consensus.Log.Warn("Block hash doesn't match", "consensus", consensus)
		consensus.mutex.Unlock()
		return
	}

	// check consensus Id
	if consensusId != consensus.consensusId {
		consensus.Log.Warn("Received message with wrong consensus Id", "myConsensusId", consensus.consensusId, "theirConsensusId", consensusId, "consensus", consensus)
		if _, ok := consensus.blocksReceived[consensus.consensusId]; !ok {
			consensus.mutex.Unlock()
			return
		}
		consensus.Log.Warn("ROLLING UP", "consensus", consensus)
		// If I received previous block (which haven't been processed. I will roll up to current block if everything checks.
	}

	aggCommitment := crypto.Ed25519Curve.Point()
	aggCommitment.UnmarshalBinary(aggreCommit[:32]) // TODO: figure out whether it's 33 bytes or 32 bytes
	aggKey := crypto.Ed25519Curve.Point()
	aggKey.UnmarshalBinary(aggreKey[:32])

	reconstructedChallenge, err := crypto.Challenge(crypto.Ed25519Curve, aggCommitment, aggKey, payload[:36]) // Only consensus Id and block hash

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

	response, err := crypto.Response(crypto.Ed25519Curve, consensus.priKey, consensus.secret, receivedChallenge)
	if err != nil {
		log.Warn("Failed to generate response", "err", err)
		return
	}
	msgTypeToSend := proto_consensus.RESPONSE
	if targetState == FINAL_RESPONSE_DONE {
		msgTypeToSend = proto_consensus.FINAL_RESPONSE
	}
	msgToSend := consensus.constructResponseMessage(msgTypeToSend, response)

	p2p.SendMessage(consensus.leader, msgToSend)

	// Set state to target state (RESPONSE_DONE, FINAL_RESPONSE_DONE)
	consensus.state = targetState

	if consensus.state == FINAL_RESPONSE_DONE {
		// BIG TODO: the block catch up logic is basically a mock now. More checks need to be done to make it correct.
		// The logic is to roll up to the latest blocks one by one to try catching up with the leader.
		for {
			val, ok := consensus.blocksReceived[consensus.consensusId]
			if ok {
				delete(consensus.blocksReceived, consensus.consensusId)

				consensus.blockHash = [32]byte{}
				consensus.consensusId++ // roll up one by one, until the next block is not received yet.

				// TODO: think about when validators know about the consensus is reached.
				// For now, the blockchain is updated right here.

				// TODO: reconstruct the whole block from header and transactions
				// For now, we used the stored whole block in consensus.blockHeader
				txDecoder := gob.NewDecoder(bytes.NewReader(val.blockHeader))
				var blockHeaderObj blockchain.Block
				err := txDecoder.Decode(&blockHeaderObj)
				if err != nil {
					consensus.Log.Debug("failed to construct the new block after consensus")
				}
				// check block data (transactions
				if !consensus.BlockVerifier(&blockHeaderObj) {
					consensus.Log.Debug("[WARNING] Block content is not verified successfully", "consensusId", consensus.consensusId)
					consensus.mutex.Unlock()
					return
				}
				consensus.OnConsensusDone(&blockHeaderObj)
			} else {
				break
			}

		}
	}
	consensus.mutex.Unlock()
}

// Processes the collective signature message sent from the leader
func (consensus *Consensus) processCollectiveSigMessage(payload []byte) {
	//#### Read payload data
	offset := 0
	// 4 byte consensus id
	consensusId := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4

	// 32 byte block hash
	blockHash := payload[offset : offset+32]
	offset += 32

	// 2 byte leader id
	leaderId := binary.BigEndian.Uint16(payload[offset : offset+2])
	offset += 2

	// 64 byte of collective signature
	collectiveSig := payload[offset : offset+64]
	offset += 64

	// N byte of bitmap
	n := len(payload) - offset - 64 // the number means 64 signature
	bitmap := payload[offset : offset+n]
	offset += n

	// 64 byte of signature on previous data
	signature := payload[offset : offset+64]
	offset += 64
	//#### END: Read payload data

	copy(consensus.blockHash[:], blockHash[:])

	// Verify block data
	// check leader Id
	myLeaderId := utils.GetUniqueIdFromPeer(consensus.leader)
	if leaderId != myLeaderId {
		consensus.Log.Warn("Received message from wrong leader", "myLeaderId", myLeaderId, "receivedLeaderId", leaderId, "consensus", consensus)
		return
	}

	// Verify signature
	if schnorr.Verify(crypto.Ed25519Curve, consensus.leader.PubKey, payload[:offset-64], signature) != nil {
		consensus.Log.Warn("Received message with invalid signature", "leaderKey", consensus.leader.PubKey, "consensus", consensus)
		return
	}

	// Verify collective signature
	err := crypto.Verify(crypto.Ed25519Curve, consensus.publicKeys, payload[:36], append(collectiveSig, bitmap...), crypto.NewThresholdPolicy((2*len(consensus.publicKeys)/3)+1))
	if err != nil {
		consensus.Log.Warn("Failed to verify the collective sig message", "consensusId", consensusId, "err", err)
	}

	// Add attack model of IncorrectResponse.
	if attack.GetInstance().IncorrectResponse() {
		consensus.Log.Warn("IncorrectResponse attacked")
		return
	}

	// check consensus Id
	if consensusId != consensus.consensusId {
		consensus.Log.Warn("Received message with wrong consensus Id", "myConsensusId", consensus.consensusId, "theirConsensusId", consensusId, "consensus", consensus)
		return
	}

	// check block hash
	if bytes.Compare(blockHash[:], consensus.blockHash[:]) != 0 {
		consensus.Log.Warn("Block hash doesn't match", "consensus", consensus)
		return
	}

	secret, msgToSend := consensus.constructCommitMessage(proto_consensus.FINAL_COMMIT)
	// Store the commitment secret
	consensus.secret = secret

	p2p.SendMessage(consensus.leader, msgToSend)

	// Set state to COMMIT_DONE
	consensus.state = FINAL_COMMIT_DONE
}
