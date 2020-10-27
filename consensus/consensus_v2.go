package consensus

import (
	"bytes"
	"context"
	"encoding/hex"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	vrf_bls "github.com/harmony-one/harmony/crypto/vrf/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/vdf/src/vdf_go"
	"github.com/pkg/errors"
)

var (
	errSenderPubKeyNotLeader  = errors.New("sender pubkey doesn't match leader")
	errVerifyMessageSignature = errors.New("verify message signature failed")
)

// IsViewChangingMode return true if curernt mode is viewchanging
func (consensus *Consensus) IsViewChangingMode() bool {
	return consensus.current.Mode() == ViewChanging
}

// HandleMessageUpdate will update the consensus state according to received message
func (consensus *Consensus) HandleMessageUpdate(ctx context.Context, msg *msg_pb.Message, senderKey *bls.SerializedPublicKey) error {
	// when node is in ViewChanging mode, it still accepts normal messages into FBFTLog
	// in order to avoid possible trap forever but drop PREPARE and COMMIT
	// which are message types specifically for a node acting as leader
	// so we just ignore those messages
	if consensus.IsViewChangingMode() &&
		(msg.Type == msg_pb.MessageType_PREPARE ||
			msg.Type == msg_pb.MessageType_COMMIT) {
		return nil
	}

	intendedForValidator, intendedForLeader :=
		!consensus.IsLeader(),
		consensus.IsLeader()

	switch t := msg.Type; true {
	// Handle validator intended messages first
	case t == msg_pb.MessageType_ANNOUNCE && intendedForValidator:
		if !bytes.Equal(senderKey[:], consensus.LeaderPubKey.Bytes[:]) &&
			consensus.current.Mode() == Normal && !consensus.IgnoreViewIDCheck.IsSet() {
			return errSenderPubKeyNotLeader
		}
		if !consensus.senderKeySanityChecks(msg, senderKey) {
			return errVerifyMessageSignature
		}
		consensus.onAnnounce(msg)
	case t == msg_pb.MessageType_PREPARED && intendedForValidator:
		if !bytes.Equal(senderKey[:], consensus.LeaderPubKey.Bytes[:]) &&
			consensus.current.Mode() == Normal && !consensus.IgnoreViewIDCheck.IsSet() {
			return errSenderPubKeyNotLeader
		}
		if !consensus.senderKeySanityChecks(msg, senderKey) {
			return errVerifyMessageSignature
		}
		consensus.onPrepared(msg)
	case t == msg_pb.MessageType_COMMITTED && intendedForValidator:
		if !bytes.Equal(senderKey[:], consensus.LeaderPubKey.Bytes[:]) &&
			consensus.current.Mode() == Normal && !consensus.IgnoreViewIDCheck.IsSet() {
			return errSenderPubKeyNotLeader
		}
		if !consensus.senderKeySanityChecks(msg, senderKey) {
			return errVerifyMessageSignature
		}
		consensus.onCommitted(msg)

	// Handle leader intended messages now
	case t == msg_pb.MessageType_PREPARE && intendedForLeader:
		consensus.onPrepare(msg)
	case t == msg_pb.MessageType_COMMIT && intendedForLeader:
		consensus.onCommit(msg)

		// Handle view change messages
	case t == msg_pb.MessageType_VIEWCHANGE:
		if !consensus.senderKeySanityChecks(msg, senderKey) {
			return errVerifyMessageSignature
		}
		consensus.onViewChange(msg)
	case t == msg_pb.MessageType_NEWVIEW:
		if !consensus.senderKeySanityChecks(msg, senderKey) {
			return errVerifyMessageSignature
		}
		consensus.onNewView(msg)
	}

	return nil
}

func (consensus *Consensus) finalCommit() {
	consensus.getLogger().Info().
		Int64("NumCommits", consensus.Decider.SignersCount(quorum.Commit)).
		Msg("[finalCommit] Finalizing Consensus")
	beforeCatchupNum := consensus.blockNum

	leaderPriKey, err := consensus.GetConsensusLeaderPrivateKey()
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("[finalCommit] leader not found")
		return
	}
	// Construct committed message
	network, err := consensus.construct(msg_pb.MessageType_COMMITTED, nil, []*bls.PrivateKeyWrapper{leaderPriKey})
	if err != nil {
		consensus.getLogger().Warn().Err(err).
			Msg("[finalCommit] Unable to construct Committed message")
		return
	}
	msgToSend, FBFTMsg :=
		network.Bytes,
		network.FBFTMsg
	commitSigAndBitmap := FBFTMsg.Payload // this may not needed
	consensus.FBFTLog.AddMessage(FBFTMsg)
	// find correct block content
	curBlockHash := consensus.blockHash
	block := consensus.FBFTLog.GetBlockByHash(curBlockHash)
	if block == nil {
		consensus.getLogger().Warn().
			Str("blockHash", hex.EncodeToString(curBlockHash[:])).
			Msg("[finalCommit] Cannot find block by hash")
		return
	}

	consensus.commitBlock(block, FBFTMsg)

	if consensus.blockNum-beforeCatchupNum != 1 {
		consensus.getLogger().Warn().
			Uint64("beforeCatchupBlockNum", beforeCatchupNum).
			Msg("[finalCommit] Leader cannot provide the correct block for committed message")
		return
	}

	// if leader success finalize the block, send committed message to validators
	// TODO: once leader rotation is implemented, leader who is about to be switched out
	//       needs to send the committed message immediately so the next leader can
	//       have the full commit signatures for new block
	// For now, the leader don't need to send immediately as the committed sig will be
	// included in the next block and sent in next prepared message.
	sendImmediately := false
	if err := consensus.msgSender.SendWithRetry(
		block.NumberU64(),
		msg_pb.MessageType_COMMITTED, []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
		p2p.ConstructMessage(msgToSend), sendImmediately); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[finalCommit] Cannot send committed message")
	} else {
		consensus.getLogger().Info().
			Hex("blockHash", curBlockHash[:]).
			Uint64("blockNum", consensus.blockNum).
			Msg("[finalCommit] Sent Committed Message")
	}

	// Dump new block into level db
	// In current code, we add signatures in block in tryCatchup, the block dump to explorer does not contains signatures
	// but since explorer doesn't need signatures, it should be fine
	// in future, we will move signatures to next block
	//explorer.GetStorageInstance(consensus.leader.IP, consensus.leader.Port, true).Dump(block, beforeCatchupNum)

	if consensus.consensusTimeout[timeoutBootstrap].IsActive() {
		consensus.consensusTimeout[timeoutBootstrap].Stop()
		consensus.getLogger().Info().Msg("[finalCommit] Start consensus timer; stop bootstrap timer only once")
	} else {
		consensus.getLogger().Info().Msg("[finalCommit] Start consensus timer")
	}
	consensus.consensusTimeout[timeoutConsensus].Start()

	consensus.getLogger().Info().
		Uint64("blockNum", block.NumberU64()).
		Uint64("epochNum", block.Epoch().Uint64()).
		Uint64("ViewId", block.Header().ViewID().Uint64()).
		Str("blockHash", block.Hash().String()).
		Int("numTxns", len(block.Transactions())).
		Int("numStakingTxns", len(block.StakingTransactions())).
		Msg("HOORAY!!!!!!! CONSENSUS REACHED!!!!!!!")

	// Sleep to wait for the full block time
	consensus.getLogger().Info().Msg("[finalCommit] Waiting for Block Time")
	<-time.After(time.Until(consensus.NextBlockDue))

	// Send commit sig/bitmap to finish the new block proposal
	consensus.CommitSigChannel <- commitSigAndBitmap

	// Update time due for next block
	consensus.NextBlockDue = time.Now().Add(consensus.BlockPeriod)
}

// BlockCommitSigs returns the byte array of aggregated
// commit signature and bitmap signed on the block
func (consensus *Consensus) BlockCommitSigs(blockNum uint64) ([]byte, error) {
	if consensus.blockNum <= 1 {
		return nil, nil
	}
	lastCommits, err := consensus.Blockchain.ReadCommitSig(blockNum)
	if err != nil ||
		len(lastCommits) < bls.BLSSignatureSizeInBytes {
		msgs := consensus.FBFTLog.GetMessagesByTypeSeq(
			msg_pb.MessageType_COMMITTED, blockNum,
		)
		if len(msgs) != 1 {
			consensus.getLogger().Error().
				Int("numCommittedMsg", len(msgs)).
				Msg("GetLastCommitSig failed with wrong number of committed message")
			return nil, errors.Errorf(
				"GetLastCommitSig failed with wrong number of committed message %d", len(msgs),
			)
		}
		lastCommits = msgs[0].Payload
	}

	return lastCommits, nil
}

// Start waits for the next new block and run consensus
func (consensus *Consensus) Start(
	blockChannel chan *types.Block, stopChan, stoppedChan, startChannel chan struct{},
) {
	go func() {
		toStart := make(chan struct{}, 1)
		isInitialLeader := consensus.IsLeader()
		if isInitialLeader {
			consensus.getLogger().Info().Time("time", time.Now()).Msg("[ConsensusMainLoop] Waiting for consensus start")
			// send a signal to indicate it's ready to run consensus
			// this signal is consumed by node object to create a new block and in turn trigger a new consensus on it
			go func() {
				<-startChannel
				toStart <- struct{}{}
				consensus.getLogger().Info().Time("time", time.Now()).Msg("[ConsensusMainLoop] Send ReadySignal")
				consensus.ReadySignal <- struct{}{}
			}()
		}
		consensus.getLogger().Info().Time("time", time.Now()).Msg("[ConsensusMainLoop] Consensus started")
		defer close(stoppedChan)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		consensus.consensusTimeout[timeoutBootstrap].Start()
		consensus.getLogger().Info().Msg("[ConsensusMainLoop] Start bootstrap timeout (only once)")

		vdfInProgress := false
		// Set up next block due time.
		consensus.NextBlockDue = time.Now().Add(consensus.BlockPeriod)
		start := false
		for {
			select {
			case <-toStart:
				start = true
			case <-ticker.C:
				consensus.getLogger().Debug().Msg("[ConsensusMainLoop] Ticker")
				if !start && isInitialLeader {
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
						consensus.getLogger().Warn().Msg("[ConsensusMainLoop] Ops Consensus Timeout!!!")
						consensus.startViewChange()
						break
					} else {
						consensus.getLogger().Warn().Msg("[ConsensusMainLoop] Ops View Change Timeout!!!")
						consensus.startViewChange()
						break
					}
				}
			case <-consensus.syncReadyChan:
				consensus.getLogger().Info().Msg("[ConsensusMainLoop] syncReadyChan")
				consensus.SetBlockNum(consensus.Blockchain.CurrentHeader().Number().Uint64() + 1)
				consensus.SetViewIDs(consensus.Blockchain.CurrentHeader().ViewID().Uint64() + 1)
				mode := consensus.UpdateConsensusInformation()
				consensus.current.SetMode(mode)
				consensus.getLogger().Info().Str("Mode", mode.String()).Msg("Node is IN SYNC")

			case <-consensus.syncNotReadyChan:
				consensus.getLogger().Info().Msg("[ConsensusMainLoop] syncNotReadyChan")
				consensus.SetBlockNum(consensus.Blockchain.CurrentHeader().Number().Uint64() + 1)
				consensus.current.SetMode(Syncing)
				consensus.getLogger().Info().Msg("[ConsensusMainLoop] Node is OUT OF SYNC")

			case newBlock := <-blockChannel:
				consensus.getLogger().Info().
					Uint64("MsgBlockNum", newBlock.NumberU64()).
					Msg("[ConsensusMainLoop] Received Proposed New Block!")

				//VRF/VDF is only generated in the beacon chain
				if consensus.NeedsRandomNumberGeneration(newBlock.Header().Epoch()) {
					// generate VRF if the current block has a new leader
					if !consensus.Blockchain.IsSameLeaderAsPreviousBlock(newBlock) {
						vrfBlockNumbers, err := consensus.Blockchain.ReadEpochVrfBlockNums(newBlock.Header().Epoch())
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
								_, err := consensus.Blockchain.ReadEpochVdfBlockNum(newBlock.Header().Epoch())
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
							_, err := consensus.Blockchain.ReadEpochVdfBlockNum(newBlock.Header().Epoch())
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
					}
				}

				startTime = time.Now()
				consensus.msgSender.Reset(newBlock.NumberU64())

				consensus.getLogger().Info().
					Int("numTxs", len(newBlock.Transactions())).
					Int("numStakingTxs", len(newBlock.StakingTransactions())).
					Time("startTime", startTime).
					Int64("publicKeys", consensus.Decider.ParticipantsCount()).
					Msg("[ConsensusMainLoop] STARTING CONSENSUS")
				consensus.announce(newBlock)

			case viewID := <-consensus.commitFinishChan:
				consensus.getLogger().Info().Msg("[ConsensusMainLoop] commitFinishChan")

				// Only Leader execute this condition
				func() {
					consensus.mutex.Lock()
					defer consensus.mutex.Unlock()
					if viewID == consensus.GetCurBlockViewID() {
						consensus.finalCommit()
					}
				}()

			case <-stopChan:
				consensus.getLogger().Info().Msg("[ConsensusMainLoop] stopChan")
				return
			}
		}
		consensus.getLogger().Info().Msg("[ConsensusMainLoop] Ended.")
	}()
}

// LastMileBlockIter is the iterator to iterate over the last mile blocks in consensus cache.
// All blocks returned are guaranteed to pass the verification.
type LastMileBlockIter struct {
	blockCandidates []*types.Block
	fbftLog         FBFTLog
	verify          func(*types.Block) error
	curIndex        int
	logger          *zerolog.Logger
}

// GetLastMileBlockIter get the iterator of the last mile blocks starting from number bnStart
func (consensus *Consensus) GetLastMileBlockIter(bnStart uint64) (*LastMileBlockIter, error) {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	if consensus.BlockVerifier == nil {
		return nil, errors.New("consensus haven't initialized yet")
	}
	blocks, _, err := consensus.getLastMileBlocksAndMsg(bnStart)
	if err != nil {
		return nil, err
	}
	return &LastMileBlockIter{
		blockCandidates: blocks,
		verify:          consensus.BlockVerifier,
		curIndex:        0,
		logger:          consensus.getLogger(),
	}, nil
}

// Next iterate to the next last mile block
func (iter *LastMileBlockIter) Next() *types.Block {
	if iter.curIndex >= len(iter.blockCandidates) {
		return nil
	}
	block := iter.blockCandidates[iter.curIndex]
	iter.curIndex++

	if !iter.fbftLog.IsBlockVerified(block) {
		if err := iter.verify(block); err != nil {
			iter.logger.Debug().Err(err).Msg("block verification failed in consensus last mile block")
			return nil
		}
		iter.fbftLog.MarkBlockVerified(block)
	}
	return block
}

func (consensus *Consensus) getLastMileBlocksAndMsg(bnStart uint64) ([]*types.Block, []*FBFTMessage, error) {
	var (
		blocks []*types.Block
		msgs   []*FBFTMessage
	)
	for blockNum := bnStart; ; blockNum++ {
		blk, msg, err := consensus.FBFTLog.GetCommittedBlockAndMsgsFromNumber(blockNum, consensus.getLogger())
		if err != nil {
			if err == errFBFTLogNotFound {
				break
			}
			return nil, nil, err
		}
		blocks = append(blocks, blk)
		msgs = append(msgs, msg)
	}
	return blocks, msgs, nil
}

// preCommitAndPropose commit the current block with 67% commit signatures and start
// proposing new block which will wait on the full commit signatures to finish
func (consensus *Consensus) preCommitAndPropose(blk *types.Block) error {
	if blk == nil {
		return errors.New("block to pre-commit is nil")
	}

	leaderPriKey, err := consensus.GetConsensusLeaderPrivateKey()
	if err != nil {
		return err
	}

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	network, err := consensus.construct(msg_pb.MessageType_COMMITTED, nil, []*bls.PrivateKeyWrapper{leaderPriKey})
	if err != nil {
		return errors.Wrap(err, "[preCommitAndPropose] Unable to construct Committed message")
	}

	msgToSend, FBFTMsg :=
		network.Bytes,
		network.FBFTMsg
	consensus.FBFTLog.AddMessage(FBFTMsg)

	blk.SetCurrentCommitSig(FBFTMsg.Payload)
	if err := consensus.OnConsensusDone(blk); err != nil {
		consensus.getLogger().Error().Err(err).Msg("[preCommitAndPropose] Failed to add block to chain")
		return err
	}

	// if leader success finalize the block, send committed message to validators
	if err := consensus.msgSender.SendWithRetry(
		blk.NumberU64(),
		msg_pb.MessageType_COMMITTED, []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
		p2p.ConstructMessage(msgToSend), true); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[preCommitAndPropose] Cannot send committed message")
	} else {
		consensus.getLogger().Info().
			Str("blockHash", blk.Hash().Hex()).
			Uint64("blockNum", consensus.blockNum).
			Msg("[preCommitAndPropose] Sent Committed Message")
	}

	// Send signal to Node to propose the new block for consensus
	consensus.ReadySignal <- struct{}{}
	return nil
}

// tryCatchup add the last mile block in PBFT log memory cache to blockchain.
func (consensus *Consensus) tryCatchup() error {
	// TODO: change this to a more systematic symbol
	if consensus.BlockVerifier == nil {
		return errors.New("consensus haven't finished initialization")
	}
	initBN := consensus.blockNum
	defer consensus.postCatchup(initBN)

	blks, msgs, err := consensus.getLastMileBlocksAndMsg(initBN)
	if err != nil {
		return errors.Wrapf(err, "[TryCatchup] Failed to get last mile blocks: %v", err)
	}
	for i := range blks {
		blk, msg := blks[i], msgs[i]
		if blk == nil {
			return nil
		}
		blk.SetCurrentCommitSig(msg.Payload)

		if !consensus.FBFTLog.IsBlockVerified(blk) {
			if err := consensus.BlockVerifier(blk); err != nil {
				consensus.getLogger().Err(err).Msg("[TryCatchup] failed block verifier")
				return err
			}
			consensus.FBFTLog.MarkBlockVerified(blk)
		}
		consensus.getLogger().Info().Msg("[TryCatchup] Adding block to chain")
		if err := consensus.commitBlock(blk, msgs[i]); err != nil {
			consensus.getLogger().Error().Err(err).Msg("[TryCatchup] Failed to add block to chain")
			return err
		}
		select {
		case consensus.VerifiedNewBlock <- blk:
		default:
			consensus.getLogger().Info().
				Str("blockHash", blk.Hash().String()).
				Msg("[TryCatchup] consensus verified block send to chan failed")
			continue
		}
	}
	return nil
}

func (consensus *Consensus) commitBlock(blk *types.Block, committedMsg *FBFTMessage) error {
	if consensus.Blockchain.CurrentBlock().NumberU64() < blk.NumberU64() {
		if err := consensus.OnConsensusDone(blk); err != nil {
			return err
		}
	}

	if !committedMsg.HasSingleSender() {
		consensus.getLogger().Error().Msg("[TryCatchup] Leader message can not have multiple sender keys")
		return errIncorrectSender
	}

	consensus.SetupForNewConsensus(blk, committedMsg)
	return nil
}

func (consensus *Consensus) SetupForNewConsensus(blk *types.Block, committedMsg *FBFTMessage) {
	atomic.AddUint64(&consensus.blockNum, 1)
	consensus.SetCurBlockViewID(committedMsg.ViewID + 1)
	consensus.LeaderPubKey = committedMsg.SenderPubkeys[0]
	// Update consensus keys at last so the change of leader status doesn't mess up normal flow
	if blk.IsLastBlockInEpoch() {
		consensus.SetMode(consensus.UpdateConsensusInformation())
	}
	consensus.ResetState()
}

func (consensus *Consensus) postCatchup(initBN uint64) {
	if initBN < consensus.blockNum {
		consensus.getLogger().Info().
			Uint64("From", initBN).
			Uint64("To", consensus.blockNum).
			Msg("[TryCatchup] Caught up!")
		consensus.switchPhase("TryCatchup", FBFTAnnounce)
	}
	// catch up and skip from view change trap
	if initBN < consensus.blockNum && consensus.IsViewChangingMode() {
		consensus.current.SetMode(Normal)
		consensus.consensusTimeout[timeoutViewChange].Stop()
	}
	// clean up old log
	consensus.FBFTLog.PruneCacheBeforeBlock(consensus.blockNum)
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
	sk := vrf_bls.NewVRFSigner(key.Pri)
	blockHash := [32]byte{}
	previousHeader := consensus.Blockchain.GetHeaderByNumber(
		newBlock.NumberU64() - 1,
	)
	if previousHeader == nil {
		return vrfBlockNumbers
	}
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
	vrfPk := vrf_bls.NewVRFVerifier(consensus.LeaderPubKey.Object)
	var blockHash [32]byte
	previousHeader := consensus.Blockchain.GetHeaderByNumber(
		headerObj.Number().Uint64() - 1,
	)
	if previousHeader == nil {
		return false
	}

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

	vrfBlockNumbers, _ := consensus.Blockchain.ReadEpochVrfBlockNums(
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
		previousVrf := consensus.Blockchain.GetVrfByNumber(vrfBlockNumbers[i])
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
		duration := time.Since(start)
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
	vrfBlockNumbers, err := consensus.Blockchain.ReadEpochVrfBlockNums(headerObj.Epoch())
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
		previousVrf := consensus.Blockchain.GetVrfByNumber(vrfBlockNumbers[i])
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
