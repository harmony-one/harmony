package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/crypto"
	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/p2p"
	proto_consensus "github.com/simple-rules/harmony-benchmark/proto/consensus"
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
	// TODO: remove these method after testnet
	tx := blockchain.NewCoinbaseTX([20]byte{0}, "y", 0)
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
	p2p.BroadcastMessage(consensus.getValidatorPeers(), msgToSend)
	// Set state to ANNOUNCE_DONE
	consensus.state = ANNOUNCE_DONE

	// Generate leader's own commitment
	secret, commitment := crypto.Commit(crypto.Ed25519Curve)
	consensus.secret = secret
	consensus.commitments[consensus.nodeId] = commitment
	consensus.bitmap.SetKey(consensus.pubKey, true)
}

// Processes the commit message sent from validators
func (consensus *Consensus) processCommitMessage(payload []byte) {
	// Read payload data
	offset := 0
	// 4 byte consensus id
	consensusId := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4

	// 32 byte block hash
	blockHash := payload[offset : offset+32]
	offset += 32

	// 2 byte validator id
	validatorId := binary.BigEndian.Uint16(payload[offset : offset+2])
	offset += 2

	// 32 byte commit
	commitment := payload[offset : offset+32]
	offset += 32

	// 64 byte of signature on all above data
	signature := payload[offset : offset+64]
	offset += 64

	// Verify signature
	value, ok := consensus.validators[validatorId]
	if !ok {
		consensus.Log.Warn("Received message from unrecognized validator", "validatorId", validatorId, "consensus", consensus)
		return
	}
	if schnorr.Verify(crypto.Ed25519Curve, value.PubKey, payload[:offset-64], signature) != nil {
		consensus.Log.Warn("Received message with invalid signature", "validatorKey", consensus.leader.PubKey, "consensus", consensus)
		return
	}

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
	_, ok = consensus.commitments[validatorId]
	shouldProcess := !ok
	if shouldProcess {
		point := crypto.Ed25519Curve.Point()
		point.UnmarshalBinary(commitment)
		consensus.commitments[validatorId] = point
		// Set the bitmap indicate this validate signed. TODO: figure out how to resolve the inconsistency of validators from commit and response messages
		consensus.bitmap.SetKey(value.PubKey, true)
	}

	if !shouldProcess {
		return
	}

	if len(consensus.commitments) >= (2*(len(consensus.validators)+1))/3+1 && consensus.state < CHALLENGE_DONE {
		consensus.Log.Debug("Enough commitments received with signatures", "numOfSignatures", len(consensus.commitments))

		// Broadcast challenge
		msgToSend := consensus.constructChallengeMessage()
		p2p.BroadcastMessage(consensus.getValidatorPeers(), msgToSend)

		// Set state to CHALLENGE_DONE
		consensus.state = CHALLENGE_DONE
	}
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
	validatorId := binary.BigEndian.Uint16(payload[offset : offset+2])
	offset += 2

	// 32 byte response
	response := payload[offset : offset+32]
	offset += 32

	// 64 byte of signature on previous data
	signature := payload[offset : offset+64]
	offset += 64
	//#### END: Read payload data

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

	// Verify signature
	value, ok := consensus.validators[validatorId]
	if !ok {
		consensus.Log.Warn("Received message from unrecognized validator", "validatorId", validatorId, "consensus", consensus)
		return
	}
	if schnorr.Verify(crypto.Ed25519Curve, value.PubKey, payload[:offset-64], signature) != nil {
		consensus.Log.Warn("Received message with invalid signature", "validatorKey", consensus.leader.PubKey, "consensus", consensus)
		return
	}

	// proceed only when the message is not received before
	_, ok = consensus.responses[validatorId]
	shouldProcess = shouldProcess && !ok
	if shouldProcess {
		scalar := crypto.Ed25519Curve.Scalar()
		scalar.UnmarshalBinary(response)
		consensus.responses[validatorId] = scalar
		// Set the bitmap indicate this validate signed. TODO: figure out how to resolve the inconsistency of validators from commit and response messages
		consensus.bitmap.SetKey(value.PubKey, true)
	}
	consensus.mutex.Unlock()

	if !shouldProcess {
		return
	}

	//consensus.Log.Debug("RECEIVED RESPONSE", "consensusId", consensusId)
	if len(consensus.responses) >= (2*len(consensus.validators))/3+1 && consensus.state != FINISHED {
		consensus.mutex.Lock()
		if len(consensus.responses) >= (2*len(consensus.validators))/3+1 && consensus.state != FINISHED {
			// Aggregate responses
			responses := make([]kyber.Scalar, 0)
			for _, val := range consensus.responses {
				responses = append(responses, val)
			}
			aggResponse, err := crypto.AggregateResponses(crypto.Ed25519Curve, responses)
			if err != nil {
				log.Error("Failed to aggregate responses")
				return
			}
			collectiveSig, err := crypto.Sign(crypto.Ed25519Curve, consensus.aggregatedCommitment, aggResponse, consensus.bitmap)

			if err != nil {
				log.Error("Failed to create collective signature")
				return
			} else {
				log.Info("CollectiveSig created.", "size", len(collectiveSig))
			}

			consensus.Log.Debug("Consensus reached with signatures.", "numOfSignatures", len(consensus.responses))
			// Reset state to FINISHED, and clear other data.
			consensus.ResetState()
			consensus.consensusId++
			consensus.Log.Debug("HOORAY!!! CONSENSUS REACHED!!!", "consensusId", consensus.consensusId)

			// TODO: reconstruct the whole block from header and transactions
			// For now, we used the stored whole block already stored in consensus.blockHeader
			txDecoder := gob.NewDecoder(bytes.NewReader(consensus.blockHeader))
			var blockHeaderObj blockchain.Block
			err = txDecoder.Decode(&blockHeaderObj)
			if err != nil {
				consensus.Log.Debug("failed to construct the new block after consensus")
			}

			// Sign the block
			// TODO(RJ): populate bitmap
			copy(blockHeaderObj.Signature[:], collectiveSig)
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
