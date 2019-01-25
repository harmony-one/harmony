package consensus

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/api/services/explorer"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/profiler"
	"github.com/harmony-one/harmony/internal/utils"
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
	utils.GetLogInstance().Debug("Waiting for block", "consensus", consensus)
	for { // keep waiting for new blocks
		newBlock := <-blockChannel
		// TODO: think about potential race condition

		c := consensus.RemovePeers(consensus.OfflinePeerList)
		if c > 0 {
			utils.GetLogInstance().Debug("WaitForNewBlock", "removed peers", c)
		}

		for !consensus.HasEnoughValidators() {
			utils.GetLogInstance().Debug("Not enough validators", "# Validators", len(consensus.PublicKeys))
			time.Sleep(waitForEnoughValidators * time.Millisecond)
		}

		startTime = time.Now()
		utils.GetLogInstance().Debug("STARTING CONSENSUS", "numTxs", len(newBlock.Transactions()), "consensus", consensus, "startTime", startTime, "publicKeys", len(consensus.PublicKeys))
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
		utils.GetLogInstance().Error("Failed to unmarshal message payload.", "err", err, "consensus", consensus)
	}

	switch message.Type {
	case consensus_proto.MessageType_PREPARE:
		consensus.processPrepareMessage(message)
	case consensus_proto.MessageType_COMMIT:
		consensus.processCommitMessage(message)
	default:
		utils.GetLogInstance().Error("Unexpected message type", "msgType", message.Type, "consensus", consensus)
	}
}

// startConsensus starts a new consensus for a block by broadcast a announce message to the validators
func (consensus *Consensus) startConsensus(newBlock *types.Block) {
	// Copy over block hash and block header data
	blockHash := newBlock.Hash()
	copy(consensus.blockHash[:], blockHash[:])

	utils.GetLogInstance().Debug("Start encoding block")
	// prepare message and broadcast to validators
	encodedBlock, err := rlp.EncodeToBytes(newBlock)
	if err != nil {
		utils.GetLogInstance().Debug("Failed encoding block")
		return
	}
	consensus.block = encodedBlock

	utils.GetLogInstance().Debug("Stop encoding block")
	msgToSend := consensus.constructAnnounceMessage()

	// Set state to AnnounceDone
	consensus.state = AnnounceDone
	// TODO: sign for leader itself
	host.BroadcastMessageFromLeader(consensus.host, consensus.GetValidatorPeers(), msgToSend, consensus.OfflinePeers)
}

// processPrepareMessage processes the prepare message sent from validators
func (consensus *Consensus) processPrepareMessage(message consensus_proto.Message) {
	consensusID := message.ConsensusId
	blockHash := message.BlockHash
	validatorID := message.SenderId
	prepareSig := message.Payload
	signature := message.Signature

	// Verify signature
	v, ok := consensus.validators.Load(validatorID)
	if !ok {
		utils.GetLogInstance().Warn("Received message from unrecognized validator", "validatorID", validatorID, "consensus", consensus)
		return
	}
	value, ok := v.(p2p.Peer)
	if !ok {
		utils.GetLogInstance().Warn("Invalid validator", "validatorID", validatorID, "consensus", consensus)
		return
	}

	message.Signature = nil
	messageBytes, err := protobuf.Marshal(&message)
	if err != nil {
		utils.GetLogInstance().Warn("Failed to marshal the prepare message", "error", err)
	}
	_ = messageBytes
	_ = signature
	// TODO: verify message signature
	//if schnorr.Verify(crypto.Ed25519Curve, value.PubKey, messageBytes, signature) != nil {
	//	consensus.Log.Warn("Received message with invalid signature", "validatorKey", consensus.leader.PubKey, "consensus", consensus)
	//	return
	//}

	// check consensus Id
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	if consensusID != consensus.consensusID {
		utils.GetLogInstance().Warn("Received Commit with wrong consensus Id", "myConsensusId", consensus.consensusID, "theirConsensusId", consensusID, "consensus", consensus)
		return
	}

	if !bytes.Equal(blockHash, consensus.blockHash[:]) {
		utils.GetLogInstance().Warn("Received Commit with wrong blockHash", "myConsensusId", consensus.consensusID, "theirConsensusId", consensusID, "consensus", consensus)
		return
	}

	prepareSigs := consensus.prepareSigs
	prepareBitmap := consensus.prepareBitmap

	// proceed only when the message is not received before
	_, ok = (*prepareSigs)[validatorID]
	shouldProcess := !ok
	if len((*prepareSigs)) >= ((len(consensus.PublicKeys)*2)/3 + 1) {
		shouldProcess = false
	}

	if shouldProcess {
		var sign bls.Sign
		err := sign.Deserialize(prepareSig)
		if err != nil {
			utils.GetLogInstance().Error("Failed to deserialize bls signature", "validatorID", validatorID)
		}
		// TODO: check bls signature
		(*prepareSigs)[validatorID] = &sign
		utils.GetLogInstance().Debug("Received new prepare signature", "numReceivedSoFar", len(*prepareSigs), "validatorID", validatorID, "PublicKeys", len(consensus.PublicKeys))

		// Set the bitmap indicate this validate signed.
		prepareBitmap.SetKey(value.PubKey, true)
	}

	if !shouldProcess {
		utils.GetLogInstance().Debug("Received additional new commit message", "validatorID", validatorID)
		return
	}

	targetState := PreparedDone
	if len((*prepareSigs)) >= ((len(consensus.PublicKeys)*2)/3+1) && consensus.state < targetState {
		utils.GetLogInstance().Debug("Enough commitments received with signatures", "num", len(*prepareSigs), "state", consensus.state)

		// Construct prepared message
		msgToSend, aggSig := consensus.constructPreparedMessage()
		consensus.aggregatedPrepareSig = aggSig

		// Broadcast prepared message
		host.BroadcastMessageFromLeader(consensus.host, consensus.GetValidatorPeers(), msgToSend, consensus.OfflinePeers)

		// Set state to targetState (ChallengeDone or FinalChallengeDone)
		consensus.state = targetState
	}
}

// Processes the commit message sent from validators
func (consensus *Consensus) processCommitMessage(message consensus_proto.Message) {
	consensusID := message.ConsensusId
	blockHash := message.BlockHash
	validatorID := message.SenderId
	commitSig := message.Payload
	signature := message.Signature

	shouldProcess := true
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	// check consensus Id
	if consensusID != consensus.consensusID {
		shouldProcess = false
		utils.GetLogInstance().Warn("Received Response with wrong consensus Id", "myConsensusId", consensus.consensusID, "theirConsensusId", consensusID, "consensus", consensus)
	}

	if !bytes.Equal(blockHash, consensus.blockHash[:]) {
		utils.GetLogInstance().Warn("Received Response with wrong blockHash", "myConsensusId", consensus.consensusID, "theirConsensusId", consensusID, "consensus", consensus)
		return
	}

	// Verify signature
	v, ok := consensus.validators.Load(validatorID)
	if !ok {
		utils.GetLogInstance().Warn("Received message from unrecognized validator", "validatorID", validatorID, "consensus", consensus)
		return
	}
	value, ok := v.(p2p.Peer)
	if !ok {
		utils.GetLogInstance().Warn("Invalid validator", "validatorID", validatorID, "consensus", consensus)
		return
	}
	message.Signature = nil
	messageBytes, err := protobuf.Marshal(&message)
	if err != nil {
		utils.GetLogInstance().Warn("Failed to marshal the commit message", "error", err)
	}
	_ = messageBytes
	_ = signature
	// TODO: verify message signature
	//if schnorr.Verify(crypto.Ed25519Curve, value.PubKey, messageBytes, signature) != nil {
	//	consensus.Log.Warn("Received message with invalid signature", "validatorKey", consensus.leader.PubKey, "consensus", consensus)
	//	return
	//}

	commitSigs := consensus.commitSigs
	commitBitmap := consensus.commitBitmap

	// proceed only when the message is not received before
	_, ok = (*commitSigs)[validatorID]
	shouldProcess = shouldProcess && !ok

	if len((*commitSigs)) >= ((len(consensus.PublicKeys)*2)/3 + 1) {
		shouldProcess = false
	}

	if shouldProcess {
		var sign bls.Sign
		err := sign.Deserialize(commitSig)
		if err != nil {
			utils.GetLogInstance().Debug("Failed to deserialize bls signature", "validatorID", validatorID)
		}
		// TODO: check bls signature
		(*commitSigs)[validatorID] = &sign
		utils.GetLogInstance().Debug("Received new commit message", "numReceivedSoFar", len(*commitSigs), "validatorID", strconv.Itoa(int(validatorID)))
		// Set the bitmap indicate this validate signed.
		commitBitmap.SetKey(value.PubKey, true)
	}

	if !shouldProcess {
		utils.GetLogInstance().Debug("Received additional new commit message", "validatorID", strconv.Itoa(int(validatorID)))
		return
	}

	threshold := 2
	targetState := CommitDone
	if len(*commitSigs) >= ((len(consensus.PublicKeys)*threshold)/3+1) && consensus.state != targetState {
		utils.GetLogInstance().Info("Enough commits received!", "num", len(*commitSigs), "state", consensus.state)

		// Construct committed message
		msgToSend, aggSig := consensus.constructCommittedMessage()
		consensus.aggregatedPrepareSig = aggSig

		// Broadcast committed message
		host.BroadcastMessageFromLeader(consensus.host, consensus.GetValidatorPeers(), msgToSend, consensus.OfflinePeers)

		// Set state to CollectiveSigDone or Finished
		consensus.state = targetState

		var blockObj types.Block
		err = rlp.DecodeBytes(consensus.block, &blockObj)
		if err != nil {
			utils.GetLogInstance().Debug("failed to construct the new block after consensus")
		}

		// Sign the block
		copy(blockObj.Header().Signature[:], aggSig.Serialize()[:])
		copy(blockObj.Header().Bitmap[:], commitBitmap.Bitmap)
		consensus.OnConsensusDone(&blockObj)
		consensus.state = targetState

		select {
		case consensus.VerifiedNewBlock <- &blockObj:
		default:
			utils.GetLogInstance().Info("[SYNC] consensus verified block send to chan failed", "blockHash", blockObj.Hash())
		}

		consensus.reportMetrics(blockObj)

		// Dump new block into level db.
		explorer.GetStorageInstance(consensus.leader.IP, consensus.leader.Port, true).Dump(&blockObj, consensus.consensusID)

		// Reset state to Finished, and clear other data.
		consensus.ResetState()
		consensus.consensusID++

		utils.GetLogInstance().Debug("HOORAY!!! CONSENSUS REACHED!!!", "consensusID", consensus.consensusID, "numOfSignatures", len(*commitSigs))

		// TODO: remove this temporary delay
		time.Sleep(500 * time.Millisecond)
		// Send signal to Node so the new block can be added and new round of consensus can be triggered
		consensus.ReadySignal <- struct{}{}
		consensus.state = Finished
	}
}

func (consensus *Consensus) reportMetrics(block types.Block) {
	endTime := time.Now()
	timeElapsed := endTime.Sub(startTime)
	numOfTxs := len(block.Transactions())
	tps := float64(numOfTxs) / timeElapsed.Seconds()
	utils.GetLogInstance().Info("TPS Report",
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
		"key":             hex.EncodeToString(consensus.pubKey.Serialize()),
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
