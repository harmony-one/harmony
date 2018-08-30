package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"net/http"
	"net/url"
	"strconv"
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
	case proto_consensus.START_CONSENSUS:
		consensus.processStartConsensusMessage(payload)
	case proto_consensus.COMMIT:
		consensus.processCommitMessage(payload, CHALLENGE_DONE)
	case proto_consensus.RESPONSE:
		consensus.processResponseMessage(payload, COLLECTIVE_SIG_DONE)
	case proto_consensus.FINAL_COMMIT:
		consensus.processCommitMessage(payload, FINAL_CHALLENGE_DONE)
	case proto_consensus.FINAL_RESPONSE:
		consensus.processResponseMessage(payload, FINISHED)
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
	consensus.commitByLeader(true)
}

// Leader commit to the message itself before receiving others commits
func (consensus *Consensus) commitByLeader(firstRound bool) {
	// Generate leader's own commitment
	secret, commitment := crypto.Commit(crypto.Ed25519Curve)
	consensus.secret = secret
	if firstRound {
		(*consensus.commitments)[consensus.nodeId] = commitment
		consensus.bitmap.SetKey(consensus.pubKey, true)
	} else {
		(*consensus.finalCommitments)[consensus.nodeId] = commitment
		consensus.finalBitmap.SetKey(consensus.pubKey, true)
	}
}

// Processes the commit message sent from validators
func (consensus *Consensus) processCommitMessage(payload []byte, targetState ConsensusState) {
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

	commitments := consensus.commitments // targetState == CHALLENGE_DONE
	bitmap := consensus.bitmap
	if targetState == FINAL_CHALLENGE_DONE {
		commitments = consensus.finalCommitments
		bitmap = consensus.finalBitmap
	}

	// proceed only when the message is not received before
	_, ok = (*commitments)[validatorId]
	shouldProcess := !ok
	if shouldProcess {
		point := crypto.Ed25519Curve.Point()
		point.UnmarshalBinary(commitment)
		(*commitments)[validatorId] = point
		// Set the bitmap indicate this validate signed. TODO: figure out how to resolve the inconsistency of validators from commit and response messages
		bitmap.SetKey(value.PubKey, true)
	}

	if !shouldProcess {
		return
	}

	if len((*commitments)) >= len(consensus.publicKeys) && consensus.state < targetState {
		consensus.Log.Debug("Enough commitments received with signatures", "num", len((*commitments)), "state", consensus.state)

		// Broadcast challenge
		msgTypeToSend := proto_consensus.CHALLENGE // targetState == CHALLENGE_DONE
		if targetState == FINAL_CHALLENGE_DONE {
			msgTypeToSend = proto_consensus.FINAL_CHALLENGE
		}
		msgToSend, challengeScalar, aggCommitment := consensus.constructChallengeMessage(msgTypeToSend)
		bytes, err := challengeScalar.MarshalBinary()
		if err != nil {
			log.Error("Failed to serialize challenge")
		}

		if msgTypeToSend == proto_consensus.CHALLENGE {
			copy(consensus.challenge[:], bytes)
			consensus.aggregatedCommitment = aggCommitment
		} else if msgTypeToSend == proto_consensus.FINAL_CHALLENGE {
			copy(consensus.finalChallenge[:], bytes)
			consensus.aggregatedFinalCommitment = aggCommitment
		}

		// Add leader's response
		consensus.responseByLeader(challengeScalar, targetState == CHALLENGE_DONE)

		// Broadcast challenge message
		p2p.BroadcastMessage(consensus.getValidatorPeers(), msgToSend)

		// Set state to targetState (CHALLENGE_DONE or FINAL_CHALLENGE_DONE)
		consensus.state = targetState
	}
}

// Leader commit to the message itself before receiving others commits
func (consensus *Consensus) responseByLeader(challenge kyber.Scalar, firstRound bool) {
	// Generate leader's own commitment
	response, err := crypto.Response(crypto.Ed25519Curve, consensus.priKey, consensus.secret, challenge)
	if err == nil {
		if firstRound {
			(*consensus.responses)[consensus.nodeId] = response
			consensus.bitmap.SetKey(consensus.pubKey, true)
		} else {
			(*consensus.finalResponses)[consensus.nodeId] = response
			consensus.finalBitmap.SetKey(consensus.pubKey, true)
		}
	} else {
		log.Warn("Failed to generate response", "err", err)
	}
}

// Processes the response message sent from validators
func (consensus *Consensus) processResponseMessage(payload []byte, targetState ConsensusState) {
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

	commitments := consensus.commitments // targetState == COLLECTIVE_SIG_DONE
	responses := consensus.responses
	bitmap := consensus.bitmap
	if targetState == FINISHED {
		commitments = consensus.finalCommitments
		responses = consensus.finalResponses
		bitmap = consensus.finalBitmap
	}

	// proceed only when the message is not received before
	_, ok = (*responses)[validatorId]
	shouldProcess = shouldProcess && !ok
	if shouldProcess {
		// verify the response matches the received commit
		responseScalar := crypto.Ed25519Curve.Scalar()
		responseScalar.UnmarshalBinary(response)
		err := consensus.verifyResponse(commitments, responseScalar, validatorId)
		if err != nil {
			consensus.Log.Warn("Failed to verify the response", "error", err)
			shouldProcess = false
		} else {
			(*responses)[validatorId] = responseScalar
			// Set the bitmap indicate this validate signed. TODO: figure out how to resolve the inconsistency of validators from commit and response messages
			consensus.bitmap.SetKey(value.PubKey, true)
		}

	}
	consensus.mutex.Unlock()

	if !shouldProcess {
		return
	}

	if len(*responses) >= len(consensus.publicKeys) && consensus.state != targetState {
		consensus.mutex.Lock()
		if len(*responses) >= len(consensus.publicKeys) && consensus.state != targetState {
			consensus.Log.Debug("Enough responses received with signatures", "num", len(*responses), "state", consensus.state)
			// Aggregate responses
			responseScalars := []kyber.Scalar{}
			for _, val := range *responses {
				responseScalars = append(responseScalars, val)
			}

			aggregatedResponse, err := crypto.AggregateResponses(crypto.Ed25519Curve, responseScalars)
			if err != nil {
				log.Error("Failed to aggregate responses")
				return
			}
			aggregatedCommitment := consensus.aggregatedCommitment
			if targetState == FINISHED {
				aggregatedCommitment = consensus.aggregatedFinalCommitment
			}
			collectiveSigAndBitmap, err := crypto.Sign(crypto.Ed25519Curve, aggregatedCommitment, aggregatedResponse, bitmap)

			if err != nil {
				log.Error("Failed to create collective signature")
				return
			} else {
				log.Info("CollectiveSig and Bitmap created.", "size", len(collectiveSigAndBitmap))
			}

			collectiveSig := [64]byte{}
			copy(collectiveSig[:], collectiveSigAndBitmap[:64])
			bitmap := collectiveSigAndBitmap[64:]

			// Set state to COLLECTIVE_SIG_DONE or FINISHED
			consensus.state = targetState

			if consensus.state != FINISHED {
				// Start the second round of Cosi
				msgToSend := consensus.constructCollectiveSigMessage(collectiveSig, bitmap)

				p2p.BroadcastMessage(consensus.getValidatorPeers(), msgToSend)
				consensus.commitByLeader(false)
			} else {
				consensus.Log.Debug("Consensus reached with signatures.", "numOfSignatures", len(*responses))
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
				copy(blockHeaderObj.Signature[:], collectiveSig[:])
				copy(blockHeaderObj.Bitmap[:], bitmap)
				consensus.OnConsensusDone(&blockHeaderObj)

				consensus.reportTPS(blockHeaderObj.NumTransactions)
				// Send signal to Node so the new block can be added and new round of consensus can be triggered
				consensus.ReadySignal <- 1
			}
		}
		consensus.mutex.Unlock()
	}
}

func (consensus *Consensus) verifyResponse(commitments *map[uint16]kyber.Point, response kyber.Scalar, validatorId uint16) error {
	if response.Equal(crypto.Ed25519Curve.Scalar()) {
		return errors.New("response is zero valued")
	}
	_, ok := (*commitments)[validatorId]
	if !ok {
		return errors.New("no commit is received for the validator")
	}
	// TODO(RJ): enable the actual check
	//challenge := crypto.Ed25519Curve.Scalar()
	//challenge.UnmarshalBinary(consensus.challenge[:])
	//
	//// compute Q = sG + r*pubKey
	//sG := crypto.Ed25519Curve.Point().Mul(response, nil)
	//r_pubKey := crypto.Ed25519Curve.Point().Mul(challenge, consensus.validators[validatorId].PubKey)
	//Q := crypto.Ed25519Curve.Point().Add(sG, r_pubKey)
	//
	//if !Q.Equal(commit) {
	//	return errors.New("recreated commit doesn't match the received one")
	//}
	return nil
}

func (consensus *Consensus) reportTPS(numOfTxs int32) {
	endTime := time.Now()
	timeElapsed := endTime.Sub(startTime)
	tps := float64(numOfTxs) / timeElapsed.Seconds()
	consensus.Log.Info("TPS Report",
		"numOfTXs", numOfTxs,
		"startTime", startTime,
		"endTime", endTime,
		"timeElapsed", timeElapsed,
		"TPS", tps,
		"consensus", consensus)
	reportMetrics(tps)
}

func reportMetrics(tps float64) {
	URL := "http://localhost:3000/report"
	form := url.Values{
		"tps": {strconv.FormatFloat(tps, 'f', 2, 64)},
	}

	body := bytes.NewBufferString(form.Encode())
	rsp, err := http.Post(URL, "application/x-www-form-urlencoded", body)
	if err != nil {
		return
	}
	defer rsp.Body.Close()
}
