package consensus

import (
	"log"
	"sync"

	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/p2p"
)

var mutex = &sync.Mutex{}

func (consensus *Consensus) WaitForNewBlock(blockChannel chan blockchain.Block) {
	for { // keep waiting for new blocks
		newBlock := <-blockChannel
		// TODO: think about potential race condition
		if consensus.state == READY {
			consensus.startConsensus(&newBlock)
		}
	}
}

// Leader's consensus message dispatcher
func (consensus *Consensus) ProcessMessageLeader(message []byte) {
	msgType, err := GetConsensusMessageType(message)
	if err != nil {
		log.Print(err)
	}

	payload, err := GetConsensusMessagePayload(message)
	if err != nil {
		log.Print(err)
	}

	log.Printf("[Leader] Received and processing message: %s\n", msgType)
	switch msgType {
	case ANNOUNCE:
		log.Printf("Unexpected message type: %s", msgType)
	case COMMIT:
		consensus.processCommitMessage(payload)
	case CHALLENGE:
		log.Printf("Unexpected message type: %s", msgType)
	case RESPONSE:
		consensus.processResponseMessage(payload)
	case START_CONSENSUS:
		consensus.processStartConsensusMessage(payload)
	default:
		log.Printf("Unexpected message type: %s", msgType)
	}
}

// Handler for message which triggers consensus process
func (consensus *Consensus) processStartConsensusMessage(payload []byte) {
	tx, _ := blockchain.NewCoinbaseTX("x", "y") // TODO: @minh, this fix is temporary to unblock the build. Please fix later.
	consensus.startConsensus(blockchain.NewGenesisBlock(tx))
}

func (consensus *Consensus) startConsensus(newBlock *blockchain.Block) {
	// prepare message and broadcast to validators
	// Construct new block
	//newBlock := constructNewBlock()
	consensus.blockHash = newBlock.Hash

	msgToSend, err := consensus.constructAnnounceMessage()
	if err != nil {
		return
	}
	// Set state to ANNOUNCE_DONE
	consensus.state = ANNOUNCE_DONE
	p2p.BroadcastMessage(consensus.validators, msgToSend)
}

// Construct the announce message to send to validators
func (consensus Consensus) constructAnnounceMessage() ([]byte, error) {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	if len(consensus.blockHash) != 32 {
		return buffer.Bytes(), errors.New(fmt.Sprintf("Block Hash size is %d bytes", len(consensus.blockHash)))
	}
	buffer.Write(consensus.blockHash)

	// 2 byte leader id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
	buffer.Write(twoBytes)

	// n byte of block header
	blockHeader := getBlockHeader()
	buffer.Write(blockHeader)

	// 4 byte of payload size
	sizeOfPayload := uint32(len(blockHeader))
	binary.BigEndian.PutUint32(fourBytes, sizeOfPayload)
	buffer.Write(fourBytes)

	// 64 byte of signature on previous data
	signature := signMessage(buffer.Bytes())
	buffer.Write(signature)

	return consensus.ConstructConsensusMessage(ANNOUNCE, buffer.Bytes()), nil
}

// TODO: fill in this function
func constructNewBlock() []byte {
	return make([]byte, 200)
}

// TODO: fill in this function
func getBlockHash(block []byte) []byte {
	return make([]byte, 32)
}

// TODO: fill in this function
func getBlockHeader() []byte {
	return make([]byte, 200)
}

// TODO: fill in this function
func signMessage(message []byte) []byte {
	return make([]byte, 64)
}

func (consensus *Consensus) processCommitMessage(payload []byte) {
	//#### Read payload data
	offset := 0
	// 4 byte consensus id
	consensusId := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4

	// 32 byte block hash
	blockHash := payload[offset : offset+32]
	offset += 32

	// 2 byte validator id
	validatorId := string(payload[offset : offset+2])
	offset += 2

	// 33 byte commit
	commit := payload[offset : offset+33]
	offset += 33

	// 64 byte of signature on previous data
	signature := payload[offset : offset+64]
	offset += 64
	//#### END: Read payload data

	// TODO: make use of the data. This is just to avoid the unused variable warning
	_ = consensusId
	_ = blockHash
	_ = commit
	_ = signature

	// proceed only when the message is not received before and this consensus phase is not done.
	mutex.Lock()
	_, ok := consensus.commits[validatorId]
	shouldProcess := !ok && consensus.state == ANNOUNCE_DONE
	if shouldProcess {
		consensus.commits[validatorId] = validatorId
		log.Printf("Number of commits received: %d", len(consensus.commits))
	}
	mutex.Unlock()

	if !shouldProcess {
		return
	}

	mutex.Lock()
	if len(consensus.commits) >= (2*len(consensus.validators))/3+1 {
		log.Printf("Enough commits received with %d signatures", len(consensus.commits))
		if consensus.state == ANNOUNCE_DONE {
			// Set state to CHALLENGE_DONE
			consensus.state = CHALLENGE_DONE
		}
		// Broadcast challenge
		msgToSend := consensus.constructChallengeMessage()
		p2p.BroadcastMessage(consensus.validators, msgToSend)
	}
	mutex.Unlock()
}

// Construct the challenge message to send to validators
func (consensus Consensus) constructChallengeMessage() []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash)

	// 2 byte leader id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
	buffer.Write(twoBytes)

	// 33 byte aggregated commit
	buffer.Write(getAggregatedCommit())

	// 33 byte aggregated key
	buffer.Write(getAggregatedKey())

	// 32 byte challenge
	buffer.Write(getChallenge())

	// 64 byte of signature on previous data
	signature := signMessage(buffer.Bytes())
	buffer.Write(signature)

	return consensus.ConstructConsensusMessage(CHALLENGE, buffer.Bytes())
}

// TODO: fill in this function
func getAggregatedCommit() []byte {
	return make([]byte, 33)
}

// TODO: fill in this function
func getAggregatedKey() []byte {
	return make([]byte, 33)
}

// TODO: fill in this function
func getChallenge() []byte {
	return make([]byte, 32)
}

func (consensus *Consensus) processResponseMessage(payload []byte) {
	//#### Read payload data
	offset := 0
	// 4 byte consensus id
	consensusId := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4

	// 32 byte block hash
	blockHash := payload[offset : offset+32]
	offset += 32

	// 2 byte validator id
	validatorId := string(payload[offset : offset+2])
	offset += 2

	// 32 byte response
	response := payload[offset : offset+32]
	offset += 32

	// 64 byte of signature on previous data
	signature := payload[offset : offset+64]
	offset += 64
	//#### END: Read payload data

	// TODO: make use of the data. This is just to avoid the unused variable warning
	_ = consensusId
	_ = blockHash
	_ = response
	_ = signature
	// proceed only when the message is not received before and this consensus phase is not done.
	mutex.Lock()
	_, ok := consensus.responses[validatorId]
	shouldProcess := !ok && consensus.state == CHALLENGE_DONE
	if shouldProcess {
		consensus.responses[validatorId] = validatorId
		log.Printf("Number of responses received: %d", len(consensus.responses))
	}
	mutex.Unlock()

	if !shouldProcess {
		return
	}

	mutex.Lock()
	if len(consensus.responses) >= (2*len(consensus.validators))/3+1 {
		log.Printf("Consensus reached with %d signatures.", len(consensus.responses))
		if consensus.state == CHALLENGE_DONE {
			// Set state to FINISHED
			consensus.state = FINISHED
			// TODO: do followups on the consensus
			log.Printf("HOORAY!!! CONSENSUS REACHED AMONG %d NODES!!!\n", len(consensus.validators))
			consensus.ResetState()
			consensus.ReadySignal <- 1
		}
		// TODO: composes new block and broadcast the new block to validators
	}
	mutex.Unlock()
}
