package consensus

import (
	"harmony-benchmark/p2p"
	"log"
	"bytes"
	"encoding/binary"
)

// Validator's consensus message dispatcher
func (consensus *Consensus) ProcessMessageValidator(message []byte) {
	msgType, err := GetConsensusMessageType(message)
	if err != nil {
		log.Print(err)
	}

	payload, err := GetConsensusMessagePayload(message)
	if err != nil {
		log.Print(err)
	}

	log.Printf("[Validator] Received and processing message: %s\n", msgType)
	switch msgType {
	case ANNOUNCE:
		consensus.processAnnounceMessage(payload)
	case COMMIT:
		log.Println("Unexpected message type: %s", msgType)
	case CHALLENGE:
		consensus.processChallengeMessage(payload)
	case RESPONSE:
		log.Println("Unexpected message type: %s", msgType)
	default:
		log.Println("Unexpected message type: %s", msgType)
	}
}

func (consensus *Consensus) processAnnounceMessage(payload []byte) {
	//#### Read payload data
	offset := 0
	// 4 byte consensus id
	consensusId := binary.BigEndian.Uint32(payload[offset:offset+4])
	offset += 4

	// 32 byte block hash
	blockHash := payload[offset:offset+32]
	offset += 32

	// 2 byte validator id
	leaderId := string(payload[offset:offset+2])
	offset += 2

	// n byte of block header
	n := len(payload) - offset - 4 - 64 // the numbers means 4 byte payload and 64 signature
	blockHeader := payload[offset:offset+n]
	offset += n

	// 4 byte of payload size (block header)
	blockHeaderSize := payload[offset:offset+4]
	offset += 4

	// 64 byte of signature on previous data
	signature := payload[offset:offset+64]
	offset += 64
	//#### END: Read payload data

	// TODO: make use of the data. This is just to avoid the unused variable warning
	_ = consensusId
	_ = blockHash
	_ = leaderId
	_ = blockHeader
	_ = blockHeaderSize
	_ = signature

	consensus.blockHash = blockHash
	// verify block data

	// sign block

	// TODO: return the signature(commit) to leader
	// For now, simply return the private key of this node.
	msgToSend := consensus.constructCommitMessage()
	p2p.SendMessage(consensus.leader, msgToSend)

	// Set state to COMMIT_DONE
	consensus.state = COMMIT_DONE
}

// Construct the commit message to send to leader (assumption the consensus data is already verified)
func (consensus Consensus) constructCommitMessage() []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash)

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

// TODO: fill in this function
func getCommitMessage() []byte {
	return make([]byte, 33)
}

func (consensus *Consensus) processChallengeMessage(payload []byte) {
	//#### Read payload data
	offset := 0
	// 4 byte consensus id
	consensusId := binary.BigEndian.Uint32(payload[offset:offset+4])
	offset += 4

	// 32 byte block hash
	blockHash := payload[offset:offset+32]
	offset += 32

	// 2 byte leader id
	leaderId := string(payload[offset:offset+2])
	offset += 2

	// 33 byte of aggregated commit
	aggreCommit := payload[offset:offset+33]
	offset += 33

	// 33 byte of aggregated key
	aggreKey := payload[offset:offset+33]
	offset += 33

	// 32 byte of aggregated key
	challenge := payload[offset:offset+32]
	offset += 32

	// 64 byte of signature on previous data
	signature := payload[offset:offset+64]
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

	// verify block data and the aggregated signatures

	// sign the message

	// TODO: return the signature(response) to leader
	// For now, simply return the private key of this node.
	msgToSend := consensus.constructResponseMessage()
	p2p.SendMessage(consensus.leader, msgToSend)

	// Set state to RESPONSE_DONE
	consensus.state = RESPONSE_DONE
}

// Construct the response message to send to leader (assumption the consensus data is already verified)
func (consensus Consensus) constructResponseMessage() []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash)

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
