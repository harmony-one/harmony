package consensus

import (
	"bytes"
	"encoding/hex"
	"time"

	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"
	vrf_bls "github.com/harmony-one/harmony/crypto/vrf/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/vdf/src/vdf_go"
)

// handlemessageupdate will update the consensus state according to received message
func (consensus *Consensus) handleMessageUpdate(payload []byte) {
	if len(payload) == 0 {
		return
	}
	msg := &msg_pb.Message{}
	if err := protobuf.Unmarshal(payload, msg); err != nil {
		consensus.getLogger().Error().Err(err).Msg("Failed to unmarshal message payload.")
		return
	}

	// when node is in ViewChanging mode, it still accepts normal messages into FBFTLog
	// in order to avoid possible trap forever but drop PREPARE and COMMIT
	// which are message types specifically for a node acting as leader
	switch {
	case (consensus.current.Mode() == ViewChanging) &&
		(msg.Type == msg_pb.MessageType_PREPARE ||
			msg.Type == msg_pb.MessageType_COMMIT):
		return
	case consensus.current.Mode() == Listening:
		return
	}

	if msg.Type == msg_pb.MessageType_VIEWCHANGE ||
		msg.Type == msg_pb.MessageType_NEWVIEW {
		if msg.GetViewchange() != nil &&
			msg.GetViewchange().ShardId != consensus.ShardID {
			consensus.getLogger().Warn().
				Uint32("myShardId", consensus.ShardID).
				Uint32("receivedShardId", msg.GetViewchange().ShardId).
				Msg("Received view change message from different shard")
			return
		}
	} else {
		if msg.GetConsensus() != nil &&
			msg.GetConsensus().ShardId != consensus.ShardID {
			consensus.getLogger().Warn().
				Uint32("myShardId", consensus.ShardID).
				Uint32("receivedShardId", msg.GetConsensus().ShardId).
				Msg("Received consensus message from different shard")
			return
		}
	}

	notMemberButStillCatchup := !consensus.Decider.AmIMemberOfCommitee() &&
		msg.Type == msg_pb.MessageType_COMMITTED

	if notMemberButStillCatchup {
		consensus.onCommitted(msg)
		return
	}

	intendedForValidator, intendedForLeader :=
		!(consensus.IsLeader() && consensus.current.Mode() == Normal),
		consensus.IsLeader()

	switch t := msg.Type; true {
	// Handle validator intended messages first
	case t == msg_pb.MessageType_ANNOUNCE &&
		intendedForValidator &&
		consensus.validatorSanityChecks(msg):
		consensus.onAnnounce(msg)
	case t == msg_pb.MessageType_PREPARED &&
		intendedForValidator &&
		consensus.validatorSanityChecks(msg):
		consensus.onPrepared(msg)
	case t == msg_pb.MessageType_COMMITTED &&
		intendedForValidator &&
		consensus.validatorSanityChecks(msg):
		consensus.onCommitted(msg)
	// Handle leader intended messages now
	case t == msg_pb.MessageType_PREPARE &&
		intendedForLeader &&
		consensus.leaderSanityChecks(msg):
		consensus.onPrepare(msg)
	case t == msg_pb.MessageType_COMMIT &&
		intendedForLeader &&
		consensus.leaderSanityChecks(msg):
		consensus.onCommit(msg)
	case t == msg_pb.MessageType_VIEWCHANGE &&
		consensus.viewChangeSanityCheck(msg):
		consensus.onViewChange(msg)
	case t == msg_pb.MessageType_NEWVIEW &&
		consensus.viewChangeSanityCheck(msg):
		consensus.onNewView(msg)
	}
}

func (consensus *Consensus) finalizeCommits() {
	consensus.getLogger().Info().
		Int64("NumCommits", consensus.Decider.SignersCount(quorum.Commit)).
		Msg("[Finalizing] Finalizing Block")
	beforeCatchupNum := consensus.blockNum
	leaderPriKey, err := consensus.GetConsensusLeaderPrivateKey()
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("[FinalizeCommits] leader not found")
		return
	}
	// Construct committed message
	network, err := consensus.construct(msg_pb.MessageType_COMMITTED, nil, leaderPriKey.GetPublicKey(), leaderPriKey)
	if err != nil {
		consensus.getLogger().Warn().Err(err).
			Msg("[FinalizeCommits] Unable to construct Committed message")
		return
	}
	msgToSend, aggSig, FBFTMsg :=
		network.Bytes,
		network.OptionalAggregateSignature,
		network.FBFTMsg
	consensus.aggregatedCommitSig = aggSig // this may not needed
	consensus.FBFTLog.AddMessage(FBFTMsg)
	// find correct block content
	curBlockHash := consensus.blockHash
	block := consensus.FBFTLog.GetBlockByHash(curBlockHash)
	if block == nil {
		consensus.getLogger().Warn().
			Str("blockHash", hex.EncodeToString(curBlockHash[:])).
			Msg("[FinalizeCommits] Cannot find block by hash")
		return
	}

	consensus.tryCatchup()
	if consensus.blockNum-beforeCatchupNum != 1 {
		consensus.getLogger().Warn().
			Uint64("beforeCatchupBlockNum", beforeCatchupNum).
			Msg("[FinalizeCommits] Leader cannot provide the correct block for committed message")
		return
	}

	if err := consensus.ChainReader.WriteLastCommits(FBFTMsg.Payload); err != nil {
		consensus.getLogger().Err(err).
			Msg("[FinalizeCommits] could not write last commits")
		return
	}

	// if leader success finalize the block, send committed message to validators
	if err := consensus.msgSender.SendWithRetry(
		block.NumberU64(),
		msg_pb.MessageType_COMMITTED, []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
		host.ConstructP2pMessage(byte(17), msgToSend)); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[Finalizing] Cannot send committed message")
	} else {
		consensus.getLogger().Info().
			Hex("blockHash", curBlockHash[:]).
			Uint64("blockNum", consensus.blockNum).
			Msg("[Finalizing] Sent Committed Message")
	}

	consensus.reportMetrics(*block)

	// Dump new block into level db
	// In current code, we add signatures in block in tryCatchup, the block dump to explorer does not contains signatures
	// but since explorer doesn't need signatures, it should be fine
	// in future, we will move signatures to next block
	//explorer.GetStorageInstance(consensus.leader.IP, consensus.leader.Port, true).Dump(block, beforeCatchupNum)

	if consensus.consensusTimeout[timeoutBootstrap].IsActive() {
		consensus.consensusTimeout[timeoutBootstrap].Stop()
		consensus.getLogger().Debug().Msg("[Finalizing] Start consensus timer; stop bootstrap timer only once")
	} else {
		consensus.getLogger().Debug().Msg("[Finalizing] Start consensus timer")
	}
	consensus.consensusTimeout[timeoutConsensus].Start()

	consensus.getLogger().Info().
		Uint64("blockNum", block.NumberU64()).
		Uint64("epochNum", block.Epoch().Uint64()).
		Uint64("ViewId", block.Header().ViewID().Uint64()).
		Str("blockHash", block.Hash().String()).
		Int("index", consensus.Decider.IndexOf(consensus.LeaderPubKey)).
		Int("numTxns", len(block.Transactions())).
		Int("numStakingTxns", len(block.StakingTransactions())).
		Msg("HOORAY!!!!!!! CONSENSUS REACHED!!!!!!!")
	// Send signal to Node so the new block can be added and new round of consensus can be triggered
	consensus.ReadySignal <- struct{}{}
}

// LastCommitSig returns the byte array of aggregated
// commit signature and bitmap of last block
func (consensus *Consensus) LastCommitSig() ([]byte, []byte, error) {
	if consensus.blockNum <= 1 {
		return nil, nil, nil
	}
	lastCommits, err := consensus.ChainReader.ReadLastCommits()
	if err != nil ||
		len(lastCommits) < shard.BLSSignatureSizeInBytes {
		msgs := consensus.FBFTLog.GetMessagesByTypeSeq(msg_pb.MessageType_COMMITTED, consensus.blockNum-1)
		if len(msgs) != 1 {
			consensus.getLogger().Error().
				Int("numCommittedMsg", len(msgs)).
				Msg("GetLastCommitSig failed with wrong number of committed message")
			return nil, nil, ctxerror.New("GetLastCommitSig failed with wrong number of committed message", "numCommittedMsg", len(msgs))
		}
		lastCommits = msgs[0].Payload
	}
	//#### Read payload data from committed msg
	aggSig := make([]byte, shard.BLSSignatureSizeInBytes)
	bitmap := make([]byte, len(lastCommits)-shard.BLSSignatureSizeInBytes)
	offset := 0
	copy(aggSig[:], lastCommits[offset:offset+shard.BLSSignatureSizeInBytes])
	offset += shard.BLSSignatureSizeInBytes
	copy(bitmap[:], lastCommits[offset:])
	//#### END Read payload data from committed msg
	return aggSig, bitmap, nil
}

// try to catch up if fall behind
func (consensus *Consensus) tryCatchup() {
	consensus.getLogger().Info().Msg("[TryCatchup] commit new blocks")
	currentBlockNum := consensus.blockNum
	for {
		msgs := consensus.FBFTLog.GetMessagesByTypeSeq(
			msg_pb.MessageType_COMMITTED, consensus.blockNum,
		)
		if len(msgs) == 0 {
			break
		}
		if len(msgs) > 1 {
			consensus.getLogger().Error().
				Int("numMsgs", len(msgs)).
				Msg("[TryCatchup] DANGER!!! we should only get one committed message for a given blockNum")
		}
		consensus.getLogger().Info().Msg("[TryCatchup] committed message found")

		block := consensus.FBFTLog.GetBlockByHash(msgs[0].BlockHash)
		if block == nil {
			blksRepr, msgsRepr, incomingMsg :=
				consensus.FBFTLog.Blocks().String(),
				consensus.FBFTLog.Messages().String(),
				msgs[0].String()
			consensus.getLogger().Debug().
				Str("FBFT-log-blocks", blksRepr).
				Str("FBFT-log-messages", msgsRepr).
				Str("incoming-message", incomingMsg).
				Msg("block was nil invariant in consensus violated")
			break
		}

		if consensus.BlockVerifier == nil {
			// do nothing
		} else if err := consensus.BlockVerifier(block); err != nil {
			consensus.getLogger().Info().Msg("[TryCatchup] block verification failed")
			return
		}

		if block.ParentHash() != consensus.ChainReader.CurrentHeader().Hash() {
			consensus.getLogger().Debug().Msg("[TryCatchup] parent block hash not match")
			break
		}
		consensus.getLogger().Info().Msg("[TryCatchup] block found to commit")

		preparedMsgs := consensus.FBFTLog.GetMessagesByTypeSeqHash(
			msg_pb.MessageType_PREPARED, msgs[0].BlockNum, msgs[0].BlockHash,
		)
		msg := consensus.FBFTLog.FindMessageByMaxViewID(preparedMsgs)
		if msg == nil {
			break
		}
		consensus.getLogger().Info().Msg("[TryCatchup] prepared message found to commit")

		// TODO(Chao): Explain the reasoning for these code
		consensus.blockHash = [32]byte{}
		consensus.blockNum = consensus.blockNum + 1
		consensus.viewID = msgs[0].ViewID + 1
		consensus.LeaderPubKey = msgs[0].SenderPubkey

		consensus.getLogger().Info().Msg("[TryCatchup] Adding block to chain")
		consensus.OnConsensusDone(block, msgs[0].Payload)
		consensus.ResetState()

		select {
		case consensus.VerifiedNewBlock <- block:
		default:
			consensus.getLogger().Info().
				Str("blockHash", block.Hash().String()).
				Msg("[TryCatchup] consensus verified block send to chan failed")
			continue
		}

		break
	}
	if currentBlockNum < consensus.blockNum {
		consensus.getLogger().Info().
			Uint64("From", currentBlockNum).
			Uint64("To", consensus.blockNum).
			Msg("[TryCatchup] Caught up!")
		consensus.switchPhase(FBFTAnnounce, true)
	}
	// catup up and skip from view change trap
	if currentBlockNum < consensus.blockNum &&
		consensus.current.Mode() == ViewChanging {
		consensus.current.SetMode(Normal)
		consensus.consensusTimeout[timeoutViewChange].Stop()
	}
	// clean up old log
	consensus.FBFTLog.DeleteBlocksLessThan(consensus.blockNum - 1)
	consensus.FBFTLog.DeleteMessagesLessThan(consensus.blockNum - 1)
}

// Start waits for the next new block and run consensus
func (consensus *Consensus) Start(
	blockChannel chan *types.Block, stopChan, stoppedChan, startChannel chan struct{},
) {
	go func() {
		toStart := false
		isInitialLeader := consensus.IsLeader()
		if isInitialLeader {
			consensus.getLogger().Info().Time("time", time.Now()).Msg("[ConsensusMainLoop] Waiting for consensus start")
			// send a signal to indicate it's ready to run consensus
			// this signal is consumed by node object to create a new block and in turn trigger a new consensus on it
			go func() {
				<-startChannel
				toStart = true
				consensus.getLogger().Info().Time("time", time.Now()).Msg("[ConsensusMainLoop] Send ReadySignal")
				consensus.ReadySignal <- struct{}{}
			}()
		}
		consensus.getLogger().Info().Time("time", time.Now()).Msg("[ConsensusMainLoop] Consensus started")
		defer close(stoppedChan)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		consensus.consensusTimeout[timeoutBootstrap].Start()
		consensus.getLogger().Debug().
			Uint64("viewID", consensus.viewID).
			Uint64("blockNum", consensus.blockNum).
			Msg("[ConsensusMainLoop] Start bootstrap timeout (only once)")

		vdfInProgress := false
		for {
			select {
			case <-ticker.C:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] Ticker")
				if toStart == false && isInitialLeader {
					continue
				}
				for k, v := range consensus.consensusTimeout {
					if consensus.current.Mode() == Syncing ||
						consensus.current.Mode() == Listening {
						v.Stop()
					}
					if !v.CheckExpire() {
						continue
					}
					if k != timeoutViewChange {
						consensus.getLogger().Debug().Msg("[ConsensusMainLoop] Ops Consensus Timeout!!!")
						consensus.startViewChange(consensus.viewID + 1)
						break
					} else {
						consensus.getLogger().Debug().Msg("[ConsensusMainLoop] Ops View Change Timeout!!!")
						viewID := consensus.current.ViewID()
						consensus.startViewChange(viewID + 1)
						break
					}
				}
			case <-consensus.syncReadyChan:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] syncReadyChan")
				consensus.SetBlockNum(consensus.ChainReader.CurrentHeader().Number().Uint64() + 1)
				consensus.SetViewID(consensus.ChainReader.CurrentHeader().ViewID().Uint64() + 1)
				mode := consensus.UpdateConsensusInformation()
				consensus.current.SetMode(mode)
				consensus.getLogger().Info().Str("Mode", mode.String()).Msg("Node is in sync")

			case <-consensus.syncNotReadyChan:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] syncNotReadyChan")
				consensus.SetBlockNum(consensus.ChainReader.CurrentHeader().Number().Uint64() + 1)
				consensus.current.SetMode(Syncing)
				consensus.getLogger().Info().Msg("Node is out of sync")

			case newBlock := <-blockChannel:
				// Debug code to trigger leader change.
				//if consensus.ShardID == 0 && newBlock.NumberU64() == 2 && strings.Contains(consensus.PubKey.SerializeToHexStr(), "65f55eb") {
				//	continue
				//}

				consensus.getLogger().Info().
					Uint64("MsgBlockNum", newBlock.NumberU64()).
					Msg("[ConsensusMainLoop] Received Proposed New Block!")

				//VRF/VDF is only generated in the beacon chain
				if consensus.NeedsRandomNumberGeneration(newBlock.Header().Epoch()) {
					// generate VRF if the current block has a new leader
					if !consensus.ChainReader.IsSameLeaderAsPreviousBlock(newBlock) {
						vrfBlockNumbers, err := consensus.ChainReader.ReadEpochVrfBlockNums(newBlock.Header().Epoch())
						if err != nil {
							consensus.getLogger().Info().
								Uint64("MsgBlockNum", newBlock.NumberU64()).
								Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
								Msg("[ConsensusMainLoop] no VRF block number from local db")
						}

						//check if VRF is already generated for the current block
						vrfAlreadyGenerated := false
						for _, v := range vrfBlockNumbers {
							if v == newBlock.NumberU64() {
								consensus.getLogger().Info().
									Uint64("MsgBlockNum", newBlock.NumberU64()).
									Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
									Msg("[ConsensusMainLoop] VRF is already generated for this block")
								vrfAlreadyGenerated = true
								break
							}
						}

						if !vrfAlreadyGenerated {
							//generate a new VRF for the current block
							vrfBlockNumbers := consensus.GenerateVrfAndProof(newBlock, vrfBlockNumbers)

							//generate a new VDF for the current epoch if there are enough VRFs in the current epoch
							//note that  >= instead of == is used, because it is possible the current leader
							//can commit this block, go offline without finishing VDF
							if (!vdfInProgress) && len(vrfBlockNumbers) >= consensus.VdfSeedSize() {
								//check local database to see if there's a VDF generated for this epoch
								//generate a VDF if no blocknum is available
								_, err := consensus.ChainReader.ReadEpochVdfBlockNum(newBlock.Header().Epoch())
								if err != nil {
									consensus.GenerateVdfAndProof(newBlock, vrfBlockNumbers)
									vdfInProgress = true
								}
							}
						}
					}

					vdfOutput, seed, err := consensus.GetNextRnd()
					if err == nil {
						vdfInProgress = false
						// Verify the randomness
						vdfObject := vdf_go.New(shard.Schedule.VdfDifficulty(), seed)
						if !vdfObject.Verify(vdfOutput) {
							consensus.getLogger().Warn().
								Uint64("MsgBlockNum", newBlock.NumberU64()).
								Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
								Msg("[ConsensusMainLoop] failed to verify the VDF output")
						} else {
							//write the VDF only if VDF has not been generated
							_, err := consensus.ChainReader.ReadEpochVdfBlockNum(newBlock.Header().Epoch())
							if err == nil {
								consensus.getLogger().Info().
									Uint64("MsgBlockNum", newBlock.NumberU64()).
									Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
									Msg("[ConsensusMainLoop] VDF has already been generated previously")
							} else {
								consensus.getLogger().Info().
									Uint64("MsgBlockNum", newBlock.NumberU64()).
									Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
									Msg("[ConsensusMainLoop] Generated a new VDF")

								newBlock.AddVdf(vdfOutput[:])
							}
						}
					} else {
						//consensus.getLogger().Error().Err(err). Msg("[ConsensusMainLoop] Failed to get randomness")
					}
				}

				startTime = time.Now()
				consensus.msgSender.Reset(newBlock.NumberU64())

				consensus.getLogger().Debug().
					Int("numTxs", len(newBlock.Transactions())).
					Time("startTime", startTime).
					Int64("publicKeys", consensus.Decider.ParticipantsCount()).
					Msg("[ConsensusMainLoop] STARTING CONSENSUS")
				consensus.announce(newBlock)

			case msg := <-consensus.MsgChan:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] MsgChan")
				consensus.handleMessageUpdate(msg)

			case viewID := <-consensus.commitFinishChan:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] commitFinishChan")
				// Only Leader execute this condition
				func() {
					consensus.mutex.Lock()
					defer consensus.mutex.Unlock()
					if viewID == consensus.viewID {
						consensus.finalizeCommits()
					}
				}()

			case <-stopChan:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] stopChan")
				return
			}
		}
		consensus.getLogger().Debug().Msg("[ConsensusMainLoop] Ended.")
	}()
}

// GenerateVrfAndProof generates new VRF/Proof from hash of previous block
func (consensus *Consensus) GenerateVrfAndProof(newBlock *types.Block, vrfBlockNumbers []uint64) []uint64 {
	key, err := consensus.GetConsensusLeaderPrivateKey()
	if err != nil {
		consensus.getLogger().Error().
			Err(err).
			Msg("[GenerateVrfAndProof] VRF generation error")
		return vrfBlockNumbers
	}
	sk := vrf_bls.NewVRFSigner(key)
	blockHash := [32]byte{}
	previousHeader := consensus.ChainReader.GetHeaderByNumber(
		newBlock.NumberU64() - 1,
	)
	previousHash := previousHeader.Hash()
	copy(blockHash[:], previousHash[:])

	vrf, proof := sk.Evaluate(blockHash[:])
	newBlock.AddVrf(append(vrf[:], proof...))

	consensus.getLogger().Info().
		Uint64("MsgBlockNum", newBlock.NumberU64()).
		Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
		Int("Num of VRF", len(vrfBlockNumbers)).
		Msg("[ConsensusMainLoop] Leader generated a VRF")

	return vrfBlockNumbers
}

// ValidateVrfAndProof validates a VRF/Proof from hash of previous block
func (consensus *Consensus) ValidateVrfAndProof(headerObj *block.Header) bool {
	vrfPk := vrf_bls.NewVRFVerifier(consensus.LeaderPubKey)
	var blockHash [32]byte
	previousHeader := consensus.ChainReader.GetHeaderByNumber(
		headerObj.Number().Uint64() - 1,
	)
	previousHash := previousHeader.Hash()
	copy(blockHash[:], previousHash[:])
	vrfProof := [96]byte{}
	copy(vrfProof[:], headerObj.Vrf()[32:])
	hash, err := vrfPk.ProofToHash(blockHash[:], vrfProof[:])

	if err != nil {
		consensus.getLogger().Warn().
			Err(err).
			Str("MsgBlockNum", headerObj.Number().String()).
			Msg("[OnAnnounce] VRF verification error")
		return false
	}

	if !bytes.Equal(hash[:], headerObj.Vrf()[:32]) {
		consensus.getLogger().Warn().
			Str("MsgBlockNum", headerObj.Number().String()).
			Msg("[OnAnnounce] VRF proof is not valid")
		return false
	}

	vrfBlockNumbers, _ := consensus.ChainReader.ReadEpochVrfBlockNums(
		headerObj.Epoch(),
	)
	consensus.getLogger().Info().
		Str("MsgBlockNum", headerObj.Number().String()).
		Int("Number of VRF", len(vrfBlockNumbers)).
		Msg("[OnAnnounce] validated a new VRF")

	return true
}

// GenerateVdfAndProof generates new VDF/Proof from VRFs in the current epoch
func (consensus *Consensus) GenerateVdfAndProof(newBlock *types.Block, vrfBlockNumbers []uint64) {
	//derive VDF seed from VRFs generated in the current epoch
	seed := [32]byte{}
	for i := 0; i < consensus.VdfSeedSize(); i++ {
		previousVrf := consensus.ChainReader.GetVrfByNumber(vrfBlockNumbers[i])
		for j := 0; j < len(seed); j++ {
			seed[j] = seed[j] ^ previousVrf[j]
		}
	}

	consensus.getLogger().Info().
		Uint64("MsgBlockNum", newBlock.NumberU64()).
		Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
		Int("Num of VRF", len(vrfBlockNumbers)).
		Msg("[ConsensusMainLoop] VDF computation started")

	// TODO ek – limit concurrency
	go func() {
		vdf := vdf_go.New(shard.Schedule.VdfDifficulty(), seed)
		outputChannel := vdf.GetOutputChannel()
		start := time.Now()
		vdf.Execute()
		duration := time.Now().Sub(start)
		consensus.getLogger().Info().
			Dur("duration", duration).
			Msg("[ConsensusMainLoop] VDF computation finished")
		output := <-outputChannel

		// The first 516 bytes are the VDF+proof and the last 32 bytes are XORed VRF as seed
		rndBytes := [548]byte{}
		copy(rndBytes[:516], output[:])
		copy(rndBytes[516:], seed[:])
		consensus.RndChannel <- rndBytes
	}()
}

// ValidateVdfAndProof validates the VDF/proof in the current epoch
func (consensus *Consensus) ValidateVdfAndProof(headerObj *block.Header) bool {
	vrfBlockNumbers, err := consensus.ChainReader.ReadEpochVrfBlockNums(headerObj.Epoch())
	if err != nil {
		consensus.getLogger().Error().Err(err).
			Str("MsgBlockNum", headerObj.Number().String()).
			Msg("[OnAnnounce] failed to read VRF block numbers for VDF computation")
	}

	//extra check to make sure there's no index out of range error
	//it can happen if epoch is messed up, i.e. VDF ouput is generated in the next epoch
	if consensus.VdfSeedSize() > len(vrfBlockNumbers) {
		return false
	}

	seed := [32]byte{}
	for i := 0; i < consensus.VdfSeedSize(); i++ {
		previousVrf := consensus.ChainReader.GetVrfByNumber(vrfBlockNumbers[i])
		for j := 0; j < len(seed); j++ {
			seed[j] = seed[j] ^ previousVrf[j]
		}
	}

	vdfObject := vdf_go.New(shard.Schedule.VdfDifficulty(), seed)
	vdfOutput := [516]byte{}
	copy(vdfOutput[:], headerObj.Vdf())
	if vdfObject.Verify(vdfOutput) {
		consensus.getLogger().Info().
			Str("MsgBlockNum", headerObj.Number().String()).
			Int("Num of VRF", consensus.VdfSeedSize()).
			Msg("[OnAnnounce] validated a new VDF")

	} else {
		consensus.getLogger().Warn().
			Str("MsgBlockNum", headerObj.Number().String()).
			Uint64("Epoch", headerObj.Epoch().Uint64()).
			Int("Num of VRF", consensus.VdfSeedSize()).
			Msg("[OnAnnounce] VDF proof is not valid")
		return false
	}

	return true
}
