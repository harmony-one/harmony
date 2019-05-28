package consensus

import (
	"encoding/hex"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/profiler"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

var (
	startTime time.Time
)

// WaitForNewBlock waits for the next new block to run consensus on
func (consensus *Consensus) WaitForNewBlock(blockChannel chan *types.Block, stopChan chan struct{}, stoppedChan chan struct{}, startChannel chan struct{}) {
	// gensis block is the first block to be processed.
	// But we shouldn't start consensus yet, as we need to wait for all validators
	// received the leader's pub key which will be propogated via Pong message.
	// After we started the first consensus, we will go back to normal case to wait
	// for new blocks.
	// The signal to start the first consensus right now is the sending of Pong message (SendPongMessage function in node/node_handler.go
	// but it can be changed to other conditions later
	first := true
	go func() {
		defer close(stoppedChan)
		for {
			select {
			default:
				if first && startChannel != nil {
					// got the signal to start consensus
					_ = <-startChannel
					first = false
				}

				utils.GetLogInstance().Debug("Waiting for block", "consensus", consensus)
				// keep waiting for new blocks
				newBlock := <-blockChannel
				// TODO: think about potential race condition

				if consensus.ShardID == 0 {
					// TODO ek/rj - re-enable this after fixing DRand
					//if core.IsEpochBlock(newBlock) { // Only beacon chain do randomness generation
					//	// Receive pRnd from DRG protocol
					//	utils.GetLogInstance().Debug("[DRG] Waiting for pRnd")
					//	pRndAndBitmap := <-consensus.PRndChannel
					//	utils.GetLogInstance().Debug("[DRG] Got pRnd", "pRnd", pRndAndBitmap)
					//	pRnd := [32]byte{}
					//	copy(pRnd[:], pRndAndBitmap[:32])
					//	bitmap := pRndAndBitmap[32:]
					//	vrfBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
					//	vrfBitmap.SetMask(bitmap)
					//
					//	// TODO: check validity of pRnd
					//	newBlock.AddRandPreimage(pRnd)
					//}

					rnd, blockHash, err := consensus.GetNextRnd()
					if err == nil {
						// Verify the randomness
						_ = blockHash
						utils.GetLogInstance().Info("Adding randomness into new block", "rnd", rnd)
						newBlock.AddRandSeed(rnd)
					} else {
						utils.GetLogInstance().Info("Failed to get randomness", "error", err)
					}
				}
				startTime = time.Now()
				utils.GetLogInstance().Debug("STARTING CONSENSUS", "numTxs", len(newBlock.Transactions()), "consensus", consensus, "startTime", startTime, "publicKeys", len(consensus.PublicKeys))
				for { // Wait until last consensus is finished
					if consensus.state == Finished {
						consensus.ResetState()
						consensus.startConsensus(newBlock)
						break
					}
					time.Sleep(500 * time.Millisecond)
				}
			case <-stopChan:
				return
			}
		}
	}()
}

// ProcessMessageLeader dispatches consensus message for the leader.
func (consensus *Consensus) ProcessMessageLeader(payload []byte) {
	message := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, message)

	if err != nil {
		utils.GetLogInstance().Error("Failed to unmarshal message payload.", "err", err, "consensus", consensus)
	}

	switch message.Type {
	case msg_pb.MessageType_PREPARE:
		consensus.processPrepareMessage(message)
	case msg_pb.MessageType_COMMIT:
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

	// Leader sign the block hash itself
	consensus.prepareSigs[consensus.SelfAddress] = consensus.priKey.SignHash(consensus.blockHash[:])

	// Construct broadcast p2p message
	utils.GetLogInstance().Warn("[Consensus]", "sent announce message", len(msgToSend))
	consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))
}

// processPrepareMessage processes the prepare message sent from validators
func (consensus *Consensus) processPrepareMessage(message *msg_pb.Message) {
	consensusMsg := message.GetConsensus()

	validatorPubKey, err := bls_cosi.BytesToBlsPublicKey(consensusMsg.SenderPubkey)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to deserialize BLS public key", "error", err)
		return
	}
	addrBytes := validatorPubKey.GetAddress()
	validatorAddress := common.BytesToAddress(addrBytes[:])

	prepareSig := consensusMsg.Payload

	prepareSigs := consensus.prepareSigs
	prepareBitmap := consensus.prepareBitmap

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	if !consensus.IsValidatorInCommittee(validatorAddress) {
		utils.GetLogInstance().Error("Invalid validator", "validatorAddress", validatorAddress)
		return
	}

	if err := consensus.checkConsensusMessage(message, validatorPubKey); err != nil {
		utils.GetLogInstance().Debug("Failed to check the validator message", "error", err, "validatorAddress", validatorAddress)
		return
	}

	// proceed only when the message is not received before
	_, ok := prepareSigs[validatorAddress]
	if ok {
		utils.GetLogInstance().Debug("Already received prepare message from the validator", "validatorAddress", validatorAddress)
		return
	}

	if len(prepareSigs) >= ((len(consensus.PublicKeys)*2)/3 + 1) {
		utils.GetLogInstance().Debug("Received additional prepare message", "validatorAddress", validatorAddress)
		return
	}

	// Check BLS signature for the multi-sig
	var sign bls.Sign
	err = sign.Deserialize(prepareSig)
	if err != nil {
		utils.GetLogInstance().Error("Failed to deserialize bls signature", "validatorAddress", validatorAddress)
		return
	}

	if !sign.VerifyHash(validatorPubKey, consensus.blockHash[:]) {
		utils.GetLogInstance().Error("Received invalid BLS signature", "validatorAddress", validatorAddress)
		return
	}

	utils.GetLogInstance().Debug("Received new prepare signature", "numReceivedSoFar", len(prepareSigs), "validatorAddress", validatorAddress, "PublicKeys", len(consensus.PublicKeys))
	prepareSigs[validatorAddress] = &sign
	prepareBitmap.SetKey(validatorPubKey, true) // Set the bitmap indicating that this validator signed.

	targetState := PreparedDone
	if len(prepareSigs) >= ((len(consensus.PublicKeys)*2)/3+1) && consensus.state < targetState {
		utils.GetLogInstance().Debug("Enough prepares received with signatures", "num", len(prepareSigs), "state", consensus.state)

		// Construct and broadcast prepared message
		msgToSend, aggSig := consensus.constructPreparedMessage()
		consensus.aggregatedPrepareSig = aggSig

		utils.GetLogInstance().Warn("[Consensus]", "sent prepared message", len(msgToSend))
		consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))

		// Set state to targetState
		consensus.state = targetState

		// Leader sign the multi-sig and bitmap (for commit phase)
		multiSigAndBitmap := append(aggSig.Serialize(), prepareBitmap.Bitmap...)
		consensus.commitSigs[consensus.SelfAddress] = consensus.priKey.SignHash(multiSigAndBitmap)
	}
}

// Processes the commit message sent from validators
func (consensus *Consensus) processCommitMessage(message *msg_pb.Message) {
	consensusMsg := message.GetConsensus()

	validatorPubKey, err := bls_cosi.BytesToBlsPublicKey(consensusMsg.SenderPubkey)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to deserialize BLS public key", "error", err)
		return
	}
	addrBytes := validatorPubKey.GetAddress()
	validatorAddress := common.BytesToAddress(addrBytes[:])

	commitSig := consensusMsg.Payload

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	if !consensus.IsValidatorInCommittee(validatorAddress) {
		utils.GetLogInstance().Error("Invalid validator", "validatorAddress", validatorAddress)
		return
	}

	if err := consensus.checkConsensusMessage(message, validatorPubKey); err != nil {
		utils.GetLogInstance().Debug("Failed to check the validator message", "validatorAddress", validatorAddress)
		return
	}

	commitSigs := consensus.commitSigs
	commitBitmap := consensus.commitBitmap

	// proceed only when the message is not received before
	_, ok := commitSigs[validatorAddress]
	if ok {
		utils.GetLogInstance().Debug("Already received commit message from the validator", "validatorAddress", validatorAddress)
		return
	}

	if len((commitSigs)) >= ((len(consensus.PublicKeys)*2)/3 + 1) {
		utils.GetLogInstance().Debug("Received additional new commit message", "validatorAddress", validatorAddress)
		return
	}

	// Verify the signature on prepare multi-sig and bitmap is correct
	var sign bls.Sign
	err = sign.Deserialize(commitSig)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to deserialize bls signature", "validatorAddress", validatorAddress)
		return
	}
	aggSig := bls_cosi.AggregateSig(consensus.GetPrepareSigsArray())
	if !sign.VerifyHash(validatorPubKey, append(aggSig.Serialize(), consensus.prepareBitmap.Bitmap...)) {
		utils.GetLogInstance().Error("Received invalid BLS signature", "validatorAddress", validatorAddress)
		return
	}

	utils.GetLogInstance().Debug("Received new commit message", "numReceivedSoFar", len(commitSigs), "validatorAddress", validatorAddress)
	commitSigs[validatorAddress] = &sign
	// Set the bitmap indicating that this validator signed.
	commitBitmap.SetKey(validatorPubKey, true)

	targetState := CommittedDone
	if len(commitSigs) >= ((len(consensus.PublicKeys)*2)/3+1) && consensus.state != targetState {
		utils.GetLogInstance().Info("Enough commits received!", "num", len(commitSigs), "state", consensus.state)

		// Construct and broadcast committed message
		msgToSend, aggSig := consensus.constructCommittedMessage()
		consensus.aggregatedCommitSig = aggSig

		utils.GetLogInstance().Warn("[Consensus]", "sent committed message", len(msgToSend))
		consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))

		var blockObj types.Block
		err := rlp.DecodeBytes(consensus.block, &blockObj)
		if err != nil {
			utils.GetLogInstance().Debug("failed to construct the new block after consensus")
		}

		// Sign the block
		blockObj.SetPrepareSig(
			consensus.aggregatedPrepareSig.Serialize(),
			consensus.prepareBitmap.Bitmap)
		blockObj.SetCommitSig(
			consensus.aggregatedCommitSig.Serialize(),
			consensus.commitBitmap.Bitmap)

		consensus.state = targetState

		select {
		case consensus.VerifiedNewBlock <- &blockObj:
		default:
			utils.GetLogInstance().Info("[SYNC] Failed to send consensus verified block for state sync", "blockHash", blockObj.Hash())
		}

		consensus.reportMetrics(blockObj)

		// Dump new block into level db.
		explorer.GetStorageInstance(consensus.leader.IP, consensus.leader.Port, true).Dump(&blockObj, consensus.consensusID)

		// Reset state to Finished, and clear other data.
		consensus.ResetState()
		consensus.consensusID++

		consensus.OnConsensusDone(&blockObj)
		utils.GetLogInstance().Debug("HOORAY!!!!!!! CONSENSUS REACHED!!!!!!!", "consensusID", consensus.consensusID, "numOfSignatures", len(commitSigs))

		// TODO: remove this temporary delay
		time.Sleep(500 * time.Millisecond)
		// Send signal to Node so the new block can be added and new round of consensus can be triggered
		consensus.ReadySignal <- struct{}{}
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
		"key":             hex.EncodeToString(consensus.PubKey.Serialize()),
		"tps":             tps,
		"txCount":         numOfTxs,
		"nodeCount":       len(consensus.PublicKeys) + 1,
		"latestBlockHash": hex.EncodeToString(consensus.blockHash[:]),
		"latestTxHashes":  txHashes,
		"blockLatency":    int(timeElapsed / time.Millisecond),
	}
	profiler.LogMetrics(metrics)
}
