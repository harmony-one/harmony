package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"sync"
	"time"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// MaxViewIDDiff limits the received view ID to only 100 further from the current view ID
const MaxViewIDDiff = 100

// State contains current mode and current viewID
type State struct {
	mode   Mode
	viewID uint64
	mux    sync.Mutex
}

// Mode return the current node mode
func (pm *State) Mode() Mode {
	return pm.mode
}

// SetMode set the node mode as required
func (pm *State) SetMode(s Mode) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.mode = s
}

// ViewID return the current viewchanging id
func (pm *State) ViewID() uint64 {
	return pm.viewID
}

// SetViewID sets the viewchanging id accordingly
func (pm *State) SetViewID(viewID uint64) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.viewID = viewID
}

// GetViewID returns the current viewchange viewID
func (pm *State) GetViewID() uint64 {
	return pm.viewID
}

// switchPhase will switch FBFTPhase to nextPhase if the desirePhase equals the nextPhase
func (consensus *Consensus) switchPhase(desired FBFTPhase, override bool) {
	if override {
		consensus.phase = desired
		return
	}

	var nextPhase FBFTPhase
	switch consensus.phase {
	case FBFTAnnounce:
		nextPhase = FBFTPrepare
	case FBFTPrepare:
		nextPhase = FBFTCommit
	case FBFTCommit:
		nextPhase = FBFTAnnounce
	}
	if nextPhase == desired {
		consensus.phase = nextPhase
	}
}

// GetNextLeaderKey uniquely determine who is the leader for given viewID
func (consensus *Consensus) GetNextLeaderKey() *bls.PublicKeyWrapper {
	wasFound, next := consensus.Decider.NextAfter(consensus.LeaderPubKey)
	if !wasFound {
		consensus.getLogger().Warn().
			Str("key", consensus.LeaderPubKey.Bytes.Hex()).
			Msg("GetNextLeaderKey: currentLeaderKey not found")
	}
	return next
}

// ResetViewChangeState reset the state for viewchange
func (consensus *Consensus) ResetViewChangeState() {
	consensus.getLogger().Debug().
		Str("Phase", consensus.phase.String()).
		Msg("[ResetViewChangeState] Resetting view change state")
	consensus.current.SetMode(Normal)
	consensus.m1Payload = []byte{}
	consensus.bhpSigs = map[uint64]map[string]*bls_core.Sign{}
	consensus.nilSigs = map[uint64]map[string]*bls_core.Sign{}
	consensus.viewIDSigs = map[uint64]map[string]*bls_core.Sign{}
	consensus.bhpBitmap = map[uint64]*bls_cosi.Mask{}
	consensus.nilBitmap = map[uint64]*bls_cosi.Mask{}
	consensus.viewIDBitmap = map[uint64]*bls_cosi.Mask{}
	consensus.Decider.ResetViewChangeVotes()
}

func createTimeout() map[TimeoutType]*utils.Timeout {
	timeouts := make(map[TimeoutType]*utils.Timeout)
	timeouts[timeoutConsensus] = utils.NewTimeout(phaseDuration)
	timeouts[timeoutViewChange] = utils.NewTimeout(viewChangeDuration)
	timeouts[timeoutBootstrap] = utils.NewTimeout(bootstrapDuration)
	return timeouts
}

// startViewChange send a  new view change
func (consensus *Consensus) startViewChange(viewID uint64) {
	if consensus.disableViewChange {
		return
	}
	consensus.consensusTimeout[timeoutConsensus].Stop()
	consensus.consensusTimeout[timeoutBootstrap].Stop()
	consensus.current.SetMode(ViewChanging)
	consensus.current.SetViewID(viewID)
	consensus.LeaderPubKey = consensus.GetNextLeaderKey()

	diff := int64(viewID - consensus.viewID)
	duration := time.Duration(diff * diff * int64(viewChangeDuration))
	consensus.getLogger().Info().
		Uint64("ViewChangingID", viewID).
		Dur("timeoutDuration", duration).
		Str("NextLeader", consensus.LeaderPubKey.Bytes.Hex()).
		Msg("[startViewChange]")

	for _, key := range consensus.priKey {
		if !consensus.IsValidatorInCommittee(key.Pub.Bytes) {
			continue
		}
		msgToSend := consensus.constructViewChangeMessage(&key)
		consensus.host.SendMessageToGroups([]nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
			p2p.ConstructMessage(msgToSend),
		)
	}

	consensus.consensusTimeout[timeoutViewChange].SetDuration(duration)
	consensus.consensusTimeout[timeoutViewChange].Start()
	consensus.getLogger().Info().
		Uint64("ViewChangingID", consensus.current.ViewID()).
		Msg("[startViewChange] start view change timer")
}

func (consensus *Consensus) onViewChange(msg *msg_pb.Message) {
	recvMsg, err := ParseViewChangeMessage(msg)
	if err != nil {
		consensus.getLogger().Warn().Msg("[onViewChange] Unable To Parse Viewchange Message")
		return
	}
	// if not leader, noop
	newLeaderKey := recvMsg.LeaderPubkey
	newLeaderPriKey, err := consensus.GetLeaderPrivateKey(newLeaderKey.Object)
	if err != nil {
		return
	}

	if consensus.Decider.IsQuorumAchieved(quorum.ViewChange) {
		consensus.getLogger().Info().
			Int64("have", consensus.Decider.SignersCount(quorum.ViewChange)).
			Int64("need", consensus.Decider.TwoThirdsSignersCount()).
			Str("validatorPubKey", recvMsg.SenderPubkey.Bytes.Hex()).
			Msg("[onViewChange] Received Enough View Change Messages")
		return
	}

	if !consensus.onViewChangeSanityCheck(recvMsg) {
		return
	}

	senderKey := recvMsg.SenderPubkey

	consensus.vcLock.Lock()
	defer consensus.vcLock.Unlock()

	// update the dictionary key if the viewID is first time received
	consensus.addViewIDKeyIfNotExist(recvMsg.ViewID)

	// TODO: remove NIL type message
	// add self m1 or m2 type message signature and bitmap
	_, ok1 := consensus.nilSigs[recvMsg.ViewID][newLeaderKey.Bytes.Hex()]
	_, ok2 := consensus.bhpSigs[recvMsg.ViewID][newLeaderKey.Bytes.Hex()]
	if !(ok1 || ok2) {
		// add own signature for newview message
		preparedMsgs := consensus.FBFTLog.GetMessagesByTypeSeq(
			msg_pb.MessageType_PREPARED, recvMsg.BlockNum,
		)
		preparedMsg := consensus.FBFTLog.FindMessageByMaxViewID(preparedMsgs)
		hasBlock := false
		if preparedMsg != nil {
			if preparedBlock := consensus.FBFTLog.GetBlockByHash(
				preparedMsg.BlockHash,
			); preparedBlock != nil {
				if consensus.BlockVerifier(preparedBlock); err != nil {
					consensus.getLogger().Error().Err(err).Msg("[onViewChange] My own prepared block verification failed")
				} else {
					hasBlock = true
				}
			}
		}
		if hasBlock {
			consensus.getLogger().Info().Msg("[onViewChange] add my M1 type messaage")
			msgToSign := append(preparedMsg.BlockHash[:], preparedMsg.Payload...)
			for i, key := range consensus.priKey {
				if err := consensus.bhpBitmap[recvMsg.ViewID].SetKey(key.Pub.Bytes, true); err != nil {
					consensus.getLogger().Warn().Msgf("[onViewChange] bhpBitmap setkey failed for key at index %d", i)
					continue
				}
				consensus.bhpSigs[recvMsg.ViewID][key.Pub.Bytes.Hex()] = key.Pri.SignHash(msgToSign)
			}
			// if m1Payload is empty, we just add one
			if len(consensus.m1Payload) == 0 {
				consensus.m1Payload = append(preparedMsg.BlockHash[:], preparedMsg.Payload...)
			}
		} else {
			consensus.getLogger().Info().Msg("[onViewChange] add my M2(NIL) type messaage")
			for i, key := range consensus.priKey {
				if err := consensus.nilBitmap[recvMsg.ViewID].SetKey(key.Pub.Bytes, true); err != nil {
					consensus.getLogger().Warn().Msgf("[onViewChange] nilBitmap setkey failed for key at index %d", i)
					continue
				}
				consensus.nilSigs[recvMsg.ViewID][key.Pub.Bytes.Hex()] = key.Pri.SignHash(NIL)
			}
		}
	}
	// add self m3 type message signature and bitmap
	_, ok3 := consensus.viewIDSigs[recvMsg.ViewID][newLeaderKey.Bytes.Hex()]
	if !ok3 {
		viewIDBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(viewIDBytes, recvMsg.ViewID)
		for i, key := range consensus.priKey {
			if err := consensus.viewIDBitmap[recvMsg.ViewID].SetKey(key.Pub.Bytes, true); err != nil {
				consensus.getLogger().Warn().Msgf("[onViewChange] viewIDBitmap setkey failed for key at index %d", i)
				continue
			}
			consensus.viewIDSigs[recvMsg.ViewID][key.Pub.Bytes.Hex()] = key.Pri.SignHash(viewIDBytes)
		}
	}

	preparedBlock := &types.Block{}
	hasBlock := false
	if len(recvMsg.Payload) != 0 && len(recvMsg.Block) != 0 {
		if err := rlp.DecodeBytes(recvMsg.Block, preparedBlock); err != nil {
			consensus.getLogger().Warn().
				Err(err).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg("[onViewChange] Unparseable prepared block data")
			return
		}
		hasBlock = true
	}

	// m2 type message
	if !hasBlock {
		_, ok := consensus.nilSigs[recvMsg.ViewID][senderKey.Bytes.Hex()]
		if ok {
			consensus.getLogger().Debug().
				Str("validatorPubKey", senderKey.Bytes.Hex()).
				Msg("[onViewChange] Already Received M2 message from validator")
			return
		}

		if !recvMsg.ViewchangeSig.VerifyHash(senderKey.Object, NIL) {
			consensus.getLogger().Warn().Msg("[onViewChange] Failed To Verify Signature For M2 Type Viewchange Message")
			return
		}

		consensus.getLogger().Info().
			Str("validatorPubKey", senderKey.Bytes.Hex()).
			Msg("[onViewChange] Add M2 (NIL) type message")
		consensus.nilSigs[recvMsg.ViewID][senderKey.Bytes.Hex()] = recvMsg.ViewchangeSig
		consensus.nilBitmap[recvMsg.ViewID].SetKey(recvMsg.SenderPubkey.Bytes, true) // Set the bitmap indicating that this validator signed.
	} else { // m1 type message
		if consensus.BlockVerifier(preparedBlock); err != nil {
			consensus.getLogger().Error().Err(err).Msg("[onViewChange] Prepared block verification failed")
			return
		}
		_, ok := consensus.bhpSigs[recvMsg.ViewID][senderKey.Bytes.Hex()]
		if ok {
			consensus.getLogger().Debug().
				Str("validatorPubKey", senderKey.Bytes.Hex()).
				Msg("[onViewChange] Already Received M1 Message From the Validator")
			return
		}
		if !recvMsg.ViewchangeSig.VerifyHash(recvMsg.SenderPubkey.Object, recvMsg.Payload) {
			consensus.getLogger().Warn().Msg("[onViewChange] Failed to Verify Signature for M1 Type Viewchange Message")
			return
		}

		// first time receive m1 type message, need verify validity of prepared message
		if len(consensus.m1Payload) == 0 || !bytes.Equal(consensus.m1Payload, recvMsg.Payload) {
			if len(recvMsg.Payload) <= 32 {
				consensus.getLogger().Warn().
					Int("len", len(recvMsg.Payload)).
					Msg("[onViewChange] M1 RecvMsg Payload Not Enough Length")
				return
			}
			blockHash := recvMsg.Payload[:32]
			aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)
			if err != nil {
				consensus.getLogger().Error().Err(err).Msg("[onViewChange] M1 RecvMsg Payload Read Error")
				return
			}

			if !consensus.Decider.IsQuorumAchievedByMask(mask) {
				consensus.getLogger().Warn().
					Msgf("[onViewChange] Quorum Not achieved")
				return
			}

			// Verify the multi-sig for prepare phase
			if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
				consensus.getLogger().Warn().
					Hex("blockHash", blockHash).
					Msg("[onViewChange] failed to verify multi signature for m1 prepared payload")
				return
			}

			// if m1Payload is empty, we just add one
			if len(consensus.m1Payload) == 0 {
				consensus.m1Payload = append(recvMsg.Payload[:0:0], recvMsg.Payload...)
				// create prepared message for new leader
				preparedMsg := FBFTMessage{
					MessageType: msg_pb.MessageType_PREPARED,
					ViewID:      recvMsg.ViewID,
					BlockNum:    recvMsg.BlockNum,
				}
				preparedMsg.BlockHash = common.Hash{}
				copy(preparedMsg.BlockHash[:], recvMsg.Payload[:32])
				preparedMsg.Payload = make([]byte, len(recvMsg.Payload)-32)
				copy(preparedMsg.Payload[:], recvMsg.Payload[32:])
				preparedMsg.SenderPubkey = newLeaderKey
				consensus.getLogger().Info().Msg("[onViewChange] New Leader Prepared Message Added")
				consensus.FBFTLog.AddMessage(&preparedMsg)

				consensus.FBFTLog.AddBlock(preparedBlock)
			}
		}
		consensus.getLogger().Info().
			Str("validatorPubKey", senderKey.Bytes.Hex()).
			Msg("[onViewChange] Add M1 (prepared) type message")
		consensus.bhpSigs[recvMsg.ViewID][senderKey.Bytes.Hex()] = recvMsg.ViewchangeSig
		consensus.bhpBitmap[recvMsg.ViewID].SetKey(recvMsg.SenderPubkey.Bytes, true) // Set the bitmap indicating that this validator signed.
	}

	// check and add viewID (m3 type) message signature
	if _, ok := consensus.viewIDSigs[recvMsg.ViewID][senderKey.Bytes.Hex()]; ok {
		consensus.getLogger().Debug().
			Str("validatorPubKey", senderKey.Bytes.Hex()).
			Msg("[onViewChange] Already Received M3(ViewID) message from the validator")
		return
	}
	viewIDHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDHash, recvMsg.ViewID)
	if !recvMsg.ViewidSig.VerifyHash(recvMsg.SenderPubkey.Object, viewIDHash) {
		consensus.getLogger().Warn().
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[onViewChange] Failed to Verify M3 Message Signature")
		return
	}
	consensus.getLogger().Info().
		Str("validatorPubKey", senderKey.Bytes.Hex()).
		Msg("[onViewChange] Add M3 (ViewID) type message")

	consensus.viewIDSigs[recvMsg.ViewID][senderKey.Bytes.Hex()] = recvMsg.ViewidSig
	// Set the bitmap indicating that this validator signed.
	consensus.viewIDBitmap[recvMsg.ViewID].SetKey(recvMsg.SenderPubkey.Bytes, true)
	consensus.getLogger().Info().
		Int("have", len(consensus.viewIDSigs[recvMsg.ViewID])).
		Int64("total", consensus.Decider.ParticipantsCount()).
		Msg("[onViewChange]")

	// received enough view change messages, change state to normal consensus
	if consensus.Decider.IsQuorumAchievedByMask(consensus.viewIDBitmap[recvMsg.ViewID]) {
		consensus.current.SetMode(Normal)
		consensus.LeaderPubKey = newLeaderKey
		consensus.ResetState()
		if len(consensus.m1Payload) == 0 {
			// TODO(Chao): explain why ReadySignal is sent only in this case but not the other case.
			// Make sure the newly proposed block have the correct view ID
			consensus.viewID = recvMsg.ViewID
			go func() {
				consensus.ReadySignal <- struct{}{}
			}()
		} else {
			consensus.getLogger().Info().
				Str("From", consensus.phase.String()).
				Str("To", FBFTCommit.String()).
				Msg("[OnViewChange] Switching phase")
			consensus.switchPhase(FBFTCommit, true)
			copy(consensus.blockHash[:], consensus.m1Payload[:32])
			aggSig, mask, err := consensus.ReadSignatureBitmapPayload(consensus.m1Payload, 32)

			if err != nil {
				consensus.getLogger().Error().Err(err).
					Msg("[onViewChange] ReadSignatureBitmapPayload Fail")
				return
			}

			consensus.aggregatedPrepareSig = aggSig
			consensus.prepareBitmap = mask
			// Leader sign and add commit message
			block := consensus.FBFTLog.GetBlockByHash(consensus.blockHash)
			if block == nil {
				consensus.getLogger().Warn().Msg("[onViewChange] failed to get prepared block for self commit")
				return
			}
			commitPayload := signature.ConstructCommitPayload(consensus.ChainReader,
				block.Epoch(), block.Hash(), block.NumberU64(), block.Header().ViewID().Uint64())
			for i, key := range consensus.priKey {
				if err := consensus.commitBitmap.SetKey(key.Pub.Bytes, true); err != nil {
					consensus.getLogger().Warn().
						Msgf("[OnViewChange] New Leader commit bitmap set failed for key at index %d", i)
					continue
				}

				if _, err := consensus.Decider.SubmitVote(
					quorum.Commit,
					key.Pub.Bytes,
					key.Pri.SignHash(commitPayload),
					common.BytesToHash(consensus.blockHash[:]),
					block.NumberU64(),
					block.Header().ViewID().Uint64(),
				); err != nil {
					consensus.getLogger().Warn().Msg("submit vote on viewchange commit failed")
					return
				}

			}
		}

		consensus.current.SetViewID(recvMsg.ViewID)
		msgToSend := consensus.constructNewViewMessage(
			recvMsg.ViewID, newLeaderPriKey,
		)

		consensus.getLogger().Warn().
			Int("payloadSize", len(consensus.m1Payload)).
			Hex("M1Payload", consensus.m1Payload).
			Msg("[onViewChange] Sent NewView Message")
		if err := consensus.msgSender.SendWithRetry(
			consensus.blockNum,
			msg_pb.MessageType_NEWVIEW,
			[]nodeconfig.GroupID{
				nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))},
			p2p.ConstructMessage(msgToSend),
		); err != nil {
			consensus.getLogger().Err(err).
				Msg("could not send out the NEWVIEW message")
		}

		consensus.viewID = recvMsg.ViewID
		consensus.ResetViewChangeState()
		consensus.consensusTimeout[timeoutViewChange].Stop()
		consensus.consensusTimeout[timeoutConsensus].Start()
		consensus.getLogger().Debug().
			Uint64("viewChangingID", consensus.current.ViewID()).
			Msg("[onViewChange] New Leader Start Consensus Timer and Stop View Change Timer")
		consensus.getLogger().Info().
			Str("myKey", newLeaderKey.Bytes.Hex()).
			Uint64("viewID", consensus.viewID).
			Uint64("block", consensus.blockNum).
			Msg("[onViewChange] I am the New Leader")
	}
}

// TODO: move to consensus_leader.go later
func (consensus *Consensus) onNewView(msg *msg_pb.Message) {
	consensus.getLogger().Info().Msg("[onNewView] Received NewView Message")
	recvMsg, err := consensus.ParseNewViewMessage(msg)
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[onNewView] Unable to Parse NewView Message")
		return
	}

	if !consensus.onNewViewSanityCheck(recvMsg) {
		return
	}

	senderKey := recvMsg.SenderPubkey
	consensus.vcLock.Lock()
	defer consensus.vcLock.Unlock()

	if recvMsg.M3AggSig == nil || recvMsg.M3Bitmap == nil {
		consensus.getLogger().Error().Msg("[onNewView] M3AggSig or M3Bitmap is nil")
		return
	}
	m3Sig := recvMsg.M3AggSig
	m3Mask := recvMsg.M3Bitmap

	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, recvMsg.ViewID)

	if !consensus.Decider.IsQuorumAchievedByMask(m3Mask) {
		consensus.getLogger().Warn().
			Msgf("[onNewView] Quorum Not achieved")
		return
	}

	if !m3Sig.VerifyHash(m3Mask.AggregatePublic, viewIDBytes) {
		consensus.getLogger().Warn().
			Str("m3Sig", m3Sig.SerializeToHexStr()).
			Hex("m3Mask", m3Mask.Bitmap).
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[onNewView] Unable to Verify Aggregated Signature of M3 (ViewID) payload")
		return
	}

	m2Mask := recvMsg.M2Bitmap
	if recvMsg.M2AggSig != nil {
		consensus.getLogger().Info().Msg("[onNewView] M2AggSig (NIL) is Not Empty")
		m2Sig := recvMsg.M2AggSig
		if !m2Sig.VerifyHash(m2Mask.AggregatePublic, NIL) {
			consensus.getLogger().Warn().
				Msg("[onNewView] Unable to Verify Aggregated Signature of M2 (NIL) payload")
			return
		}
	}

	// check when M3 sigs > M2 sigs, then M1 (recvMsg.Payload) should not be empty
	preparedBlock := &types.Block{}
	hasBlock := false
	if len(recvMsg.Payload) != 0 && len(recvMsg.Block) != 0 {
		if err := rlp.DecodeBytes(recvMsg.Block, preparedBlock); err != nil {
			consensus.getLogger().Warn().
				Err(err).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg("[onNewView] Unparseable prepared block data")
			return
		}

		blockHash := recvMsg.Payload[:32]
		preparedBlockHash := preparedBlock.Hash()
		if !bytes.Equal(preparedBlockHash[:], blockHash) {
			consensus.getLogger().Warn().
				Err(err).
				Str("blockHash", preparedBlock.Hash().Hex()).
				Str("payloadBlockHash", hex.EncodeToString(blockHash)).
				Msg("[onNewView] Prepared block hash doesn't match msg block hash.")
			return
		}
		hasBlock = true
		if consensus.BlockVerifier(preparedBlock); err != nil {
			consensus.getLogger().Error().Err(err).Msg("[onNewView] Prepared block verification failed")
			return
		}
	}

	if m2Mask == nil || m2Mask.Bitmap == nil ||
		(m2Mask != nil && m2Mask.Bitmap != nil &&
			utils.CountOneBits(m3Mask.Bitmap) > utils.CountOneBits(m2Mask.Bitmap)) {
		if len(recvMsg.Payload) <= 32 {
			consensus.getLogger().Info().
				Msg("[onNewView] M1 (prepared) Type Payload Not Have Enough Length")
			return
		}
		// m1 is not empty, check it's valid
		blockHash := recvMsg.Payload[:32]
		aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)
		if err != nil {
			consensus.getLogger().Error().Err(err).
				Msg("[onNewView] ReadSignatureBitmapPayload Failed")
			return
		}
		if !aggSig.VerifyHash(mask.AggregatePublic, blockHash) {
			consensus.getLogger().Warn().
				Msg("[onNewView] Failed to Verify Signature for M1 (prepare) message")
			return
		}
		copy(consensus.blockHash[:], blockHash)
		consensus.aggregatedPrepareSig = aggSig
		consensus.prepareBitmap = mask
		// create prepared message from newview
		preparedMsg := FBFTMessage{
			MessageType: msg_pb.MessageType_PREPARED,
			ViewID:      recvMsg.ViewID,
			BlockNum:    recvMsg.BlockNum,
		}
		preparedMsg.BlockHash = common.Hash{}
		copy(preparedMsg.BlockHash[:], blockHash[:])
		preparedMsg.Payload = make([]byte, len(recvMsg.Payload)-32)
		copy(preparedMsg.Payload[:], recvMsg.Payload[32:])
		preparedMsg.SenderPubkey = senderKey
		consensus.FBFTLog.AddMessage(&preparedMsg)

		if hasBlock {
			consensus.FBFTLog.AddBlock(preparedBlock)
		}
	}

	// newView message verified success, override my state
	consensus.viewID = recvMsg.ViewID
	consensus.current.SetViewID(recvMsg.ViewID)
	consensus.LeaderPubKey = senderKey
	consensus.ResetViewChangeState()

	// change view and leaderKey to keep in sync with network
	if consensus.blockNum != recvMsg.BlockNum {
		consensus.getLogger().Info().
			Str("newLeaderKey", consensus.LeaderPubKey.Bytes.Hex()).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onNewView] New Leader Changed")
		return
	}

	// NewView message is verified, change state to normal consensus
	if hasBlock {
		// Construct and send the commit message
		commitPayload := signature.ConstructCommitPayload(consensus.ChainReader,
			preparedBlock.Epoch(), preparedBlock.Hash(), preparedBlock.NumberU64(), preparedBlock.Header().ViewID().Uint64())
		groupID := []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))}
		for _, key := range consensus.priKey {
			if !consensus.IsValidatorInCommittee(key.Pub.Bytes) {
				continue
			}
			network, err := consensus.construct(
				msg_pb.MessageType_COMMIT,
				commitPayload,
				&key,
			)
			if err != nil {
				consensus.getLogger().Err(err).Msg("could not create commit message")
				return
			}
			msgToSend := network.Bytes
			consensus.getLogger().Info().Msg("onNewView === commit")
			consensus.host.SendMessageToGroups(
				groupID,
				p2p.ConstructMessage(msgToSend),
			)
		}
		consensus.getLogger().Info().
			Str("From", consensus.phase.String()).
			Str("To", FBFTCommit.String()).
			Msg("[OnViewChange] Switching phase")
		consensus.switchPhase(FBFTCommit, true)
	} else {
		consensus.ResetState()
		consensus.getLogger().Info().Msg("onNewView === announce")
	}
	consensus.getLogger().Info().
		Str("newLeaderKey", consensus.LeaderPubKey.Bytes.Hex()).
		Msg("new leader changed")
	consensus.getLogger().Info().
		Msg("validator start consensus timer and stop view change timer")
	consensus.consensusTimeout[timeoutConsensus].Start()
	consensus.consensusTimeout[timeoutViewChange].Stop()
}
