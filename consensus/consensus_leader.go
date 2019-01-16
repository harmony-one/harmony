package consensus

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/ethereum/go-ethereum/log"
	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/api/services/explorer"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/internal/profiler"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

const (
	waitForEnoughValidators = 1000
)

var (
	startTime time.Time
)

// WaitForNewBlock waits for the next new block to run consensus on
func (consensus *Consensus) WaitForNewBlock(blockChannel chan *types.Block) {
	consensus.Log.Debug("Waiting for block", "consensus", consensus)
	for { // keep waiting for new blocks
		newBlock := <-blockChannel
		// TODO: think about potential race condition

		c := consensus.RemovePeers(consensus.OfflinePeerList)
		if c > 0 {
			consensus.Log.Debug("WaitForNewBlock", "removed peers", c)
		}

		for !consensus.HasEnoughValidators() {
			consensus.Log.Debug("Not enough validators", "# Validators", len(consensus.PublicKeys))
			time.Sleep(waitForEnoughValidators * time.Millisecond)
		}

		startTime = time.Now()
		consensus.Log.Debug("STARTING CONSENSUS", "numTxs", len(newBlock.Transactions()), "consensus", consensus, "startTime", startTime, "publicKeys", len(consensus.PublicKeys))
		for consensus.state == Finished {
			// time.Sleep(500 * time.Millisecond)
			consensus.ResetState()
			consensus.startConsensus(newBlock)
			break
		}
	}
}

// ProcessMessageLeader dispatches consensus message for the leader.
func (consensus *Consensus) ProcessMessageLeader(payload []byte) {
	message := consensus_proto.Message{}
	err := message.XXX_Unmarshal(payload)

	if err != nil {
		consensus.Log.Error("Failed to unmarshal message payload.", "err", err, "consensus", consensus)
	}

	switch message.Type {
	case consensus_proto.MessageType_COMMIT:
		consensus.processCommitMessage(message, ChallengeDone)
	case consensus_proto.MessageType_RESPONSE:
		consensus.processResponseMessage(message, CollectiveSigDone)
	case consensus_proto.MessageType_FINAL_COMMIT:
		consensus.processCommitMessage(message, FinalChallengeDone)
	case consensus_proto.MessageType_FINAL_RESPONSE:
		consensus.processResponseMessage(message, Finished)
	default:
		consensus.Log.Error("Unexpected message type", "msgType", message.Type, "consensus", consensus)
	}
}

// startConsensus starts a new consensus for a block by broadcast a announce message to the validators
func (consensus *Consensus) startConsensus(newBlock *types.Block) {
	// Copy over block hash and block header data
	blockHash := newBlock.Hash()
	copy(consensus.blockHash[:], blockHash[:])

	consensus.Log.Debug("Start encoding block")
	// prepare message and broadcast to validators
	encodedBlock, err := rlp.EncodeToBytes(newBlock)
	if err != nil {
		consensus.Log.Debug("Failed encoding block")
		return
	}
	consensus.block = encodedBlock

	consensus.Log.Debug("Stop encoding block")
	msgToSend := consensus.constructAnnounceMessage()

	// Set state to AnnounceDone
	consensus.state = AnnounceDone
	consensus.commitByLeader(true)
	host.BroadcastMessageFromLeader(consensus.host, consensus.GetValidatorPeers(), msgToSend, consensus.OfflinePeers)
}

// commitByLeader commits to the message by leader himself before receiving others commits
func (consensus *Consensus) commitByLeader(firstRound bool) {
	// Generate leader's own commitment
	secret, commitment := crypto.Commit(crypto.Ed25519Curve)
	consensus.secret[consensus.consensusID] = secret
	if firstRound {
		consensus.mutex.Lock()
		defer consensus.mutex.Unlock()
		(*consensus.commitments)[consensus.nodeID] = commitment
		consensus.bitmap.SetKey(consensus.pubKey, true)
	} else {
		(*consensus.finalCommitments)[consensus.nodeID] = commitment
		consensus.finalBitmap.SetKey(consensus.pubKey, true)
	}
}

// processCommitMessage processes the commit message sent from validators
func (consensus *Consensus) processCommitMessage(message consensus_proto.Message, targetState State) {
	consensusID := message.ConsensusId
	blockHash := message.BlockHash
	validatorID := message.SenderId
	commitment := message.Payload
	signature := message.Signature

	// Verify signature
	v, ok := consensus.validators.Load(validatorID)
	if !ok {
		consensus.Log.Warn("Received message from unrecognized validator", "validatorID", validatorID, "consensus", consensus)
		return
	}
	value, ok := v.(p2p.Peer)
	if !ok {
		consensus.Log.Warn("Invalid validator", "validatorID", validatorID, "consensus", consensus)
		return
	}

	message.Signature = nil
	messageBytes, err := message.XXX_Marshal([]byte{}, true)
	if err != nil {
		consensus.Log.Warn("Failed to marshal the announce message", "error", err)
	}
	if schnorr.Verify(crypto.Ed25519Curve, value.PubKey, messageBytes, signature) != nil {
		consensus.Log.Warn("Received message with invalid signature", "validatorKey", consensus.leader.PubKey, "consensus", consensus)
		return
	}

	// check consensus Id
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	if consensusID != consensus.consensusID {
		consensus.Log.Warn("Received Commit with wrong consensus Id", "myConsensusId", consensus.consensusID, "theirConsensusId", consensusID, "consensus", consensus)
		return
	}

	if !bytes.Equal(blockHash, consensus.blockHash[:]) {
		consensus.Log.Warn("Received Commit with wrong blockHash", "myConsensusId", consensus.consensusID, "theirConsensusId", consensusID, "consensus", consensus)
		return
	}

	commitments := consensus.commitments // targetState == ChallengeDone
	bitmap := consensus.bitmap
	if targetState == FinalChallengeDone {
		commitments = consensus.finalCommitments
		bitmap = consensus.finalBitmap
	}

	// proceed only when the message is not received before
	_, ok = (*commitments)[validatorID]
	shouldProcess := !ok
	if len((*commitments)) >= ((len(consensus.PublicKeys)*2)/3 + 1) {
		shouldProcess = false
	}
	if shouldProcess {
		point := crypto.Ed25519Curve.Point()
		point.UnmarshalBinary(commitment)
		(*commitments)[validatorID] = point
		consensus.Log.Debug("Received new commit message", "num", len(*commitments), "validatorID", validatorID, "PublicKeys", len(consensus.PublicKeys))
		// Set the bitmap indicate this validate signed.
		bitmap.SetKey(value.PubKey, true)
	}

	if !shouldProcess {
		consensus.Log.Debug("Received additional new commit message", "validatorID", validatorID)
		return
	}

	if len((*commitments)) >= ((len(consensus.PublicKeys)*2)/3+1) && consensus.state < targetState {
		consensus.Log.Debug("Enough commitments received with signatures", "num", len(*commitments), "state", consensus.state)

		// Broadcast challenge
		msgTypeToSend := consensus_proto.MessageType_CHALLENGE // targetState == ChallengeDone
		if targetState == FinalChallengeDone {
			msgTypeToSend = consensus_proto.MessageType_FINAL_CHALLENGE
		}

		msgToSend, challengeScalar, aggCommitment := consensus.constructChallengeMessage(msgTypeToSend)
		bytes, err := challengeScalar.MarshalBinary()
		if err != nil {
			log.Error("Failed to serialize challenge")
		}

		if msgTypeToSend == consensus_proto.MessageType_CHALLENGE {
			copy(consensus.challenge[:], bytes)
			consensus.aggregatedCommitment = aggCommitment
		} else if msgTypeToSend == consensus_proto.MessageType_FINAL_CHALLENGE {
			copy(consensus.finalChallenge[:], bytes)
			consensus.aggregatedFinalCommitment = aggCommitment
		}

		// Add leader's response
		consensus.responseByLeader(challengeScalar, targetState == ChallengeDone)

		// Broadcast challenge message
		host.BroadcastMessageFromLeader(consensus.host, consensus.GetValidatorPeers(), msgToSend, consensus.OfflinePeers)

		// Set state to targetState (ChallengeDone or FinalChallengeDone)
		consensus.state = targetState
	}
}

// Leader commit to the message itself before receiving others commits
func (consensus *Consensus) responseByLeader(challenge kyber.Scalar, firstRound bool) {
	// Generate leader's own commitment
	response, err := crypto.Response(crypto.Ed25519Curve, consensus.priKey, consensus.secret[consensus.consensusID], challenge)
	if err == nil {
		if firstRound {
			(*consensus.responses)[consensus.nodeID] = response
			consensus.bitmap.SetKey(consensus.pubKey, true)
		} else {
			(*consensus.finalResponses)[consensus.nodeID] = response
			consensus.finalBitmap.SetKey(consensus.pubKey, true)
		}
	} else {
		log.Warn("leader failed to generate response", "err", err)
	}
}

// Processes the response message sent from validators
func (consensus *Consensus) processResponseMessage(message consensus_proto.Message, targetState State) {
	consensusID := message.ConsensusId
	blockHash := message.BlockHash
	validatorID := message.SenderId
	response := message.Payload
	signature := message.Signature

	shouldProcess := true
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	// check consensus Id
	if consensusID != consensus.consensusID {
		shouldProcess = false
		consensus.Log.Warn("Received Response with wrong consensus Id", "myConsensusId", consensus.consensusID, "theirConsensusId", consensusID, "consensus", consensus)
	}

	if !bytes.Equal(blockHash, consensus.blockHash[:]) {
		consensus.Log.Warn("Received Response with wrong blockHash", "myConsensusId", consensus.consensusID, "theirConsensusId", consensusID, "consensus", consensus)
		return
	}

	// Verify signature
	v, ok := consensus.validators.Load(validatorID)
	if !ok {
		consensus.Log.Warn("Received message from unrecognized validator", "validatorID", validatorID, "consensus", consensus)
		return
	}
	value, ok := v.(p2p.Peer)
	if !ok {
		consensus.Log.Warn("Invalid validator", "validatorID", validatorID, "consensus", consensus)
		return
	}
	message.Signature = nil
	messageBytes, err := message.XXX_Marshal([]byte{}, true)
	if err != nil {
		consensus.Log.Warn("Failed to marshal the announce message", "error", err)
	}
	if schnorr.Verify(crypto.Ed25519Curve, value.PubKey, messageBytes, signature) != nil {
		consensus.Log.Warn("Received message with invalid signature", "validatorKey", consensus.leader.PubKey, "consensus", consensus)
		return
	}

	commitments := consensus.commitments // targetState == CollectiveSigDone
	responses := consensus.responses
	bitmap := consensus.bitmap
	if targetState == Finished {
		commitments = consensus.finalCommitments
		responses = consensus.finalResponses
		bitmap = consensus.finalBitmap
	}

	// proceed only when the message is not received before
	_, ok = (*responses)[validatorID]
	shouldProcess = shouldProcess && !ok

	if len((*responses)) >= ((len(consensus.PublicKeys)*2)/3 + 1) {
		shouldProcess = false
	}

	if shouldProcess {
		// verify the response matches the received commit
		responseScalar := crypto.Ed25519Curve.Scalar()
		responseScalar.UnmarshalBinary(response)
		err := consensus.verifyResponse(commitments, responseScalar, validatorID)
		if err != nil {
			consensus.Log.Warn("leader failed to verify the response", "error", err, "VID", strconv.Itoa(int(validatorID)))
			shouldProcess = false
		} else {
			(*responses)[validatorID] = responseScalar
			consensus.Log.Debug("Received new response message", "num", len(*responses), "validatorID", strconv.Itoa(int(validatorID)))
			// Set the bitmap indicate this validate signed.
			bitmap.SetKey(value.PubKey, true)
		}
	}

	if !shouldProcess {
		consensus.Log.Debug("Received new response message", "validatorID", strconv.Itoa(int(validatorID)))
		return
	}

	threshold := 2
	if targetState == Finished {
		threshold = 1
	}
	if len(*responses) >= ((len(consensus.PublicKeys)*threshold)/3+1) && consensus.state != targetState {
		if len(*responses) >= ((len(consensus.PublicKeys)*threshold)/3+1) && consensus.state != targetState {
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
			if targetState == Finished {
				aggregatedCommitment = consensus.aggregatedFinalCommitment
			}

			collectiveSigAndBitmap, err := crypto.Sign(crypto.Ed25519Curve, aggregatedCommitment, aggregatedResponse, bitmap)

			if err != nil {
				log.Error("Failed to create collective signature")
				return
			}
			log.Info("CollectiveSig and Bitmap created.", "size", len(collectiveSigAndBitmap))

			collectiveSig := [64]byte{}
			copy(collectiveSig[:], collectiveSigAndBitmap[:64])
			bitmap := collectiveSigAndBitmap[64:]

			// Set state to CollectiveSigDone or Finished
			consensus.state = targetState

			if consensus.state != Finished {
				// Start the second round of Cosi
				msgToSend := consensus.constructCollectiveSigMessage(collectiveSig, bitmap)

				host.BroadcastMessageFromLeader(consensus.host, consensus.GetValidatorPeers(), msgToSend, consensus.OfflinePeers)
				consensus.commitByLeader(false)
			} else {
				var blockObj types.Block
				err = rlp.DecodeBytes(consensus.block, &blockObj)
				if err != nil {
					consensus.Log.Debug("failed to construct the new block after consensus")
				}

				// Sign the block
				copy(blockObj.Header().Signature[:], collectiveSig[:])
				copy(blockObj.Header().Bitmap[:], bitmap)
				consensus.OnConsensusDone(&blockObj)

				select {
				case consensus.VerifiedNewBlock <- &blockObj:
				default:
					consensus.Log.Info("[sync] consensus verified block send to chan failed", "blockHash", blockObj.Hash())
				}

				consensus.reportMetrics(blockObj)

				// Dump new block into level db.
				explorer.GetStorageInstance(consensus.leader.IP, consensus.leader.Port, true).Dump(&blockObj, consensus.consensusID)

				// Reset state to Finished, and clear other data.
				consensus.ResetState()
				consensus.consensusID++

				consensus.Log.Debug("HOORAY!!! CONSENSUS REACHED!!!", "consensusID", consensus.consensusID, "numOfSignatures", len(*responses))

				// TODO: remove this temporary delay
				time.Sleep(500 * time.Millisecond)
				// Send signal to Node so the new block can be added and new round of consensus can be triggered
				consensus.ReadySignal <- struct{}{}
			}
		}
	}
}

func (consensus *Consensus) verifyResponse(commitments *map[uint32]kyber.Point, response kyber.Scalar, validatorID uint32) error {
	if response.Equal(crypto.Ed25519Curve.Scalar()) {
		return errors.New("response is zero valued")
	}
	_, ok := (*commitments)[validatorID]
	if !ok {
		return errors.New("no commit is received for the validator")
	}
	return nil
}

func (consensus *Consensus) reportMetrics(block types.Block) {
	endTime := time.Now()
	timeElapsed := endTime.Sub(startTime)
	numOfTxs := len(block.Transactions())
	tps := float64(numOfTxs) / timeElapsed.Seconds()
	consensus.Log.Info("TPS Report",
		"numOfTXs", numOfTxs,
		"startTime", startTime,
		"endTime", endTime,
		"timeElapsed", timeElapsed,
		"TPS", tps,
		"consensus", consensus)

	// Post metrics
	profiler := profiler.GetProfiler()
	if profiler.MetricsReportURL == "" {
		return
	}

	txHashes := []string{}
	for i, end := 0, len(block.Transactions()); i < 3 && i < end; i++ {
		txHash := block.Transactions()[end-1-i].Hash()
		txHashes = append(txHashes, hex.EncodeToString(txHash[:]))
	}
	metrics := map[string]interface{}{
		"key":             consensus.pubKey.String(),
		"tps":             tps,
		"txCount":         numOfTxs,
		"nodeCount":       len(consensus.PublicKeys) + 1,
		"latestBlockHash": hex.EncodeToString(consensus.blockHash[:]),
		"latestTxHashes":  txHashes,
		"blockLatency":    int(timeElapsed / time.Millisecond),
	}
	profiler.LogMetrics(metrics)
}

// HasEnoughValidators checks the number of publicKeys to determine
// if the shard has enough validators
// FIXME (HAR-82): we need epoch support or a better way to determine
// when to initiate the consensus
func (consensus *Consensus) HasEnoughValidators() bool {
	if len(consensus.PublicKeys) < consensus.MinPeers {
		return false
	}
	return true
}
