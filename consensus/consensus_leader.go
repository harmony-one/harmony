package consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/p2p"
	proto_consensus "harmony-benchmark/proto/consensus"
	"strings"
	"time"
)

var (
	startTime time.Time
)

// Waits for the next new block to run consensus on
func (consensus *Consensus) WaitForNewBlock(blockChannel chan blockchain.Block) {
	consensus.Log.Debug("Waiting for block", "consensus", consensus)
	for { // keep waiting for new blocks
		newBlock := <-blockChannel
		// TODO: think about potential race condition
		startTime = time.Now()
		consensus.Log.Info("STARTING CONSENSUS", "consensus", consensus, "startTime", startTime)
		for consensus.state == FINISHED {
			time.Sleep(500 * time.Millisecond)
			consensus.startConsensus(&newBlock)
			break
		}
	}
}

// Consensus message dispatcher for the leader
func (consensus *Consensus) ProcessMessageLeader(message []byte) {
	msgType, err := proto_consensus.GetConsensusMessageType(message)
	if err != nil {
		consensus.Log.Error("Failed to get consensus message type.", "err", err, "consensus", consensus)
	}

	payload, err := proto_consensus.GetConsensusMessagePayload(message)
	if err != nil {
		consensus.Log.Error("Failed to get consensus message payload.", "err", err, "consensus", consensus)
	}

	switch msgType {
	case proto_consensus.ANNOUNCE:
		consensus.Log.Error("Unexpected message type", "msgType", msgType, "consensus", consensus)
	case proto_consensus.COMMIT:
		consensus.processCommitMessage(payload)
	case proto_consensus.CHALLENGE:
		consensus.Log.Error("Unexpected message type", "msgType", msgType, "consensus", consensus)
	case proto_consensus.RESPONSE:
		consensus.processResponseMessage(payload)
	case proto_consensus.START_CONSENSUS:
		consensus.processStartConsensusMessage(payload)
	default:
		consensus.Log.Error("Unexpected message type", "msgType", msgType, "consensus", consensus)
	}
}

// Handler for message which triggers consensus process
func (consensus *Consensus) processStartConsensusMessage(payload []byte) {
	tx := blockchain.NewCoinbaseTX("x", "y", 0)
	consensus.startConsensus(blockchain.NewGenesisBlock(tx, 0))
}

// Starts a new consensus for a block by broadcast a announce message to the validators
func (consensus *Consensus) startConsensus(newBlock *blockchain.Block) {
	// Copy over block hash and block header data
	copy(consensus.blockHash[:], newBlock.Hash[:])

	// prepare message and broadcast to validators
	byteBuffer := bytes.NewBuffer([]byte{})
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(newBlock)
	consensus.blockHeader = byteBuffer.Bytes()

	msgToSend := consensus.constructAnnounceMessage()
	p2p.BroadcastMessage(consensus.validators, msgToSend)
	// Set state to ANNOUNCE_DONE
	consensus.state = ANNOUNCE_DONE
	// Generate leader's own commitment
}

// Constructs the announce message
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

	return proto_consensus.ConstructConsensusMessage(proto_consensus.ANNOUNCE, buffer.Bytes())
}

func signMessage(message []byte) []byte {
	// TODO: implement real ECC signature
	mockSignature := sha256.Sum256(message)
	return append(mockSignature[:], mockSignature[:]...)
}

// Processes the commit message sent from validators
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

	// 32 byte commit
	commit := payload[offset : offset+32]
	offset += 32

	// 64 byte of signature on previous data
	signature := payload[offset : offset+64]
	offset += 64
	//#### END: Read payload data

	// TODO: make use of the data. This is just to avoid the unused variable warning
	_ = commit
	_ = signature

	// check consensus Id
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	if consensusId != consensus.consensusId {
		consensus.Log.Warn("Received COMMIT with wrong consensus Id", "myConsensusId", consensus.consensusId, "theirConsensusId", consensusId, "consensus", consensus)
		return
	}

	if bytes.Compare(blockHash, consensus.blockHash[:]) != 0 {
		consensus.Log.Warn("Received COMMIT with wrong blockHash", "myConsensusId", consensus.consensusId, "theirConsensusId", consensusId, "consensus", consensus)
		return
	}

	// proceed only when the message is not received before
	_, ok := consensus.commits[validatorId]
	shouldProcess := !ok
	if shouldProcess {
		consensus.commits[validatorId] = validatorId
	}

	if !shouldProcess {
		return
	}

	if len(consensus.commits) >= (2*len(consensus.validators))/3+1 && consensus.state < CHALLENGE_DONE {
		consensus.Log.Debug("Enough commits received with signatures", "numOfSignatures", len(consensus.commits))

		// Broadcast challenge
		msgToSend := consensus.constructChallengeMessage()
		p2p.BroadcastMessage(consensus.validators, msgToSend)

		// Set state to CHALLENGE_DONE
		consensus.state = CHALLENGE_DONE
	}
}

// Construct the challenge message
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

	return proto_consensus.ConstructConsensusMessage(proto_consensus.CHALLENGE, buffer.Bytes())
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

// Processes the response message sent from validators
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

	shouldProcess := true
	consensus.mutex.Lock()
	// check consensus Id
	if consensusId != consensus.consensusId {
		shouldProcess = false
		consensus.Log.Warn("Received RESPONSE with wrong consensus Id", "myConsensusId", consensus.consensusId, "theirConsensusId", consensusId, "consensus", consensus)
	}

	if bytes.Compare(blockHash, consensus.blockHash[:]) != 0 {
		consensus.Log.Warn("Received RESPONSE with wrong blockHash", "myConsensusId", consensus.consensusId, "theirConsensusId", consensusId, "consensus", consensus)
		return
	}

	// proceed only when the message is not received before
	_, ok := consensus.responses[validatorId]
	shouldProcess = shouldProcess && !ok
	if shouldProcess {
		consensus.responses[validatorId] = validatorId
		//consensus.Log.Debug("Number of responses received", "consensusId", consensus.consensusId, "count", len(consensus.responses))
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
			// For now, we used the stored whole block already stored in consensus.blockHeader
			txDecoder := gob.NewDecoder(bytes.NewReader(consensus.blockHeader))
			var blockHeaderObj blockchain.Block
			err := txDecoder.Decode(&blockHeaderObj)
			if err != nil {
				consensus.Log.Debug("failed to construct the new block after consensus")
			}
			consensus.OnConsensusDone(&blockHeaderObj)

			// TODO: @ricl these logic are irrelevant to consensus, move them to another file, say profiler.
			endTime := time.Now()
			timeElapsed := endTime.Sub(startTime)
			numOfTxs := blockHeaderObj.NumTransactions
			consensus.Log.Info("TPS Report",
				"numOfTXs", numOfTxs,
				"startTime", startTime,
				"endTime", endTime,
				"timeElapsed", timeElapsed,
				"TPS", float64(numOfTxs)/timeElapsed.Seconds(),
				"consensus", consensus)

			// Send signal to Node so the new block can be added and new round of consensus can be triggered
			consensus.ReadySignal <- 1
		}
		consensus.mutex.Unlock()
	}
}
