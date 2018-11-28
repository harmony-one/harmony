package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"

	"github.com/harmony-one/harmony/profiler"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2p"
	proto_consensus "github.com/harmony-one/harmony/proto/consensus"
)

var (
	startTime time.Time
)

// WaitForNewBlock waits for the next new block to run consensus on
func (consensus *Consensus) WaitForNewBlock(blockChannel chan blockchain.Block) {
	consensus.Log.Debug("Waiting for block", "consensus", consensus)
	for { // keep waiting for new blocks
		newBlock := <-blockChannel

		if !consensus.HasEnoughValidators() {
			consensus.Log.Debug("Not enough validators", "# Validators", len(consensus.publicKeys))
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// TODO: think about potential race condition
		startTime = time.Now()
		consensus.Log.Debug("STARTING CONSENSUS", "consensus", consensus, "startTime", startTime)
		for consensus.state == Finished {
			// time.Sleep(500 * time.Millisecond)
			consensus.startConsensus(&newBlock)
			break
		}
	}
}

// WaitForNewBlock waits for the next new block to run consensus on
func (consensus *Consensus) WaitForNewBlockAccount(blockChannel chan *types.Block) {
	consensus.Log.Debug("Waiting for block", "consensus", consensus)
	for { // keep waiting for new blocks
		newBlock := <-blockChannel
		// TODO: think about potential race condition
		startTime = time.Now()
		consensus.Log.Debug("STARTING CONSENSUS", "consensus", consensus, "startTime", startTime)
		for consensus.state == Finished {
			// time.Sleep(500 * time.Millisecond)
			data, err := rlp.EncodeToBytes(newBlock)
			if err == nil {
				consensus.Log.Debug("Sample tx", "tx", newBlock.Transactions()[0])
				consensus.startConsensus(&blockchain.Block{Hash: newBlock.Hash(), AccountBlock: data})
			} else {
				consensus.Log.Error("Failed encoding the block with RLP")
			}
			break
		}
	}
}

// ProcessMessageLeader dispatches consensus message for the leader.
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
	case proto_consensus.StartConsensus:
		consensus.processStartConsensusMessage(payload)
	case proto_consensus.Commit:
		consensus.processCommitMessage(payload, ChallengeDone)
	case proto_consensus.Response:
		consensus.processResponseMessage(payload, CollectiveSigDone)
	case proto_consensus.FinalCommit:
		consensus.processCommitMessage(payload, FinalChallengeDone)
	case proto_consensus.FinalResponse:
		consensus.processResponseMessage(payload, Finished)
	default:
		consensus.Log.Error("Unexpected message type", "msgType", msgType, "consensus", consensus)
	}
}

// processStartConsensusMessage is the handler for message which triggers consensus process.
func (consensus *Consensus) processStartConsensusMessage(payload []byte) {
	// TODO: remove these method after testnet
	tx := blockchain.NewCoinbaseTX([20]byte{0}, "y", 0)
	consensus.startConsensus(blockchain.NewGenesisBlock(tx, 0))
}

// startConsensus starts a new consensus for a block by broadcast a announce message to the validators
func (consensus *Consensus) startConsensus(newBlock *blockchain.Block) {
	// Copy over block hash and block header data
	copy(consensus.blockHash[:], newBlock.Hash[:])

	consensus.Log.Debug("Start encoding block")
	// prepare message and broadcast to validators
	byteBuffer := bytes.NewBuffer([]byte{})
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(newBlock)
	consensus.blockHeader = byteBuffer.Bytes()

	consensus.Log.Debug("Stop encoding block")
	msgToSend := consensus.constructAnnounceMessage()
	p2p.BroadcastMessageFromLeader(consensus.GetValidatorPeers(), msgToSend)
	// Set state to AnnounceDone
	consensus.state = AnnounceDone
	consensus.commitByLeader(true)
}

// commitByLeader commits to the message itself before receiving others commits
func (consensus *Consensus) commitByLeader(firstRound bool) {
	// Generate leader's own commitment
	secret, commitment := crypto.Commit(crypto.Ed25519Curve)
	consensus.secret[consensus.consensusID] = secret
	if firstRound {
		(*consensus.commitments)[consensus.nodeID] = commitment
		consensus.bitmap.SetKey(consensus.pubKey, true)
	} else {
		(*consensus.finalCommitments)[consensus.nodeID] = commitment
		consensus.finalBitmap.SetKey(consensus.pubKey, true)
	}
}

// processCommitMessage processes the commit message sent from validators
func (consensus *Consensus) processCommitMessage(payload []byte, targetState State) {
	// Read payload data
	offset := 0
	// 4 byte consensus id
	consensusID := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4

	// 32 byte block hash
	blockHash := payload[offset : offset+32]
	offset += 32

	// 2 byte validator id
	validatorID := binary.BigEndian.Uint16(payload[offset : offset+2])
	offset += 2

	// 32 byte commit
	commitment := payload[offset : offset+32]
	offset += 32

	// 64 byte of signature on all above data
	signature := payload[offset : offset+64]
	offset += 64

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

	if schnorr.Verify(crypto.Ed25519Curve, value.PubKey, payload[:offset-64], signature) != nil {
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
	if len((*commitments)) >= ((len(consensus.publicKeys)*2)/3 + 1) {
		shouldProcess = false
	}
	if shouldProcess {
		point := crypto.Ed25519Curve.Point()
		point.UnmarshalBinary(commitment)
		(*commitments)[validatorID] = point
		consensus.Log.Debug("Received new commit message", "num", len(*commitments), "validatorID", validatorID)
		// Set the bitmap indicate this validate signed. TODO: figure out how to resolve the inconsistency of validators from commit and response messages
		bitmap.SetKey(value.PubKey, true)
	}

	if !shouldProcess {
		consensus.Log.Debug("Received new commit message", "validatorID", validatorID)
		return
	}

	if len((*commitments)) >= ((len(consensus.publicKeys)*2)/3+1) && consensus.state < targetState {
		consensus.Log.Debug("Enough commitments received with signatures", "num", len(*commitments), "state", consensus.state)

		// Broadcast challenge
		msgTypeToSend := proto_consensus.Challenge // targetState == ChallengeDone
		if targetState == FinalChallengeDone {
			msgTypeToSend = proto_consensus.FinalChallenge
		}
		msgToSend, challengeScalar, aggCommitment := consensus.constructChallengeMessage(msgTypeToSend)
		bytes, err := challengeScalar.MarshalBinary()
		if err != nil {
			log.Error("Failed to serialize challenge")
		}

		if msgTypeToSend == proto_consensus.Challenge {
			copy(consensus.challenge[:], bytes)
			consensus.aggregatedCommitment = aggCommitment
		} else if msgTypeToSend == proto_consensus.FinalChallenge {
			copy(consensus.finalChallenge[:], bytes)
			consensus.aggregatedFinalCommitment = aggCommitment
		}

		// Add leader's response
		consensus.responseByLeader(challengeScalar, targetState == ChallengeDone)

		// Broadcast challenge message
		p2p.BroadcastMessageFromLeader(consensus.GetValidatorPeers(), msgToSend)

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
		log.Warn("Failed to generate response", "err", err)
	}
}

// Processes the response message sent from validators
func (consensus *Consensus) processResponseMessage(payload []byte, targetState State) {
	//#### Read payload data
	offset := 0
	// 4 byte consensus id
	consensusID := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4

	// 32 byte block hash
	blockHash := payload[offset : offset+32]
	offset += 32

	// 2 byte validator id
	validatorID := binary.BigEndian.Uint16(payload[offset : offset+2])
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

	if schnorr.Verify(crypto.Ed25519Curve, value.PubKey, payload[:offset-64], signature) != nil {
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

	if len((*responses)) >= ((len(consensus.publicKeys)*2)/3 + 1) {
		shouldProcess = false
	}

	if shouldProcess {
		// verify the response matches the received commit
		responseScalar := crypto.Ed25519Curve.Scalar()
		responseScalar.UnmarshalBinary(response)
		err := consensus.verifyResponse(commitments, responseScalar, validatorID)
		if err != nil {
			consensus.Log.Warn("Failed to verify the response", "error", err)
			shouldProcess = false
		} else {
			(*responses)[validatorID] = responseScalar
			consensus.Log.Debug("Received new response message", "num", len(*responses), "validatorID", validatorID)
			// Set the bitmap indicate this validate signed. TODO: figure out how to resolve the inconsistency of validators from commit and response messages
			bitmap.SetKey(value.PubKey, true)
		}
	}

	if !shouldProcess {
		consensus.Log.Debug("Received new response message", "validatorID", validatorID)
		return
	}

	if len(*responses) >= ((len(consensus.publicKeys)*2)/3+1) && consensus.state != targetState {
		if len(*responses) >= ((len(consensus.publicKeys)*2)/3+1) && consensus.state != targetState {
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

				p2p.BroadcastMessageFromLeader(consensus.GetValidatorPeers(), msgToSend)
				consensus.commitByLeader(false)
			} else {
				consensus.Log.Debug("Consensus reached with signatures.", "numOfSignatures", len(*responses))
				// Reset state to Finished, and clear other data.
				consensus.ResetState()
				consensus.consensusID++
				consensus.Log.Debug("HOORAY!!! CONSENSUS REACHED!!!", "consensusID", consensus.consensusID)

				// TODO: reconstruct the whole block from header and transactions
				// For now, we used the stored whole block already stored in consensus.blockHeader
				txDecoder := gob.NewDecoder(bytes.NewReader(consensus.blockHeader))
				var blockHeaderObj blockchain.Block
				err = txDecoder.Decode(&blockHeaderObj)
				if err != nil {
					consensus.Log.Debug("failed to construct the new block after consensus")
				}

				// Sign the block
				copy(blockHeaderObj.Signature[:], collectiveSig[:])
				copy(blockHeaderObj.Bitmap[:], bitmap)
				consensus.OnConsensusDone(&blockHeaderObj)

				consensus.reportMetrics(blockHeaderObj)
				// Send signal to Node so the new block can be added and new round of consensus can be triggered
				consensus.ReadySignal <- struct{}{}
			}
		}
	}
}

func (consensus *Consensus) verifyResponse(commitments *map[uint16]kyber.Point, response kyber.Scalar, validatorID uint16) error {
	if response.Equal(crypto.Ed25519Curve.Scalar()) {
		return errors.New("response is zero valued")
	}
	_, ok := (*commitments)[validatorID]
	if !ok {
		return errors.New("no commit is received for the validator")
	}
	// TODO(RJ): enable the actual check
	//challenge := crypto.Ed25519Curve.Scalar()
	//challenge.UnmarshalBinary(consensus.challenge[:])
	//
	//// compute Q = sG + r*pubKey
	//sG := crypto.Ed25519Curve.Point().Mul(response, nil)
	//r_pubKey := crypto.Ed25519Curve.Point().Mul(challenge, consensus.validators[validatorID].PubKey)
	//Q := crypto.Ed25519Curve.Point().Add(sG, r_pubKey)
	//
	//if !Q.Equal(commit) {
	//	return errors.New("recreated commit doesn't match the received one")
	//}
	return nil
}

func (consensus *Consensus) reportMetrics(block blockchain.Block) {
	if block.IsStateBlock() { // Skip state block stats
		return
	}

	endTime := time.Now()
	timeElapsed := endTime.Sub(startTime)
	numOfTxs := block.NumTransactions
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
	for i, end := 0, len(block.TransactionIds); i < 3 && i < end; i++ {
		txHashes = append(txHashes, hex.EncodeToString(block.TransactionIds[end-1-i][:]))
	}
	metrics := map[string]interface{}{
		"key":             consensus.pubKey.String(),
		"tps":             tps,
		"txCount":         numOfTxs,
		"nodeCount":       len(consensus.publicKeys) + 1,
		"latestBlockHash": hex.EncodeToString(consensus.blockHash[:]),
		"latestTxHashes":  txHashes,
		"blockLatency":    int(timeElapsed / time.Millisecond),
	}
	profiler.LogMetrics(metrics)
}

func (consensus *Consensus) HasEnoughValidators() bool {
	if len(consensus.publicKeys) < consensus.MinPeers {
		return false
	}
	return true
}
