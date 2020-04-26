package consensus

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"time"

	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"
	vrf_bls "github.com/harmony-one/harmony/crypto/vrf/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/vdf/src/vdf_go"
	"github.com/pkg/errors"
)

// HandleMessageUpdate will update the consensus state according to received message
func (consensus *Consensus) HandleMessageUpdate(payload []byte) {
	if len(payload) == 0 {
		return
	}
	msg := &msg_pb.Message{}
	if err := protobuf.Unmarshal(payload, msg); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to unmarshal message payload.")
		return
	}

	// when node is in ViewChanging mode, it still accepts normal messages into FBFTLog
	// in order to avoid possible trap forever but drop PREPARE and COMMIT
	// which are message types specifically for a node acting as leader
	if (consensus.current.Mode() == ViewChanging) &&
		(msg.Type == msg_pb.MessageType_PREPARE ||
			msg.Type == msg_pb.MessageType_COMMIT) {
		return
	}

	if msg.Type == msg_pb.MessageType_VIEWCHANGE ||
		msg.Type == msg_pb.MessageType_NEWVIEW {
		if vc := msg.GetViewchange(); vc != nil &&
			vc.ShardId != consensus.ShardID {
			utils.Logger().Warn().
				Uint32("myShardId", consensus.ShardID).
				Uint32("receivedShardId", vc.ShardId).
				Msg("Received view change message from different shard")
			return
		}
	} else {
		if con := msg.GetConsensus(); con != nil &&
			con.ShardId != consensus.ShardID {
			utils.Logger().Warn().
				Uint32("myShardId", consensus.ShardID).
				Uint32("receivedShardId", con.ShardId).
				Msg("Received consensus message from different shard")
			return
		}
	}

	intendedForValidator, intendedForLeader :=
		!consensus.IsLeader(),
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
	utils.Logger().Info().
		Int64("NumCommits", consensus.Decider.SignersCount(quorum.Commit)).
		Msg("[finalizeCommits] Finalizing Block")
	beforeCatchupNum := consensus.blockNum
	leaderPriKey, err := consensus.GetConsensusLeaderPrivateKey()
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[FinalizeCommits] leader not found")
		return
	}
	// Construct committed message
	network, err := consensus.construct(
		msg_pb.MessageType_COMMITTED, nil, leaderPriKey.GetPublicKey(), leaderPriKey,
	)
	if err != nil {
		utils.Logger().Warn().Err(err).
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
		utils.Logger().Warn().
			Str("blockHash", hex.EncodeToString(curBlockHash[:])).
			Msg("[FinalizeCommits] Cannot find block by hash")
		return
	}

	consensus.tryCatchup()
	if consensus.blockNum-beforeCatchupNum != 1 {
		utils.Logger().Warn().
			Uint64("beforeCatchupBlockNum", beforeCatchupNum).
			Msg("[FinalizeCommits] Leader cannot provide the correct block for committed message")
		return
	}

	// if leader success finalize the block, send committed message to validators
	if err := consensus.msgSender.SendWithRetry(
		block.NumberU64(),
		msg_pb.MessageType_COMMITTED, []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
		p2p.ConstructMessage(msgToSend)); err != nil {
		utils.Logger().Warn().Err(err).Msg("[finalizeCommits] Cannot send committed message")
	} else {
		utils.Logger().Info().
			Hex("blockHash", curBlockHash[:]).
			Uint64("blockNum", consensus.blockNum).
			Msg("[finalizeCommits] Sent Committed Message")
	}

	consensus.timeouts.consensus.Start()

	utils.Logger().Info().
		Uint64("blockNum", block.NumberU64()).
		Uint64("epochNum", block.Epoch().Uint64()).
		Uint64("ViewId", block.Header().ViewID().Uint64()).
		Str("blockHash", block.Hash().String()).
		Int("index", consensus.Decider.IndexOf(consensus.LeaderPubKey)).
		Int("numTxns", len(block.Transactions())).
		Int("numStakingTxns", len(block.StakingTransactions())).
		Msg("HOORAY!!!!!!! CONSENSUS REACHED!!!!!!!")

	if n := time.Now(); n.Before(consensus.NextBlockDue) {
		// Sleep to wait for the full block time
		utils.Logger().Debug().Msg("[finalizeCommits] Waiting for Block Time")
		time.Sleep(consensus.NextBlockDue.Sub(n))
	}

	// Update time due for next block
	consensus.NextBlockDue = time.Now().Add(consensus.BlockPeriod)
}

// BlockCommitSig returns the byte array of aggregated
// commit signature and bitmap signed on the block
func (consensus *Consensus) BlockCommitSig(blockNum uint64) ([]byte, []byte, error) {
	if consensus.blockNum <= 1 {
		return nil, nil, nil
	}
	lastCommits, err := consensus.ChainReader.ReadCommitSig(blockNum)
	if err != nil ||
		len(lastCommits) < shard.BLSSignatureSizeInBytes {
		msgs := consensus.FBFTLog.GetMessagesByTypeSeq(
			msg_pb.MessageType_COMMITTED, consensus.blockNum-1,
		)
		if len(msgs) != 1 {
			utils.Logger().Error().
				Int("numCommittedMsg", len(msgs)).
				Msg("GetLastCommitSig failed with wrong number of committed message")
			return nil, nil, errors.Errorf(
				"GetLastCommitSig failed with wrong number of committed message %d", len(msgs),
			)
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
	utils.Logger().Info().Msg("[TryCatchup] commit new blocks")
	currentBlockNum := consensus.blockNum
	for {
		msgs := consensus.FBFTLog.GetMessagesByTypeSeq(
			msg_pb.MessageType_COMMITTED, consensus.blockNum,
		)
		if len(msgs) == 0 {
			break
		}
		if len(msgs) > 1 {
			utils.Logger().Error().
				Int("numMsgs", len(msgs)).
				Msg("DANGER!!! we should only get one committed message for a given blockNum")
		}

		var committedMsg *FBFTMessage
		var block *types.Block
		for i := range msgs {
			tmpBlock := consensus.FBFTLog.GetBlockByHash(msgs[i].BlockHash)
			if tmpBlock == nil {
				blksRepr, msgsRepr, incomingMsg :=
					consensus.FBFTLog.Blocks().String(),
					consensus.FBFTLog.Messages().String(),
					msgs[i].String()
				utils.Logger().Debug().
					Str("FBFT-log-blocks", blksRepr).
					Str("FBFT-log-messages", msgsRepr).
					Str("incoming-message", incomingMsg).
					Uint64("blockNum", msgs[i].BlockNum).
					Uint64("viewID", msgs[i].ViewID).
					Str("blockHash", msgs[i].BlockHash.Hex()).
					Msg("[TryCatchup] Failed finding a matching block for committed message")
				continue
			}

			resp := make(chan error)
			consensus.Verify.Request <- blkComeback{tmpBlock, resp}
			if err := <-resp; err != nil {
				utils.Logger().Error().Err(err).
					Msg("block verification failed in try catchup")
				continue
			}

			committedMsg = msgs[i]
			block = tmpBlock
			break
		}
		if block == nil || committedMsg == nil {
			utils.Logger().Error().Msg("[TryCatchup] Failed finding a valid committed message.")
			break
		}

		if block.ParentHash() != consensus.ChainReader.CurrentHeader().Hash() {
			utils.Logger().Debug().Msg("[TryCatchup] parent block hash not match")
			break
		}
		utils.Logger().Info().Msg("[TryCatchup] block found to commit")

		preparedMsgs := consensus.FBFTLog.GetMessagesByTypeSeqHash(
			msg_pb.MessageType_PREPARED, committedMsg.BlockNum, committedMsg.BlockHash,
		)
		msg := consensus.FBFTLog.FindMessageByMaxViewID(preparedMsgs)
		if msg == nil {
			break
		}
		utils.Logger().Info().Msg("[TryCatchup] prepared message found to commit")

		// TODO(Chao): Explain the reasoning for these code
		consensus.blockHash = [32]byte{}
		consensus.blockNum = consensus.blockNum + 1
		consensus.viewID = committedMsg.ViewID + 1
		consensus.LeaderPubKey = committedMsg.SenderPubkey

		utils.Logger().Info().Msg("[TryCatchup] Adding block to chain")

		// Fill in the commit signatures
		block.SetCurrentCommitSig(committedMsg.Payload)
		resp := make(chan error)
		consensus.RoundCompleted.Request <- blkComeback{block, resp}

		if err := <-resp; err != nil {
			utils.Logger().Error().Err(err).
				Msg("block processing after finishing consensus failed")
		}

		consensus.ResetState()
		// TODO need to let state sync know that i caught up somehow

		break
	}

	if currentBlockNum < consensus.blockNum {
		utils.Logger().Info().
			Uint64("From", currentBlockNum).
			Uint64("To", consensus.blockNum).
			Msg("[TryCatchup] Caught up!")
		consensus.switchPhase(FBFTAnnounce, true)
	}
	// catup up and skip from view change trap
	if currentBlockNum < consensus.blockNum &&
		consensus.current.Mode() == ViewChanging {
		consensus.current.SetMode(Normal)
	}
	// clean up old log
	consensus.FBFTLog.DeleteBlocksLessThan(consensus.blockNum - 1)
	consensus.FBFTLog.DeleteMessagesLessThan(consensus.blockNum - 1)
}

// Start waits for the next new block and run consensus
func (consensus *Consensus) Start(
	blockChannel chan *types.Block,
) error {

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	vdfInProgress := false

	if consensus.IsLeader() {
		consensus.current.SetMode(Normal)
		go func() {
			// give other nodes at network startup a chance to do things
			time.Sleep(10 * time.Second)
			consensus.ReadySignal <- struct{}{}
			utils.Logger().Info().Msg("leader sent out consensus ready signal")
		}()
	}

	// Set up next block due time.
	consensus.NextBlockDue = time.Now().Add(consensus.BlockPeriod)
	consensus.timeouts.consensus.Start()
	consensus.timeouts.viewChange.Start()

	for {

		select {
		case <-ticker.C:
			utils.Logger().Debug().Msg("[ConsensusMainLoop] Ticker")
			if !consensus.IsLeader() {
				continue
			}
			// TODO think about this some more
			if m := consensus.current.Mode(); m == Syncing || m == Listening {
				if !consensus.timeouts.consensus.WithinLimit() {
					utils.Logger().Debug().Msg("[ConsensusMainLoop] Ops Consensus Timeout!!!")
					consensus.startViewChange(consensus.viewID + 1)
				} else if !consensus.timeouts.viewChange.WithinLimit() {
					utils.Logger().Debug().Msg("[ConsensusMainLoop] Ops View Change Timeout!!!")
					viewID := consensus.current.ViewID()
					consensus.startViewChange(viewID + 1)
				}
			}

		case <-consensus.syncReadyChan:
			utils.Logger().Debug().Msg("[ConsensusMainLoop] syncReadyChan")
			consensus.SetBlockNum(consensus.ChainReader.CurrentHeader().Number().Uint64() + 1)
			consensus.SetViewID(consensus.ChainReader.CurrentHeader().ViewID().Uint64() + 1)
			mode := consensus.UpdateConsensusInformation()
			consensus.current.SetMode(mode)
			utils.Logger().Info().Str("Mode", mode.String()).Msg("Node is IN SYNC")

		case <-consensus.syncNotReadyChan:
			utils.Logger().Debug().Msg("[ConsensusMainLoop] syncNotReadyChan")
			consensus.SetBlockNum(consensus.ChainReader.CurrentHeader().Number().Uint64() + 1)
			consensus.current.SetMode(Syncing)
			utils.Logger().Info().Msg("[ConsensusMainLoop] Node is OUT OF SYNC")

		case newBlock := <-blockChannel:

			utils.Logger().Info().
				Uint64("MsgBlockNum", newBlock.NumberU64()).
				Msg("[ConsensusMainLoop] Received Proposed New Block!")

			//VRF/VDF is only generated in the beacon chain
			if consensus.NeedsRandomNumberGeneration(newBlock.Header().Epoch()) {
				// generate VRF if the current block has a new leader
				if !consensus.ChainReader.IsSameLeaderAsPreviousBlock(newBlock) {
					vrfBlockNumbers, err := consensus.ChainReader.ReadEpochVrfBlockNums(
						newBlock.Header().Epoch(),
					)
					if err != nil {
						utils.Logger().Info().
							Uint64("MsgBlockNum", newBlock.NumberU64()).
							Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
							Msg("[ConsensusMainLoop] no VRF block number from local db")
					}

					//check if VRF is already generated for the current block
					vrfAlreadyGenerated := false
					for _, v := range vrfBlockNumbers {
						if v == newBlock.NumberU64() {
							utils.Logger().Info().
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
						utils.Logger().Warn().
							Uint64("MsgBlockNum", newBlock.NumberU64()).
							Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
							Msg("[ConsensusMainLoop] failed to verify the VDF output")
					} else {
						//write the VDF only if VDF has not been generated
						_, err := consensus.ChainReader.ReadEpochVdfBlockNum(newBlock.Header().Epoch())
						if err == nil {
							utils.Logger().Info().
								Uint64("MsgBlockNum", newBlock.NumberU64()).
								Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
								Msg("[ConsensusMainLoop] VDF has already been generated previously")
						} else {
							utils.Logger().Info().
								Uint64("MsgBlockNum", newBlock.NumberU64()).
								Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
								Msg("[ConsensusMainLoop] Generated a new VDF")
							newBlock.AddVdf(vdfOutput[:])
						}
					}
				}
			}

			startTime = time.Now()
			consensus.msgSender.Reset(newBlock.NumberU64())
			utils.Logger().Debug().
				Int("numTxs", len(newBlock.Transactions())).
				Int("numStakingTxs", len(newBlock.StakingTransactions())).
				Time("startTime", startTime).
				Int64("publicKeys", consensus.Decider.ParticipantsCount()).
				Msg("[ConsensusMainLoop] STARTING CONSENSUS")
			consensus.announce(newBlock)

		case viewID := <-consensus.commitFinishChan:
			utils.Logger().Debug().Msg("[ConsensusMainLoop] commitFinishChan")
			// Only Leader execute this condition
			go func() {
				if viewID == consensus.viewID {
					consensus.finalizeCommits()
					consensus.ReadySignal <- struct{}{}
					fmt.Println("after finalize and sent ready signal")
				}
			}()

		}
	}
	return nil
}

// GenerateVrfAndProof generates new VRF/Proof from hash of previous block
func (consensus *Consensus) GenerateVrfAndProof(
	newBlock *types.Block, vrfBlockNumbers []uint64,
) []uint64 {
	key, err := consensus.GetConsensusLeaderPrivateKey()
	if err != nil {
		utils.Logger().Error().
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

	utils.Logger().Info().
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
		utils.Logger().Warn().
			Err(err).
			Str("MsgBlockNum", headerObj.Number().String()).
			Msg("[OnAnnounce] VRF verification error")
		return false
	}

	if !bytes.Equal(hash[:], headerObj.Vrf()[:32]) {
		utils.Logger().Warn().
			Str("MsgBlockNum", headerObj.Number().String()).
			Msg("[OnAnnounce] VRF proof is not valid")
		return false
	}

	vrfBlockNumbers, _ := consensus.ChainReader.ReadEpochVrfBlockNums(
		headerObj.Epoch(),
	)
	utils.Logger().Info().
		Str("MsgBlockNum", headerObj.Number().String()).
		Int("Number of VRF", len(vrfBlockNumbers)).
		Msg("[OnAnnounce] validated a new VRF")

	return true
}

// GenerateVdfAndProof generates new VDF/Proof from VRFs in the current epoch
func (consensus *Consensus) GenerateVdfAndProof(
	newBlock *types.Block, vrfBlockNumbers []uint64,
) {
	//derive VDF seed from VRFs generated in the current epoch
	seed := [32]byte{}
	for i := 0; i < consensus.VdfSeedSize(); i++ {
		previousVrf := consensus.ChainReader.GetVrfByNumber(vrfBlockNumbers[i])
		for j := 0; j < len(seed); j++ {
			seed[j] = seed[j] ^ previousVrf[j]
		}
	}

	utils.Logger().Info().
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
		duration := time.Since(start)
		utils.Logger().Info().
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
		utils.Logger().Error().Err(err).
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
		utils.Logger().Info().
			Str("MsgBlockNum", headerObj.Number().String()).
			Int("Num of VRF", consensus.VdfSeedSize()).
			Msg("[OnAnnounce] validated a new VDF")

	} else {
		utils.Logger().Warn().
			Str("MsgBlockNum", headerObj.Number().String()).
			Uint64("Epoch", headerObj.Epoch().Uint64()).
			Int("Num of VRF", consensus.VdfSeedSize()).
			Msg("[OnAnnounce] VDF proof is not valid")
		return false
	}

	return true
}
