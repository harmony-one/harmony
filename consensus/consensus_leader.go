package consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/p2p"
	"strings"
	"encoding/gob"
	"fmt"
	"time"
)

// WaitForNewBlock waits for a new block.
func (consensus *Consensus) WaitForNewBlock(blockChannel chan blockchain.Block) {
	consensus.Log.Debug("Waiting for block", "consensus", consensus)
	for { // keep waiting for new blocks
		newBlock := <-blockChannel
		// TODO: think about potential race condition
		consensus.Log.Debug("STARTING CONSENSUS", "consensus", consensus)
		for consensus.state == FINISHED {
			time.Sleep(500 * time.Millisecond)
			consensus.startConsensus(&newBlock)
			break
		}
	}
}

// ProcessMessageLeader is the leader's consensus message dispatcher
func (consensus *Consensus) ProcessMessageLeader(message []byte) {
	msgType, err := GetConsensusMessageType(message)
	if err != nil {
		consensus.Log.Error("Failed to get consensus message type.", "err", err, "consensus", consensus)
	}

	payload, err := GetConsensusMessagePayload(message)
	if err != nil {
		consensus.Log.Error("Failed to get consensus message payload.", "err", err, "consensus", consensus)
	}

	switch msgType {
	case ANNOUNCE:
		consensus.Log.Error("Unexpected message type", "msgType", msgType, "consensus", consensus)
	case COMMIT:
		consensus.processCommitMessage(payload)
	case CHALLENGE:
		consensus.Log.Error("Unexpected message type", "msgType", msgType, "consensus", consensus)
	case RESPONSE:
		consensus.processResponseMessage(payload)
	case START_CONSENSUS:
		consensus.processStartConsensusMessage(payload)
	default:
		consensus.Log.Error("Unexpected message type", "msgType", msgType, "consensus", consensus)
	}
}

// Handler for message which triggers consensus process
func (consensus *Consensus) processStartConsensusMessage(payload []byte) {
	tx := blockchain.NewCoinbaseTX("x", "y")
	consensus.startConsensus(blockchain.NewGenesisBlock(tx))
}

func (consensus *Consensus) startConsensus(newBlock *blockchain.Block) {
	// prepare message and broadcast to validators

	// Copy over block hash and block header data
	copy(consensus.blockHash[:], newBlock.Hash[:])

	byteBuffer := bytes.NewBuffer([]byte{})
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(newBlock)
	consensus.blockHeader = byteBuffer.Bytes()

	msgToSend := consensus.constructAnnounceMessage()
	fmt.Printf("BROADCAST ANNOUNCE: %d\n", consensus.consensusId)
	p2p.BroadcastMessage(consensus.validators, msgToSend)
	fmt.Printf("BROADCAST ANNOUNCE DONE: %d\n", consensus.consensusId)
	// Set state to ANNOUNCE_DONE
	consensus.state = ANNOUNCE_DONE
}

// Construct the announce message to send to validators
func (consensus *Consensus) constructAnnounceMessage() []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:])

	// 2 byte leader id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
	buffer.Write(twoBytes)

	// n byte of block header
	buffer.Write(consensus.blockHeader)

	// 4 byte of payload size
	sizeOfPayload := uint32(len(consensus.blockHeader))
	binary.BigEndian.PutUint32(fourBytes, sizeOfPayload)
	buffer.Write(fourBytes)

	// 64 byte of signature on previous data
	signature := signMessage(buffer.Bytes())
	buffer.Write(signature)

	consensus.Log.Debug("SENDING ANNOUNCE")
	return consensus.ConstructConsensusMessage(ANNOUNCE, buffer.Bytes())
}

func signMessage(message []byte) []byte {
	// TODO: implement real ECC signature
	mockSignature := sha256.Sum256(message)
	return append(mockSignature[:], mockSignature[:]...)
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

	// check consensus Id
	if consensusId != consensus.consensusId {
		consensus.Log.Debug("[ERROR] Received COMMIT with wrong consensus Id", "myConsensusId", consensus.consensusId, "theirConsensusId", consensusId, "consensus", consensus)
		return
	}

	// proceed only when the message is not received before and this consensus phase is not done.
	consensus.mutex.Lock()
	_, ok := consensus.commits[validatorId]
	shouldProcess := !ok
	if shouldProcess {
		consensus.commits[validatorId] = validatorId
		//consensus.Log.Debug("Number of commits received", "count", len(consensus.commits))
		fmt.Printf("Number of COMMITS received %d\n", len(consensus.commits))
	}
	consensus.mutex.Unlock()

	if !shouldProcess {
		return
	}

	if len(consensus.commits) >= (2*len(consensus.validators))/3+1 && consensus.state < CHALLENGE_DONE {
		consensus.mutex.Lock()
		if len(consensus.commits) >= (2*len(consensus.validators))/3+1 && consensus.state < CHALLENGE_DONE {
			consensus.Log.Debug("Enough commits received with signatures", "numOfSignatures", len(consensus.commits))
			// Broadcast challenge
			msgToSend := consensus.constructChallengeMessage()
			//fmt.Printf("BROADCAST CHALLENGE: %d\n", consensus.consensusId)
			p2p.BroadcastMessage(consensus.validators, msgToSend)
			//fmt.Printf("BROADCAST CHALLENGE DONE: %d\n", consensus.consensusId)
			// Set state to CHALLENGE_DONE
			consensus.state = CHALLENGE_DONE
		}
		consensus.mutex.Unlock()
	}
}

// Construct the challenge message to send to validators
func (consensus *Consensus) constructChallengeMessage() []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:])

	// 2 byte leader id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
	buffer.Write(twoBytes)

	// 33 byte aggregated commit
	buffer.Write(getAggregatedCommit(consensus.commits))

	// 33 byte aggregated key
	buffer.Write(getAggregatedKey(consensus.commits))

	// 32 byte challenge
	buffer.Write(getChallenge())

	// 64 byte of signature on previous data
	signature := signMessage(buffer.Bytes())
	buffer.Write(signature)

	return consensus.ConstructConsensusMessage(CHALLENGE, buffer.Bytes())
}

func getAggregatedCommit(commits map[string]string) []byte {
	// TODO: implement actual commit aggregation
	var commitArray []string
	for _, val := range commits {
		commitArray = append(commitArray, val)
	}
	var commit [32]byte
	commit = sha256.Sum256([]byte(strings.Join(commitArray, "")))
	return append(commit[:], byte(0))
}

func getAggregatedKey(commits map[string]string) []byte {
	// TODO: implement actual key aggregation
	var commitArray []string
	for key := range commits {
		commitArray = append(commitArray, key)
	}
	var commit [32]byte
	commit = sha256.Sum256([]byte(strings.Join(commitArray, "")))
	return append(commit[:], byte(0))
}

func getChallenge() []byte {
	// TODO: implement actual challenge data
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


	// check consensus Id
	if consensusId != consensus.consensusId {
		consensus.Log.Debug("[ERROR] Received RESPONSE with wrong consensus Id", "myConsensusId", consensus.consensusId, "theirConsensusId", consensusId, "consensus", consensus)
		return
	}

	// proceed only when the message is not received before and this consensus phase is not done.
	consensus.mutex.Lock()
	_, ok := consensus.responses[validatorId]
	shouldProcess := !ok
	if shouldProcess {
		consensus.responses[validatorId] = validatorId
		//consensus.Log.Debug("Number of responses received", "count", len(consensus.responses), "consensudId", consensusId)
		fmt.Printf("Number of RESPONSES received %d\n", len(consensus.responses))
	}
	consensus.mutex.Unlock()

	if !shouldProcess {
		return
	}

	//consensus.Log.Debug("RECEIVED RESPONSE", "consensusId", consensusId)
	if len(consensus.responses) >= (2*len(consensus.validators))/3+1 && consensus.state != FINISHED {
		consensus.mutex.Lock()
		if len(consensus.responses) >= (2*len(consensus.validators))/3+1 && consensus.state != FINISHED {
			consensus.Log.Debug("Consensus reached with signatures.", "numOfSignatures", len(consensus.responses))
			// Reset state to FINISHED, and clear other data.
			consensus.ResetState()
			consensus.consensusId++
			consensus.Log.Debug("HOORAY!!! CONSENSUS REACHED!!!", "consensusId", consensus.consensusId)

			// TODO: reconstruct the whole block from header and transactions
			// For now, we used the stored whole block in consensus.blockHeader
			txDecoder := gob.NewDecoder(bytes.NewReader(consensus.blockHeader))
			var blockHeaderObj blockchain.Block
			err := txDecoder.Decode(&blockHeaderObj)
			if err != nil {
				consensus.Log.Debug("failed to construct the new block after consensus")
			}
			consensus.OnConsensusDone(&blockHeaderObj)

			// Send signal to Node so the new block can be added and new round of consensus can be triggered
			consensus.ReadySignal <- 1
		}
		consensus.mutex.Unlock()

		// TODO: composes new block and broadcast the new block to validators
	}
}
