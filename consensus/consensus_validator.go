package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/p2p"
	"regexp"
	"strconv"
)

// Validator's consensus message dispatcher
func (consensus *Consensus) ProcessMessageValidator(message []byte) {
	msgType, err := GetConsensusMessageType(message)
	if err != nil {
		consensus.Log.Error("Failed to get consensus message type", "err", err, "consensus", consensus)
	}

	payload, err := GetConsensusMessagePayload(message)
	if err != nil {
		consensus.Log.Error("Failed to get consensus message payload", "err", err, "consensus", consensus)
	}

	switch msgType {
	case ANNOUNCE:
		consensus.processAnnounceMessage(payload)
	case COMMIT:
		consensus.Log.Error("Unexpected message type", "msgType", msgType, "consensus", consensus)
	case CHALLENGE:
		consensus.processChallengeMessage(payload)
	case RESPONSE:
		consensus.Log.Error("Unexpected message type", "msgType", msgType, "consensus", consensus)
	default:
		consensus.Log.Error("Unexpected message type", "msgType", msgType, "consensus", consensus)
	}
}

func (consensus *Consensus) processAnnounceMessage(payload []byte) {
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

	// n byte of block header
	n := len(payload) - offset - 4 - 64 // the numbers means 4 byte payload and 64 signature
	blockHeader := payload[offset : offset+n]
	offset += n

	// 4 byte of payload size (block header)
	blockHeaderSize := payload[offset : offset+4]
	offset += 4

	// 64 byte of signature on previous data
	signature := payload[offset : offset+64]
	offset += 64
	//#### END: Read payload data

	// TODO: make use of the data. This is just to avoid the unused variable warning
	_ = consensusId

	_ = leaderId
	_ = blockHeader
	_ = blockHeaderSize
	_ = signature

	copy(consensus.blockHash[:], blockHash[:])

	// Verify block data
	// check leader Id
	leaderPrivKey := consensus.leader.Ip + consensus.leader.Port
	reg, _ := regexp.Compile("[^0-9]+")
	socketId := reg.ReplaceAllString(leaderPrivKey, "")
	value, _ := strconv.Atoi(socketId)
	if leaderId != uint16(value) {
		consensus.Log.Warn("Received message from wrong leader", "myLeaderId", consensus.consensusId, "receivedLeaderId", consensusId, "consensus", consensus)
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

	// TODO: return the signature(commit) to leader
	// For now, simply return the private key of this node.
	msgToSend := consensus.constructCommitMessage()
	// consensus.Log.Debug("SENDING COMMIT", "consensusId", consensus.consensusId, "consensus", consensus)
	p2p.SendMessage(consensus.leader, msgToSend)

	// Set state to COMMIT_DONE
	consensus.state = COMMIT_DONE
}

// Construct the commit message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructCommitMessage() []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:])

	// 2 byte validator id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
	buffer.Write(twoBytes)

	// 33 byte of commit
	commit := getCommitMessage()
	buffer.Write(commit)

	// 64 byte of signature on previous data
	signature := signMessage(buffer.Bytes())
	buffer.Write(signature)

	return consensus.ConstructConsensusMessage(COMMIT, buffer.Bytes())
}

func getCommitMessage() []byte {
	// TODO: use real cosi signature
	return make([]byte, 33)
}

func (consensus *Consensus) processChallengeMessage(payload []byte) {
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

	// 32 byte of aggregated key
	challenge := payload[offset : offset+32]
	offset += 32

	// 64 byte of signature on previous data
	signature := payload[offset : offset+64]
	offset += 64
	//#### END: Read payload data

	// TODO: make use of the data. This is just to avoid the unused variable warning
	_ = consensusId
	_ = blockHash
	_ = leaderId
	_ = aggreCommit
	_ = aggreKey
	_ = challenge
	_ = signature

	// Verify block data and the aggregated signatures

	// check leader Id
	leaderPrivKey := consensus.leader.Ip + consensus.leader.Port
	reg, _ := regexp.Compile("[^0-9]+")
	socketId := reg.ReplaceAllString(leaderPrivKey, "")
	value, _ := strconv.Atoi(socketId)
	if leaderId != uint16(value) {
		consensus.Log.Warn("Received message from wrong leader", "myLeaderId", consensus.consensusId, "receivedLeaderId", consensusId, "consensus", consensus)
		return
	}

	consensus.mutex.Lock()
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

	// TODO: verify aggregated commits with real schnor cosign verification

	// TODO: return the signature(response) to leader
	// For now, simply return the private key of this node.
	msgToSend := consensus.constructResponseMessage()
	// consensus.Log.Debug("SENDING RESPONSE", "consensusId", consensus.consensusId, "consensus", consensus)
	p2p.SendMessage(consensus.leader, msgToSend)

	// Set state to RESPONSE_DONE
	consensus.state = RESPONSE_DONE

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
	consensus.mutex.Unlock()
}

// Construct the response message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructResponseMessage() []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:32])

	// 2 byte validator id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
	buffer.Write(twoBytes)

	// 32 byte of response
	response := getResponseMessage()
	buffer.Write(response)

	// 64 byte of signature on previous data
	signature := signMessage(buffer.Bytes())
	buffer.Write(signature)

	return consensus.ConstructConsensusMessage(RESPONSE, buffer.Bytes())
}

// TODO: fill in this function
func getResponseMessage() []byte {
	return make([]byte, 32)
}
